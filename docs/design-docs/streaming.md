---
title: Streaming, backpressure, and abort propagation
status: accepted
last-reviewed: 2026-04-25
---

# Streaming, backpressure, and abort propagation

Two modules stream responses today: `chat_completions` (SSE text frames) and `audio_speech` (raw audio bytes). Both share the same plumbing — `backendhttp.Client.DoStream(...) → (status, headers, stream io.ReadCloser, err)` — but frame the bytes differently.

## Why a streaming-specific design

The non-streaming modules (`embeddings`, `images_generations`, `images_edits`, `audio_transcriptions`) buffer the backend's full response, parse what they need (usage / duration), and write the body in one shot. That path is straightforward; streaming adds three concerns:

1. **Headers go on the wire before the body is complete.** No retry, no error envelope after the first byte.
2. **Backpressure must propagate end-to-end.** A slow customer must slow the backend, not pile bytes in worker memory.
3. **Aborts must propagate the other way.** A customer disconnect must release the backend's GPU.

## Frame-level handling

### chat_completions (SSE)

Backend emits `text/event-stream`. The module:

- Sets headers `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive` BEFORE writing any byte.
- Reads from `bufio.NewReader(stream).ReadBytes('\n')` so each SSE line is a discrete write.
- Writes each line through to the response writer with an explicit `Flusher.Flush()` after every line. Without flushing, Go's `net/http` would buffer up to 4 KiB before the customer's TCP connection sees data.
- Peeks each `data:` line for an embedded `usage` block; the most recent one observed is the metering basis at end-of-stream.

The module does NOT re-frame the SSE bytes — `[DONE]` terminators, comments (`: keep-alive`), and named events all pass through verbatim. Re-framing is the bridge's responsibility (it injects `include_usage` for OpenAI compatibility, per its `streaming-semantics.md`).

### audio_speech (raw bytes)

Backend emits binary audio. The module:

- Reads `Content-Type` from the backend's response header and relays it (defaults to `audio/mpeg` when absent — covers TTS backends that don't set one).
- Pipes the stream into the response writer via `io.Copy(w, stream)`. Backpressure is a side effect of `io.Copy` — it reads only as fast as the writer accepts.
- Never sets `Content-Length` (the worker doesn't know the total bytes without buffering, which would defeat streaming). Transfer-Encoding is implicitly chunked.

No frame parsing; bytes are opaque.

## Backpressure

The chain is `backend ─► worker.stream ─► response writer ─► bridge ─► customer`. Each link is synchronous; a slow consumer at any point pauses every upstream. Specifically:

- `io.Copy` (audio_speech) reads only as fast as the writer drains.
- `ReadBytes('\n')` + `Write(line)` (chat_completions) pulls one line, writes one line. The buffered reader holds at most one line ahead.
- The Go HTTP server's `ResponseWriter` enforces TCP-level flow control to the client.

There is no in-memory queue between these stages. Worker memory does not grow with stream length.

## Abort propagation

Two directions:

### Customer disconnects → tear down backend

- `r.Context()` (the request context, plumbed in `middleware.go` as `ctx := r.Context()`) is the parent of the backend HTTP call.
- When the customer closes their connection, Go cancels the request context, which cancels the backend's `DoStream` request, which closes the upstream connection, which signals the inference backend to stop generating.
- The module's loop sees the next `Read` return an error and exits; the `defer stream.Close()` runs.

This works for chat_completions and audio_speech identically.

### Backend hangup → tear down customer

- `Read` from the upstream stream returns `io.EOF` (clean close) or an error.
- The module exits its loop; the response writer flushes any in-flight bytes; the customer's connection closes.

For SSE, a backend hangup mid-stream is indistinguishable from a clean end-of-stream from the customer's view — they get the bytes they got, and reconciliation runs against whatever `usage` block was last observed.

## Reconciliation under streaming

- chat_completions: the `usage` block is in the penultimate SSE chunk by OpenAI convention. The module records the most recent `usage` it saw; on stream end (clean or error), `usageToUnits(lastUsage)` becomes the `actualUnits` returned to the middleware. A stream that ended before any `usage` was emitted returns 0 → no over-debit, the upfront estimate stands.
- audio_speech: returns 0 unconditionally (metering is from `len(input)`, computed upfront).

## What streaming modules cannot do

- **Cannot retry the backend.** Once a header is on the wire, the customer is committed to the response we're forwarding. A backend error mid-stream becomes a truncated response, not a 502.
- **Cannot return an error envelope after first byte.** The body is the body; any error path that runs after `WriteHeader` is logged-only.
- **Cannot batch.** Each module instance handles exactly one in-flight stream per goroutine; the paid-route semaphore caps total concurrency.

## Cross-references

- Module contract: [capability-modules.md](capability-modules.md).
- Gateway-side SSE handling (include_usage injection,
  partial-success refunds) lives in the `livepeer-openai-gateway`
  streaming semantics docs.
