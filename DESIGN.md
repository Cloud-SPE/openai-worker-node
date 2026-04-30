# DESIGN.md — openai-worker-node

## What this service is

`openai-worker-node` is the payee-side HTTP adapter in the Livepeer BYOC payment architecture. It terminates OpenAI-compatible HTTP requests from `openai-livepeer-bridge`, validates the attached payment via a co-located `livepeer-payment-daemon` (receiver mode), forwards the request body to a local inference backend, and streams the response back.

It replaces the go-livepeer orchestrator on the worker side: there is no transcoding network, no orchestrator pool, no on-chain service registration. The worker is discovered by the bridge via static config (`nodes.yaml`) and is known-to-be-paid-for only through the `livepeer-payment` HTTP header.

## Position in the stack

```
┌────────────────┐    HTTPS     ┌──────────────────────────────────────┐
│ openai-        │  /v1/...     │ openai-worker-node  (this repo)      │
│ livepeer-      │ ───────────▶ │  ┌────────────────────────────────┐  │
│ bridge         │              │  │ HTTP router                    │  │
│                │ ◀─────────── │  │ ├─ payment middleware          │  │
└────────────────┘  SSE/JSON    │  │ └─ capability modules          │  │
                                 │  │    (chat, embeddings, …)       │  │
                                 │  └────────┬─────────────┬─────────┘  │
                                 │     gRPC  │             │  HTTP      │
                                 │   (unix)  │             │(localhost) │
                                 │           ▼             ▼            │
                                 │  ┌────────────────┐  ┌──────────┐    │
                                 │  │ livepeer-      │  │ vLLM /   │    │
                                 │  │ payment-daemon │  │ whisper/ │    │
                                 │  │ (receiver)     │  │ diffusers│    │
                                 │  └────────────────┘  └──────────┘    │
                                 └──────────────────────────────────────┘
```

## Business domains

One domain per capability, plus one shared domain for the HTTP surface:

| Domain | Purpose |
|---|---|
| `http` | Fastify-equivalent router, shared middleware stack, graceful shutdown |
| `payment` | Payee-daemon gRPC client, `ProcessPayment` + `DebitBalance` pipeline |
| `module.chat_completions` | `openai:/v1/chat/completions` adapter + token accounting |
| `module.embeddings` | `openai:/v1/embeddings` adapter + input-token accounting |
| `module.images` | `openai:/v1/images/generations` + `/v1/images/edits` (shared impl) |
| `module.audio_speech` | `openai:/v1/audio/speech` (TTS) |
| `module.audio_transcriptions` | `openai:/v1/audio/transcriptions` (ASR) |
| `module.video_generations` | `openai:/v1/video/generations` (backlog — async job model) |

Each module is self-contained under `internal/modules/<name>/` and follows the same layer rule the top-level source tree does: `types → config → service → runtime`. Registration is one line in `internal/runtime/register.go`.

## The layered architecture

Per the harness convention, code under `internal/` flows forward only:

```
types  →  config  →  repo  →  service  →  runtime
                                              ↓
                                           (UI — n/a)
utils ─────────────────────────────────────▶ (cross-cutting)
providers ─────────────────────────────────▶ (cross-cutting, single hop)
```

`runtime/` wires `providers/` + `service/` into an HTTP server. Nothing else imports `providers/`. `service/` contains pure business logic and is unit-testable without a network.

Capability modules follow the same rule internally: a module's `runtime/` registers routes; its `service/` computes work units and dispatches to the backend via a `providers/` interface. Modules do not import from each other.

## The payment pipeline (one request)

```
HTTP request
  │
  ▼
runtime.Mux  ── payment middleware ─┐
  │                                 │
  │                      ProcessPayment(payment_bytes, work_id)   ──gRPC──▶ payee daemon
  │                                 ◀── { sender, credited_ev, winners_queued }
  │                                 │
  │                      estimator(req) → est_units
  │                                 │
  │                      DebitBalance(sender, work_id, est_units) ──gRPC──▶ payee daemon
  │                                 ◀── { balance }   (must be ≥ 0)
  │                                 │
  ▼
module.Handle(req, resp_writer)    ──HTTP──▶ inference backend
  │                                 ◀── body / SSE stream
  │                                 │
  │                      (stream out to bridge)
  │                                 │
  │                      actual_units = meter(req, resp)
  │                      if actual > est: DebitBalance(delta)
  ▼
HTTP response complete
```

Reconciliation direction is over-debit only (user decision). If actual < est the ledger stays ahead; we do not credit back.

## Cross-process contracts

### worker.yaml (worker-owned file)
Single file, bind-mounted into both the daemon and this worker. The worker carries its own copy of parsing/validation in [`internal/config/`](internal/config/), covering only the fields it reads (worker section + capabilities). The daemon owns its section and validates it independently. Drift between worker and daemon is caught at startup via `VerifyDaemonCatalog`, not at compile time. Daemon-side schema reference: [Cloud-SPE/livepeer-modules `payment-daemon` shared-yaml](https://github.com/Cloud-SPE/livepeer-modules/blob/main/payment-daemon/docs/design-docs/shared-yaml.md).

### Payee daemon gRPC
Sources live in [`internal/proto/livepeer/payments/v1/`](internal/proto/livepeer/payments/v1/); generated Go in `internal/proto/gen/go/...`. The `.proto` files are wire-compatible copies of the daemon's; regenerate with `make proto`. This repo consumes the `PayeeDaemonClient` — it does not implement the service.

Startup sequence:
1. Parse `--config` → in-memory `Config`.
2. Dial payee-daemon unix socket.
3. Call `PayeeDaemon.ListCapabilities`; assert equality with parsed `Config.Capabilities`. Fail-closed on mismatch.
4. Register capability modules; bind HTTP listener.

### Bridge HTTP contract
Defined in `docs/product-specs/`. Endpoints exposed:

- `GET /health` — liveness + `protocol_version` + `max_concurrent` + `inflight`
- `GET /capabilities` — mirrors the daemon catalog, plus the worker-owned `protocol_version`, and omits backend routing details
- `GET /quote?sender=&capability=` — proxies to `PayeeDaemon.GetQuote`
- `GET /quotes?sender=` — batched version of `/quote` over all capabilities
- `POST /v1/<capability-path>` — paid work, one per capability

## Explicit non-goals (v1)

- No fan-out / load-balancing to multiple backends per (capability, model). One backend URL per pair.
- No authn/authz beyond the payment header. Payment IS auth.
- No rate limiting beyond per-sender balance exhaustion.
- No hot config reload. Restart both processes to change config.
- No credit-back primitive. Over-debit is final.
- No video generation. Backlog (async job model; needs a jobs table).
- No streaming ingest capabilities (e.g. FFMPEG live transcoding). Backlog, different module interface entirely.

## Invariants summary

Enumerated in full in `AGENTS.md`. The short list:

1. Payment is auth — enforced by a lint that checks every paid route passes through the middleware.
2. Fail-closed on config / daemon mismatch.
3. Shared YAML schema comes from the library package, not from here.
4. Providers boundary is a single hop.
5. No code without a plan.
6. Test coverage ≥ 75% per package.
