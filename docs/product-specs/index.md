# Product-specs index

The HTTP contract this worker exposes to `openai-livepeer-bridge`, and the gRPC contract it consumes from `livepeer-payment-daemon`. These are the service boundaries external consumers rely on.

## HTTP surface (this worker, for the bridge)

| Method | Path | Paid? | Spec |
|---|---|---|---|
| `GET` | `/health` | no | `health.md` (planned) |
| `GET` | `/capabilities` | no | `capabilities.md` (planned) |
| `GET` | `/quote?sender=&capability=` | no | `quote.md` (planned) |
| `GET` | `/quotes?sender=` | no | `quote.md` (planned) |
| `POST` | `/v1/chat/completions` | yes | `chat_completions.md` (planned) |
| `POST` | `/v1/embeddings` | yes | `embeddings.md` (planned) |
| `POST` | `/v1/images/generations` | yes | `images.md` (planned) |
| `POST` | `/v1/images/edits` | yes | `images.md` (planned) |
| `POST` | `/v1/audio/speech` | yes | `audio_speech.md` (planned) |
| `POST` | `/v1/audio/transcriptions` | yes | `audio_transcriptions.md` (planned) |

Each paid route accepts the base64-encoded `livepeer.payments.v1.Payment` proto in the `livepeer-payment` header. The request body and response body are OpenAI-compatible for the route's canonical endpoint.

## gRPC surface (consumed, from the payment daemon)

Defined in [`../../../livepeer-payment-library/proto/livepeer/payments/v1/payee_daemon.proto`](../../../livepeer-payment-library/proto/livepeer/payments/v1/payee_daemon.proto). This worker does not ship a copy of the proto; generated code lives in the library's `proto/gen/go/` and is consumed as a Go module dep.

Methods used:

| RPC | When called |
|---|---|
| `ListCapabilities` | Once at startup; catalog cross-check. |
| `GetQuote` | Not called by this worker (the bridge proxies through the worker's HTTP `/quote` to `PayeeDaemon.GetQuote`). |
| `ProcessPayment` | Every paid request. |
| `DebitBalance` | Every paid request (up-front + reconcile). |

Methods consumed indirectly (proxied by HTTP routes):

| HTTP route | Proxies to |
|---|---|
| `GET /quote` | `PayeeDaemon.GetQuote` |
| `GET /quotes` | `PayeeDaemon.ListCapabilities` → one `GetQuote` per capability |

## Error contract

| Condition | HTTP status | Body |
|---|---|---|
| Missing / malformed `livepeer-payment` header | 402 Payment Required | `{ "error": "missing_or_invalid_payment" }` |
| `ProcessPayment` returned error (invalid ticket, unknown sender, expired) | 402 Payment Required | `{ "error": "payment_rejected", "detail": "<daemon message>" }` |
| `DebitBalance` returned negative balance | 402 Payment Required | `{ "error": "insufficient_balance" }` |
| Model not loaded / capability not advertised | 404 Not Found | `{ "error": "capability_not_found" }` |
| Backend 5xx or connection error | 502 Bad Gateway | `{ "error": "backend_unavailable" }` |
| Request body schema validation failed | 400 Bad Request | `{ "error": "invalid_request", "detail": "..." }` |
| `max_concurrent_requests` exceeded | 503 Service Unavailable | `{ "error": "capacity_exhausted" }` |

## Conventions

- All specs under this directory are versioned; breaking changes bump the `protocol_version` advertised by `/health` and `/capabilities`.
- Request/response shapes are documented with JSON Schema or a minimal example; Go types in `internal/types/` are the canonical source of truth.
- Specs may reference design-docs for "why" decisions; they do not reference exec-plans.
