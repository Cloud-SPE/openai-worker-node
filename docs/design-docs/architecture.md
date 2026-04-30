---
title: Architecture
status: accepted
last-reviewed: 2026-04-24
---

# Architecture

## Layer stack

```
┌─────────────────────────────────────────────────────────┐
│  runtime/           HTTP server, lifecycle, signal loop │  ← may import anything below
├─────────────────────────────────────────────────────────┤
│  service/           business logic                      │  ← may import repo, providers, config, types
│    ├─ payment/        (payment middleware pipeline)     │
│    └─ modules/        (capability modules — see below)  │
├─────────────────────────────────────────────────────────┤
│  repo/              (intentionally thin in v1)          │  ← stateless adapter; only health-snapshot cache lives here
├─────────────────────────────────────────────────────────┤
│  config/            validated structs                   │  ← may import types; owns worker.yaml parse + validation
├─────────────────────────────────────────────────────────┤
│  types/             pure data                           │  ← imports nothing in internal/
└─────────────────────────────────────────────────────────┘

  providers/          cross-cutting interfaces + defaults
```

## Dependency rule

A package at layer N may import only packages at layers < N, plus `providers/`. No exceptions.

Concretely: `service/modules/chat_completions` may import `providers/tokenizer`, `providers/backend_http`, `config`, and `types`, but may not import `runtime/http`, `service/modules/embeddings`, `google.golang.org/grpc`, or any cross-cutting library directly.

Enforced by `lint/layer-check` running as a `go vet` analyzer in CI. A second custom lint (`payment-middleware-check`) verifies every paid HTTP route is registered through `runtime/http.RegisterPaidRoute`, never `Register` alone.

## Domain inventory

| Path | Purpose |
|---|---|
| `internal/runtime/http/` | HTTP server, route registration, payment middleware, shutdown. Only layer that owns listeners. |
| `internal/service/payment/` | Payment pipeline: `ProcessPayment` → `DebitBalance` → forward → reconcile. Callable by every module through a shared interface. |
| `internal/service/modules/chat_completions/` | `openai:/v1/chat/completions` adapter: request schema, estimator, streaming dispatch, token metering. |
| `internal/service/modules/embeddings/` | `openai:/v1/embeddings` adapter. |
| `internal/service/modules/images/` | `openai:/v1/images/generations` + `/v1/images/edits` (shared internal impl). |
| `internal/service/modules/audio_speech/` | `openai:/v1/audio/speech` (TTS). |
| `internal/service/modules/audio_transcriptions/` | `openai:/v1/audio/transcriptions` (ASR, multipart upload). |
| `internal/service/modules/video_generations/` | `openai:/v1/video/generations`. Backlog. |
| `internal/config/` | Worker-side `Config` projection of `sharedyaml.Config`, plus worker-only fields (`http_listen`, `max_concurrent_requests`, …). |
| `internal/types/` | Pure data types (capability IDs, work-unit kinds, request IDs). |

## Providers inventory

All cross-cutting concerns enter through `internal/providers/`. One interface per concern; one or more implementations.

| Provider | Interface role | Default impl |
|---|---|---|
| `PayeeDaemon` | gRPC client for `livepeer.payments.v1.PayeeDaemon` | `providers/payeedaemon/grpc` (unix socket) |
| `BackendHTTP` | HTTP client for inference backends | `providers/backendhttp/fetch` |
| `Tokenizer` | Token counting for chat and embeddings | `providers/tokenizer/tiktoken` |
| `Clock` | System time | `providers/clock/system` |
| `MetricsSink` | Counter / Gauge / Histogram | `providers/metrics/noop` |
| `Logger` | Structured log | `providers/logger/slog` |

Providers are wired in `cmd/openai-worker-node/main.go` and injected into `service/`.

## Capability-module pattern

Each capability module is a Go package at `internal/service/modules/<name>/`. A module exports one top-level type — the `Module` — that implements a small interface consumed by `runtime/http`:

```go
type Module interface {
    Capability() string                    // "openai:/v1/chat/completions"
    WorkUnit() string                      // "token"
    HTTPMethod() string                    // "POST"
    HTTPPath() string                      // "/v1/chat/completions"

    // EstimateWorkUnits extracts a conservative upper bound from the
    // request, used for the up-front DebitBalance call.
    EstimateWorkUnits(r *http.Request, body []byte) (int64, error)

    // Serve dispatches the (already-payment-validated) request to the
    // backend, streams the response, and returns the actual work units
    // consumed for reconciliation.
    Serve(ctx context.Context, w http.ResponseWriter, r *http.Request, body []byte, model string) (actualUnits int64, err error)
}
```

Modules are registered in `internal/runtime/http/register.go`. The module list there is the only place that can change to add a capability; no other file in the tree references specific module names.

The `payment-middleware-check` lint inspects this registration site to verify every module is wrapped in the paid-route adapter.

## Request lifecycle (one paid request)

```
HTTP request
  │
  ▼
runtime/http/server.go            ── routes to the module's registered path
  │
  ├── paymentMiddleware:
  │     ProcessPayment(header, work_id)        ──▶ PayeeDaemon
  │     est := module.EstimateWorkUnits(req)
  │     DebitBalance(sender, work_id, est)     ──▶ PayeeDaemon
  │     (fail-closed on either error)
  │
  ├── module.Serve(ctx, w, req, body, model)   ──▶ BackendHTTP
  │     (streams response to client)
  │     returns actualUnits
  │
  ├── reconcile:
  │     if actualUnits > est:
  │         DebitBalance(sender, work_id, actualUnits - est)   ──▶ PayeeDaemon
  │     (over-debit only; never credit back in v1)
  ▼
response complete
```

## Startup sequence

1. Parse `--config /etc/livepeer/worker.yaml` via the worker's local config package.
2. Project to worker-internal `Config` (worker-only fields + per-capability model→backend map).
3. Dial `PayeeDaemon` unix socket.
4. `PayeeDaemon.ListCapabilities` → compare capability/model/price tuples to local parse.
   - Mismatch → log diff, exit non-zero.
5. Instantiate each capability module from config.
6. Register routes; bind HTTP listener.
7. Start health endpoint (reports `protocol_version`, `inflight`, `max_concurrent`).

## Build artifacts

- Single Go binary: `openai-worker-node`
- No proto generation in this repo — we consume `livepeer.payments.v1` from the library module.
- Optional: Docker image (later exec-plan).

## What this architecture does NOT solve

- Horizontal scaling across multiple worker instances sharing balance state. The daemon is per-worker; multi-worker deployments are multi-daemon.
- Streaming-ingest capabilities (FFMPEG live transcoding). Backlog; needs a different module interface entirely (long-lived bidirectional connection, periodic `DebitBalance`, stream-cancel on insufficient balance).
- Custom operator-defined capabilities. Backlog; will likely land as either a `custom:` config-only passthrough module or a sidecar gRPC module pattern.
- A metrics dashboard. `MetricsSink` is pluggable; the operator provides it.
