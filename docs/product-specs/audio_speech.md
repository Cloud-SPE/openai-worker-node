---
title: POST /v1/audio/speech
status: live
last-reviewed: 2026-04-25
module: internal/service/modules/audio_speech
---

# POST /v1/audio/speech

OpenAI-compatible text-to-speech endpoint. JSON request in, **streaming raw audio bytes** out.

## Request

| Field             | Type    | Notes                                       |
| ----------------- | ------- | ------------------------------------------- |
| `model`           | string  | Required                                    |
| `input`           | string  | The text to synthesize; metering basis      |
| `voice`           | string  | Forwarded verbatim                          |
| `response_format` | string  | `mp3` \| `opus` \| `aac` \| `flac` \| `wav` \| `pcm` (forwarded verbatim) |
| `speed`           | number  | 0.25–4.0 (forwarded verbatim)               |

The worker only inspects `model` (for routing) and `input` (for metering); every other field rides through to the backend.

## Metering

- **Capability**: `openai:/v1/audio/speech`
- **Work unit**: `character`
- **Estimate / actual**: `utf8.RuneCountInString(input)` — exact, not an estimate. No reconciliation needed; the upfront debit is the final charge.

Empty input counts as 0 (the backend will reject it). Multi-byte characters count as one each.

## Response

Streaming. The worker uses `backendhttp.DoStream` so audio bytes flow to the bridge as soon as the backend produces them — the bridge can begin playback before synthesis finishes.

- **Content-Type**: relayed verbatim from the backend (`audio/mpeg`, `audio/wav`, `audio/opus`, …). When the backend omits a Content-Type the worker defaults to `audio/mpeg` (matches OpenAI's documented default).
- **Transfer-Encoding**: chunked. The worker never sets a `Content-Length` since it does not know the total byte count without buffering the full response.

A client disconnect mid-stream propagates as a `ctx.Done()` to the backend HTTP call; the worker tears down both ends and logs a Warn. The customer is still billed for the full `len(input)` — synthesis work is not refundable once dispatched.

## Errors

| Condition                                              | Status | Body                                                |
| ------------------------------------------------------ | ------ | --------------------------------------------------- |
| Body not JSON, or `model` missing                      | 400    | `{ "error": "invalid_request", "detail": "..." }`  |
| `(capability, model)` not configured                    | 404    | `{ "error": "capability_not_found" }`               |
| Backend connection / 5xx                                | 502    | `{ "error": "backend_unavailable" }`                |
| Worker at `max_concurrent_requests`                     | 503    | `{ "error": "capacity_exhausted" }`                 |

## Implementation notes

- File: `internal/service/modules/audio_speech/module.go`.
- The worker does NOT transcode or re-package audio. If the customer asked for `pcm` and the backend returned `audio/mpeg`, that's the bytes the customer gets — the bridge enforces the format-mismatch contract on its side.
- Streaming uses `io.Copy(w, stream)`; backpressure flows naturally from the bridge's connection through the worker's response writer to the backend's reader.
