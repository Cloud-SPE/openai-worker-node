# Product-specs index

The HTTP contract this worker exposes to `openai-livepeer-bridge`, and the gRPC contract it consumes from `livepeer-payment-daemon`. These are the service boundaries external consumers rely on.

## HTTP surface (this worker, for the bridge)

| Method | Path                          | Paid? | Status | Spec                                                  |
| ------ | ----------------------------- | ----- | ------ | ----------------------------------------------------- |
| `GET`  | `/health`                     | no    | live   | inline below                                          |
| `GET`  | `/capabilities`               | no    | live   | inline below                                          |
| `GET`  | `/quote?sender=&capability=`  | no    | live   | inline below                                          |
| `GET`  | `/quotes?sender=`             | no    | live   | inline below                                          |
| `POST` | `/v1/chat/completions`        | yes   | live   | [chat_completions.md](chat_completions.md)            |
| `POST` | `/v1/embeddings`              | yes   | live   | [embeddings.md](embeddings.md)                        |
| `POST` | `/v1/images/generations`      | yes   | live   | [images.md](images.md)                                |
| `POST` | `/v1/images/edits`            | yes   | live   | [images.md](images.md)                                |
| `POST` | `/v1/audio/speech`            | yes   | live   | [audio_speech.md](audio_speech.md)                    |
| `POST` | `/v1/audio/transcriptions`    | yes   | live   | [audio_transcriptions.md](audio_transcriptions.md)    |

Each paid route accepts the base64-encoded `livepeer.payments.v1.Payment` proto in the `livepeer-payment` header. The request body and response body are OpenAI-compatible for the route's canonical endpoint.

## gRPC surface (consumed, from the payment daemon)

Defined in [`../../../livepeer-modules-project/payment-daemon/proto/livepeer/payments/v1/payee_daemon.proto`](../../../livepeer-modules-project/payment-daemon/proto/livepeer/payments/v1/payee_daemon.proto). This worker does not ship a copy of the proto; generated code lives in the library's `proto/gen/go/` and is consumed as a Go module dep.

Methods used:

| RPC                  | When called                                                                                                              |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `ListCapabilities`   | Once at startup (catalog cross-check) and once per request to `GET /quotes` (to enumerate capabilities for the response). |
| `GetQuote`           | Once per `GET /quote` request, and once per advertised capability when serving `GET /quotes`.                            |
| `ProcessPayment`     | Every paid request.                                                                                                      |
| `DebitBalance`       | Every paid request (up-front + reconcile).                                                                               |

Proxy mapping for the unpaid HTTP surface:

| HTTP route   | Proxies to                                                                       |
| ------------ | -------------------------------------------------------------------------------- |
| `GET /quote` | `PayeeDaemon.GetQuote`                                                           |
| `GET /quotes`| `PayeeDaemon.ListCapabilities` → one `PayeeDaemon.GetQuote` per advertised capability |

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
