---
id: 0002
slug: chat-completions-module
title: First capability module — openai:/v1/chat/completions
status: completed
owner: agent
opened: 2026-04-24
completed: 2026-04-24
---

## Goal

Ship the first real capability module end-to-end: parse config, register the route, count tokens, dispatch to a vLLM-compatible backend, stream SSE back to the caller, reconcile the final token count. This is the module pattern every other capability will mirror; get the pattern right here and the rest is replication.

## Non-goals

- No embeddings, images, or audio. Those are 0004–0006.
- No payment middleware. That is 0003, which this plan depends on.
- No request-body validation beyond what the backend itself rejects. A full OpenAI schema checker is a later plan; until then, we trust the bridge to send valid bodies.
- No load balancing to multiple backends. One URL per model per the architecture doc.

## Cross-repo dependencies

- Library `0018-per-capability-pricing` (shared YAML + `ListCapabilities` + per-model pricing).
- This repo: `0003-payment-middleware` (the paid-route adapter this module gets wrapped in).

## Approach

- [x] Module package at `internal/service/modules/chat_completions/`.
- [x] Request/response types in `internal/service/modules/chat_completions/types.go` — narrow slice of OpenAI's schema (model, messages, max_tokens, stream, usage). Extra fields ride through verbatim.
- [x] `EstimateWorkUnits`: sums tokens across messages (role + content) via the tokenizer provider + adds the requested `max_tokens` (falling back to `DefaultMaxCompletionTokens`). Single-price charging per the metering decision — no input/output split.
- [x] `Serve`: branches on `stream=true` vs `stream=false`.
  - Non-streaming: `POST` via `backendhttp.Client.DoJSON`, write buffered response through, derive actual units from `usage.total_tokens`.
  - Streaming: `POST` via `DoStream`, pipe SSE chunks byte-for-byte to the client with per-chunk `Flusher.Flush`, scrape the final `usage` chunk for reconciliation.
- [x] Streaming: byte-exact SSE proxy — each line (including trailing `\n`) is written verbatim. `data: [DONE]` + named-event lines + comments are ignored for usage extraction but still forwarded. Client disconnect mid-stream causes `Write` to error, which we return without re-wrapping (middleware only logs post-response errors).
- [x] Prerequisite providers:
  - `internal/providers/backendhttp/` — `Client` with `DoJSON` + `DoStream`, fetch impl, concurrency-safe Fake. 0% coverage via unit tests (exercised end-to-end via module tests — fake is used there).
  - `internal/providers/tokenizer/` — `Tokenizer` interface + `NewWordCount(multiplierPct)` placeholder implementation (whitespace split × 1.33 default). 100% coverage, 7 tests.
- [x] Integration tests: 10 tests against a fake backend.
  - ExtractModel happy path + missing model + bad JSON.
  - EstimateWorkUnits with default max, explicit max, multi-part content.
  - Serve non-streaming happy path + no-usage fallback + backend error (502).
  - Serve streaming happy path + no-usage fallback + backend error.
  - Capability/path accessors.
  - Module coverage: 89.2% of statements.
- [x] main wiring: `cmd/openai-worker-node/main.go` now assembles config, dials daemon, cross-checks catalog, registers the module, starts the server, and handles SIGINT/SIGTERM graceful shutdown. Binary builds at 18 MB; smoke-tested against bad and placeholder configs (both fail-closed correctly).

## Decisions log

### 2026-04-24 — Word-count tokenizer as the default provider

`tiktoken-go` is the obvious choice for accurate counts, but it's heavyweight (embedded BPE tables) and its first-run download makes test execution flaky. For v1 we ship a whitespace tokenizer with a 1.33× multiplier — OpenAI's documented words-to-tokens ratio. Over-estimates on mixed-script text, which is safe under the over-debit policy. Provider swap for tiktoken / sentencepiece is tracked in tech-debt when real traffic surfaces a case where the estimate matters.

### 2026-04-24 — Model argument on EstimateWorkUnits reserved for per-model tokenizer dispatch

The interface signature includes `model` even though the default word-count tokenizer ignores it. This keeps the seam for future per-family tokenizer routing (llama's sentencepiece vs OpenAI's tiktoken) without needing an interface change later.

### 2026-04-24 — Streaming abort: rely on server-side context cancellation, no explicit cancel call

Go's `http.Server` already cancels the request `Context` when the client disconnects. That propagates to the backend's HTTP request (created with `http.NewRequestWithContext`) without us adding cancel plumbing. Mirrors bridge semantics; no code needed.

### 2026-04-24 — Content-type handling for multi-part messages

Multi-part content arrays (`[{"type":"text","text":"..."}, {"type":"image_url", ...}]`) are flattened to their textual part for tokenization only. Non-text parts render as their JSON, which over-counts. The backend still sees the original bytes — we never re-serialize the request body.

## Open questions

All resolved for v1.

## Artifacts produced

Files landed:

- `internal/providers/backendhttp/{doc,interface,fetch,fake}.go`
- `internal/providers/tokenizer/{doc,tokenizer,tokenizer_test}.go`
- `internal/service/modules/chat_completions/{doc,types,module,module_test}.go`
- `cmd/openai-worker-node/main.go` — full wiring replacing the scaffold stub.
