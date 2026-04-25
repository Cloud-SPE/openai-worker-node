---
title: POST /v1/chat/completions
status: live
last-reviewed: 2026-04-25
module: internal/service/modules/chat_completions
---

# POST /v1/chat/completions

OpenAI-compatible chat completions endpoint. Streaming (SSE) and non-streaming variants are served by the same handler — the `stream` field on the request body decides which response shape is returned.

## Request

Standard OpenAI chat-completion shape. The fields the worker reads explicitly:

| Field         | Type                              | Used for                                      |
| ------------- | --------------------------------- | --------------------------------------------- |
| `model`       | string                            | Capability/model lookup → backend URL         |
| `messages`    | array of `{ role, content }`      | Upfront token estimate                        |
| `max_tokens`  | int (optional)                    | Output-side reservation; defaults to 2048     |
| `stream`      | bool (optional, default `false`)  | Selects SSE vs JSON response path             |

Every other field (`temperature`, `tools`, `response_format`, …) rides through to the backend verbatim.

The bridge supplies the base64-encoded `livepeer.payments.v1.Payment` proto in the `livepeer-payment` header.

## Metering

- **Capability**: `openai:/v1/chat/completions`
- **Work unit**: `token`
- **Upfront estimate**: `Σ tokenize(role) + tokenize(content) + max(max_tokens, 2048)`
- **Actual**: `usage.total_tokens` from the backend response (final SSE chunk in streaming mode, or buffered JSON otherwise)
- **Reconciliation**: middleware issues a second `DebitBalance` for `actual − estimate` when `actual > estimate`. No refund path — the over-debit-accepted policy applies in the other direction.
- **Tokenizer**: provider-pluggable (`internal/providers/tokenizer`). Default impl is a word-count ×1.33 estimator; per-family swaps land via the tokenizer interface.

A request that returns `stream: true` but is interrupted mid-stream still reconciles using the most recent `usage` block observed; if the backend never emitted one, the upfront estimate stands as the final charge.

## Response

### Non-streaming (`stream: false` or omitted)

`Content-Type: application/json`. Whole body is buffered, then forwarded with the backend's status code.

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1735689600,
  "model": "gpt-4o-mini",
  "choices": [...],
  "usage": { "prompt_tokens": 12, "completion_tokens": 34, "total_tokens": 46 }
}
```

### Streaming (`stream: true`)

`Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`. The backend's SSE frames are forwarded chunk-for-chunk; the worker does not re-frame or buffer.

The terminal frame conventionally carries `data: [DONE]`. The penultimate frame carries the `usage` block; that block is what the worker reconciles against.

The worker MUST NOT modify the SSE payload (beyond passing bytes through). In particular, the worker does not inject or strip the `usage` block — that responsibility belongs to the bridge per its `streaming-semantics.md` design doc.

## Errors

Standard worker error envelope (see [index.md](index.md)). Endpoint-specific cases:

| Condition                                                                | Status | Body                                                                |
| ------------------------------------------------------------------------ | ------ | ------------------------------------------------------------------- |
| JSON body fails to parse, or `model` field missing                       | 400    | `{ "error": "invalid_request", "detail": "..." }`                  |
| `(capability, model)` not in worker config                               | 404    | `{ "error": "capability_not_found" }`                               |
| Backend connection / 5xx                                                  | 502    | `{ "error": "backend_unavailable" }`                                |
| Worker at `max_concurrent_requests`                                       | 503    | `{ "error": "capacity_exhausted" }`                                 |

All payment-related rejections (missing header, ProcessPayment failure, negative balance after upfront debit) fall through the shared middleware — see the universal error contract in `index.md`.

## Implementation notes

- File: `internal/service/modules/chat_completions/module.go`.
- Default `max_tokens` ceiling is `Module.DefaultMaxCompletionTokens`, set to 2048 in `New`. Operators may bump it before passing the module to the mux.
- Multi-modal `content` arrays are flattened via best-effort text extraction for tokenization purposes only — the backend sees the original bytes.
