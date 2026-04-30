# Product-specs index

The HTTP contract this worker exposes to `livepeer-openai-gateway` and
the orch-coordinator, and the gRPC contract it consumes from
`livepeer-payment-daemon`. These are the service boundaries external
consumers rely on.

## HTTP surface (this worker, for the bridge)

| Method | Path                          | Paid? | Status | Spec                                                  |
| ------ | ----------------------------- | ----- | ------ | ----------------------------------------------------- |
| `GET`  | `/health`                     | no    | live   | inline below                                          |
| `GET`  | `/registry/offerings`         | no    | live   | inline below                                          |
| `POST` | `/v1/chat/completions`        | yes   | live   | [chat_completions.md](chat_completions.md)            |
| `POST` | `/v1/embeddings`              | yes   | live   | [embeddings.md](embeddings.md)                        |
| `POST` | `/v1/images/generations`      | yes   | live   | [images.md](images.md)                                |
| `POST` | `/v1/images/edits`            | yes   | live   | [images.md](images.md)                                |
| `POST` | `/v1/audio/speech`            | yes   | live   | [audio_speech.md](audio_speech.md)                    |
| `POST` | `/v1/audio/transcriptions`    | yes   | live   | [audio_transcriptions.md](audio_transcriptions.md)    |

Each paid route accepts the base64-encoded `livepeer.payments.v1.Payment` proto in the `livepeer-payment` header. The request body and response body are OpenAI-compatible for the route's canonical endpoint.

## gRPC surface (consumed, from the payment daemon)

Defined in [`../../../internal/proto/livepeer/payments/v1/payee_daemon.proto`](../../../internal/proto/livepeer/payments/v1/payee_daemon.proto). This repo carries its own wire-compatible copy of the proto; generated Go stubs land at `internal/proto/gen/go/` and are regenerated with `make proto`. The upstream side (the daemon) is at [Cloud-SPE/livepeer-modules `payment-daemon`](https://github.com/Cloud-SPE/livepeer-modules/tree/main/payment-daemon/proto/livepeer/payments/v1).

Methods used:

| RPC                  | When called                                                                                                              |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `ListCapabilities`   | Once at startup for the worker/daemon catalog cross-check.                                                 |
| `GetQuote`           | Not used by the v3.0.1 worker HTTP surface. Retained only in the provider layer for daemon compatibility. |
| `ProcessPayment`     | Every paid request.                                                                                                      |
| `DebitBalance`       | Every paid request (up-front + reconcile).                                                                               |

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

- All specs under this directory are versioned; breaking changes bump
  the worker HTTP `api_version` and/or shared YAML `protocol_version`
  advertised on `/health`.
- Request/response shapes are documented with JSON Schema or a minimal example; Go types in `internal/types/` are the canonical source of truth.
- Specs may reference design-docs for "why" decisions; they do not reference exec-plans.
