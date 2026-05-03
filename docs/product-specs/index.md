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
| `POST` | `/v1/payment/ticket-params`   | no*   | live   | inline below                                          |
| `POST` | `/v1/chat/completions`        | yes   | live   | [chat_completions.md](chat_completions.md)            |
| `POST` | `/v1/embeddings`              | yes   | live   | [embeddings.md](embeddings.md)                        |
| `POST` | `/v1/images/generations`      | yes   | live   | [images.md](images.md)                                |
| `POST` | `/v1/images/edits`            | yes   | live   | [images.md](images.md)                                |
| `POST` | `/v1/audio/speech`            | yes   | live   | [audio_speech.md](audio_speech.md)                    |
| `POST` | `/v1/audio/transcriptions`    | yes   | live   | [audio_transcriptions.md](audio_transcriptions.md)    |

Each paid route accepts the base64-encoded `livepeer.payments.v1.Payment` proto in the `livepeer-payment` header. The request body and response body are OpenAI-compatible for the route's canonical endpoint.

`POST /v1/payment/ticket-params` is unpaid in the payment sense, but when `auth_token` is configured in `worker.yaml` it requires `Authorization: Bearer <token>` exactly like `/registry/offerings`. It is a thin proxy to the local receiver-mode payment daemon's `GetTicketParams` RPC and does no pricing or crypto locally.

Minimal request / response shape:

```json
POST /v1/payment/ticket-params
Authorization: Bearer <token>
Content-Type: application/json

{
  "sender_eth_address": "0x1111111111111111111111111111111111111111",
  "recipient_eth_address": "0xd00354656922168815fcd1e51cbddb9e359e3c7f",
  "face_value_wei": "1250000",
  "capability": "openai:/v1/chat/completions",
  "offering": "gpt-oss-20b"
}
```

```json
{
  "ticket_params": {
    "recipient": "0xd00354656922168815fcd1e51cbddb9e359e3c7f",
    "face_value": "1250000",
    "win_prob": "0x123456",
    "recipient_rand_hash": "0xaabbcc",
    "seed": "0xdeadbeef",
    "expiration_block": "9876543",
    "expiration_params": {
      "creation_round": 4523,
      "creation_round_block_hash": "0x01020304"
    }
  }
}
```

## gRPC surface (consumed, from the payment daemon)

Defined in [`../../../internal/proto/livepeer/payments/v1/payee_daemon.proto`](../../../internal/proto/livepeer/payments/v1/payee_daemon.proto). This repo carries its own wire-compatible copy of the proto; generated Go stubs land at `internal/proto/gen/go/` and are regenerated with `make proto`. The upstream side (the daemon) is at [Cloud-SPE/livepeer-modules `payment-daemon`](https://github.com/Cloud-SPE/livepeer-modules/tree/main/payment-daemon/proto/livepeer/payments/v1).

Methods used:

| RPC                  | When called                                                                                                              |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `ListCapabilities`   | Once at startup for the worker/daemon catalog cross-check.                                                 |
| `GetQuote`           | Not used by the current worker HTTP surface. Retained only in the provider layer for daemon compatibility. |
| `GetTicketParams`    | On `POST /v1/payment/ticket-params`; returns canonical payee-issued ticket params for sender-side ticket minting. |
| `OpenSession`        | Every paid request, before `ProcessPayment`, to bind payee-side pricing metadata to `work_id`. |
| `ProcessPayment`     | Every paid request, after `OpenSession`.                                                                                                      |
| `DebitBalance`       | Every paid request (up-front + reconcile).                                                                               |
| `CloseSession`       | Every paid request after payment processing completes; pending sessions created before a rejected `ProcessPayment` cannot be explicitly closed by the current daemon API. |

## Error contract

| Condition | HTTP status | Body |
|---|---|---|
| Missing / malformed `livepeer-payment` header | 402 Payment Required | `{ "error": "missing_or_invalid_payment" }` |
| `ProcessPayment` returned error (invalid ticket, unknown sender, expired) | 402 Payment Required | `{ "error": "payment_rejected", "detail": "<daemon message>" }` |
| `DebitBalance` returned negative balance | 402 Payment Required | `{ "error": "insufficient_balance" }` |
| Model not loaded / capability not advertised | 404 Not Found | `{ "error": "capability_not_found" }` |
| Backend 5xx or connection error | 502 Bad Gateway | `{ "error": "backend_unavailable" }` |
| Request body schema validation failed | 400 Bad Request | `{ "error": "invalid_request", "detail": "..." }` |
| `/registry/offerings` or `/v1/payment/ticket-params` bearer missing / invalid when `auth_token` enabled | 401 Unauthorized | `{ "error": "unauthorized", "detail": "missing or invalid bearer token" }` |
| `GetTicketParams` daemon unavailable | 503 Service Unavailable | `{ "error": "payment_daemon_unavailable", "detail": "<daemon message>" }` |
| `GetTicketParams` daemon failed to issue params | 500 Internal Server Error | `{ "error": "ticket_params_unavailable", "detail": "<daemon message>" }` |
| `max_concurrent_requests` exceeded | 503 Service Unavailable | `{ "error": "capacity_exhausted" }` |

## Conventions

- All specs under this directory are versioned; breaking changes bump
  the worker HTTP `api_version` and/or shared YAML `protocol_version`
  advertised on `/health`.
- Request/response shapes are documented with JSON Schema or a minimal example; Go types in `internal/types/` are the canonical source of truth.
- Specs may reference design-docs for "why" decisions; they do not reference exec-plans.
