---
title: Worker metrics catalog
status: accepted
last-reviewed: 2026-04-25
---

# Worker metrics catalog

What `openai-worker-node` exposes on `/metrics` and why each metric exists. Every metric pairs to a question an operator will dashboard or alert on. No vanity metrics.

**Cross-repo conventions**: [`../../../livepeer-modules-conventions/metrics-conventions.md`](../../../livepeer-modules-conventions/metrics-conventions.md). This doc covers worker-specific instantiation; the conventions doc covers cross-repo rules (naming, label keys, bucket presets, cardinality cap, dual-histogram, audit-log philosophy, provider boundary).

The pattern mirrors [Cloud-SPE/livepeer-modules `service-registry-daemon` observability](https://github.com/Cloud-SPE/livepeer-modules/blob/main/service-registry-daemon/docs/design-docs/observability.md) (status `verified`) — same package layout, same flag names, same `livepeer_<repo>_*` prefix.

Phases:

- **Phase 1** (this doc, in-scope to build): Recorder provider + TCP `/metrics` listener + ~10 metrics covering request lifecycle, payment validation, capacity, backend health, and the daemon RPC critical path.
- **Phase 2**: streaming TTFT, reconcile-delta histograms, payment-rejection breakdowns, per-capability stream-abort accounting, optional audit-log surface for per-request drilldown.
- **Phase 3**: no new Prometheus metrics. Work-units served + daemon counters are the only signals the bridge needs from the worker — per-worker margin lives in [`openai-livepeer-bridge`'s `operator-economics-metrics-tooling`](../../../openai-livepeer-bridge/docs/exec-plans/tech-debt-tracker.md#operator-economics-metrics-tooling) rollups.

## Conventions (worker-specific)

The cross-repo conventions doc covers: prefix (`livepeer_worker_*`), bucket presets (Default + Fast), cardinality cap (`--metrics-max-series-per-metric` default `10000`), dual-histogram pattern for gRPC, forbidden labels, audit-log-vs-label rule, provider boundary.

Worker-specific:

- **Allowed labels (this repo)**: `capability`, `model`, `unit`, `outcome`, `reason`, `error_class`, `method`. `(capability, model)` cardinality is bounded by `worker.yaml`.
- **No per-customer drilldown surface in v1.** The worker doesn't see customers (the only payer-side identifier it has is `sender` address, which is forbidden). If per-request drilldown becomes a real need, it lands as an audit-log table in Phase 2 mirroring service-registry's pattern.
- **Endpoint**: `--metrics-listen=:9093` (port `:9093` per [`port-allocation.md`](../../../livepeer-modules-conventions/port-allocation.md)). Off by default. `METRICS_PORT` in `.env.example` is the docker-compose convenience.

## Phase 1 catalog

### Request lifecycle

| Question | Metric | Type | Labels |
|---|---|---|---|
| End-to-end latency by capability/model/outcome | `livepeer_worker_request_duration_seconds` | histogram (Default buckets) | `capability`, `model`, `outcome={2xx,4xx,402,5xx,canceled}` |
| Throughput per capability/model | `livepeer_worker_requests_total` | counter | `capability`, `model`, `outcome` |
| Work units actually billed (the revenue signal) | `livepeer_worker_work_units_total` | counter | `capability`, `model`, `unit={token,character,audio_second,image_step_megapixel}` |

`livepeer_worker_work_units_total` is the most important metric in this catalog. It's what the bridge cross-checks against `livepeer_bridge_revenue_usd_cents_total` for margin reconciliation. The `unit` label is needed because different capabilities meter in different dimensions.

Outcome buckets:
- `2xx` — request succeeded, response complete (streaming or not).
- `4xx` — client error excluding 402 (bad model, malformed body, multipart parse fail).
- `402` — payment-related rejection (validation, insufficient balance). Broken out so 402 spikes are visible without parsing 4xx noise.
- `5xx` — backend or worker internal error.
- `canceled` — client disconnect / context cancellation. Emitted when streaming aborts after first byte; otherwise rolls up to whatever the response would have been.

### Payment validation

| Question | Metric | Type | Labels |
|---|---|---|---|
| Why are payments being rejected? | `livepeer_worker_payment_rejections_total` | counter | `reason={process_payment_failed,insufficient_balance,debit_error,header_invalid,header_missing}` |
| Latency to colocated daemon (every paid request hits it 2–3×) — default range | `livepeer_worker_daemon_rpc_duration_seconds` | histogram (Default buckets) | `method`, `outcome` |
| Latency to colocated daemon — sub-ms detail | `livepeer_worker_daemon_rpc_duration_seconds_fast` | histogram (Fast buckets) | `method`, `outcome` |
| Daemon RPC accounting | `livepeer_worker_daemon_rpc_calls_total` | counter | `method={ProcessPayment,DebitBalance,GetQuote,ListCapabilities}`, `outcome={ok,error}` |

The daemon RPC is the gRPC critical path — both histograms (Default + Fast) are emitted per the dual-histogram convention. `ProcessPayment` and `DebitBalance` happen inline before any backend work; a p99 spike here is invisible in the request histogram but easy to spot in the daemon RPC histograms.

### Capacity & shedding

| Question | Metric | Type | Labels |
|---|---|---|---|
| Am I shedding load? | `livepeer_worker_capacity_rejections_total` | counter | `capability` |
| Current concurrency | `livepeer_worker_inflight_requests` | gauge | — |

`livepeer_worker_inflight_requests` is the live semaphore depth — the number `/health` already returns. Surfacing it as a gauge means operators don't have to scrape `/health` separately.

### Backend health

| Question | Metric | Type | Labels |
|---|---|---|---|
| Is the upstream model backend healthy? | `livepeer_worker_backend_request_duration_seconds` | histogram (Default buckets) | `capability`, `model` |
| Backend errors by class | `livepeer_worker_backend_errors_total` | counter | `capability`, `model`, `error_class={timeout,5xx,malformed,connect}` |
| Backend request accounting | `livepeer_worker_backend_requests_total` | counter | `capability`, `model`, `outcome={ok,error}` |
| Last successful backend response (alert on staleness) | `livepeer_worker_backend_last_success_timestamp_seconds` | gauge | `capability`, `model` |

Backend errors are split from request errors: a 5xx-from-backend ≠ 5xx-to-client (the worker sometimes maps backend issues to 502). The `error_class` taxonomy mirrors what `internal/providers/backendhttp/fetch.go` already classifies in logs.

### Build / health

| Metric | Type | Labels |
|---|---|---|
| `livepeer_worker_build_info` | gauge=1 | `version`, `protocol_version`, `go_version` |
| `livepeer_worker_max_concurrent` | gauge | — |

`livepeer_worker_build_info` lets every series be tagged with the build/protocol. `livepeer_worker_max_concurrent` is config — paired with `livepeer_worker_inflight_requests` it gives utilization at a glance. Standard `process_*` and `go_*` collectors handle uptime, GC, FD count.

## Phase 2 catalog (additive)

| Question | Metric | Type | Labels |
|---|---|---|---|
| Streaming time-to-first-token/byte | `livepeer_worker_stream_ttft_seconds` | histogram | `capability` (chat + TTS only), `model` |
| Stream lifecycle outcomes | `livepeer_worker_streams_total` | counter | `capability`, `model`, `outcome={completed,client_canceled,backend_error,timeout}` |
| Is metering accurate (estimate vs. actual)? | `livepeer_worker_reconcile_delta_units` | histogram (signed) | `capability`, `model` |
| How often does reconciliation fire? | `livepeer_worker_reconcile_total` | counter | `capability`, `model`, `direction={over,under,exact}` |

The reconcile-delta histogram answers "are we systematically under-estimating max_tokens?" — if yes, the upfront `DebitBalance` reservation is wrong and customers occasionally hit unexpected 402s. The signed histogram (negative = under-estimated, positive = over-estimated) shows bias.

`livepeer_worker_streams_total` distinguishes customer disconnect (`client_canceled`) from backend hangup (`backend_error`). Phase 1 rolls these into `livepeer_worker_requests_total{outcome=canceled|5xx}`; Phase 2 splits when streaming-quality SLOs warrant it.

## Phase 3

The worker doesn't grow new Prometheus metrics in Phase 3. Per-worker margin and per-customer cohort analysis live in the bridge (USD); per-worker on-chain economics live in the daemon (wei). The worker emits work-units and lets those layers correlate.

The one Phase 3-class deliverable from this repo is **schema stability**: once Phase 1 has been live against mainnet for ≥30 days, lock the metric names + label sets and treat them as a public surface (semver + deprecation). Until then, names can change.

## What we deliberately do NOT measure

- **Per-customer or per-API-key counters.** The worker doesn't see customers; the only payer-side identifier is `sender` address (high-cardinality + forbidden by conventions).
- **Tokenizer cache hit rate.** Interesting once, then forgotten. Add only if a real performance issue shows up.
- **Multipart parse duration.** Would be `<10 ms` on every request; nobody looks at it.
- **Request body size histogram.** `maxPaidRequestBodyBytes` already caps it.
- **Worker startup count.** `process_start_time_seconds` covers it.
- **Per-backend-URL labels.** Cardinality + URLs may contain credentials. `capability + model` is the right grouping.

## Wiring

Same package split as service-registry: `internal/providers/metrics/` (Recorder + impls) + `internal/runtime/metrics/` (TCP listener). Per-provider decorators live next to the provider they wrap. Per the conventions doc, **no service or repo package may import `prometheus/client_golang` directly** — all emissions go through the Recorder interface (enforced by depguard).

### Package layout

- `internal/providers/metrics/recorder.go` — `Recorder` interface: a fat **domain-specific** surface (e.g. `IncRequest(capability, model, outcome string)`, `AddWorkUnits(capability, model, unit string, n int64)`, `ObserveDaemonRPC(method, outcome string, d time.Duration)`), NOT a generic `Counter/Gauge/Histogram` factory. Adding a metric means adding a method here and implementing it in every Recorder. Also exports label-value constants so call sites can't typo a label value.
- `internal/providers/metrics/noop.go` — zero-cost noop. Every method is a no-op; `Handler()` returns 404 with a clear "metrics listener not enabled" message.
- `internal/providers/metrics/prometheus.go` — `prometheus/client_golang` impl. Owns a custom `*prometheus.Registry` (not the global default) + standard collectors. Defines the bucket presets inline. Holds the `capVec` cardinality wrapper that drops new label tuples beyond `MaxSeriesPerMetric`; the wrapper exposes both `inc()` and `add(delta)` (worker-specific extension over service-registry's `inc()`-only shape, needed for cumulative `AddWorkUnits`). Dual-histogram for the unix-socket gRPC histogram is two distinct `*prometheus.HistogramVec` fields written from one `ObserveDaemonRPC` call.
- `internal/providers/metrics/testhelpers.go` — `Counter` test helper.
- `internal/runtime/metrics/listener.go` — TCP HTTP listener (`/metrics` + `/healthz`), graceful shutdown integrated with the worker lifecycle.

### Decorators (per-provider, inline `WithMetrics` constructor)

Each provider package adds a `WithMetrics(inner, recorder, ...) <Interface>` constructor and a private wrapper struct **inside its existing source file** (e.g. `interface.go`, `tokenizer.go`), matching service-registry's pattern. Tests live in a separate `metered_test.go` per provider package. Production wiring is one wrap per provider in `cmd/openai-worker-node/main.go`.

- `internal/providers/payeedaemon/interface.go` exports `WithMetrics(c Client, rec metrics.Recorder) Client` → `livepeer_worker_daemon_rpc_calls_total{method,outcome}`, `livepeer_worker_daemon_rpc_duration_seconds{method,outcome}` + `_fast` (dual-histogram, unix-socket gRPC).
- `internal/providers/backendhttp/interface.go` exports `WithMetrics(c Client, rec metrics.Recorder, capability, model string) Client` → `livepeer_worker_backend_requests_total{capability,model,outcome}`, `livepeer_worker_backend_request_duration_seconds{capability,model}`, `livepeer_worker_backend_errors_total{capability,model,error_class}`, `livepeer_worker_backend_last_success_timestamp_seconds{capability,model}`. Takes `(capability, model)` at construction because the existing `Client` interface methods don't carry them — refactoring to a `Request` struct that carries the labels (eliminating per-pair wrapping) is a Phase 2 item; today each capability module wraps via a `backendFor(model)` helper per request.
- `internal/providers/tokenizer/tokenizer.go` exports `WithMetrics(t Tokenizer, rec metrics.Recorder) Tokenizer` → `livepeer_worker_tokenizer_calls_total{model,outcome}`. Latency intentionally skipped — typically <100 µs.

### HTTP middleware (customer-facing surface)

`internal/runtime/http/middleware.go` extends `paymentMiddleware` with a metrics observation pass. Emits `livepeer_worker_requests_total`, `livepeer_worker_request_duration_seconds`, `livepeer_worker_payment_rejections_total`, `livepeer_worker_capacity_rejections_total`. The `livepeer_worker_inflight_requests` gauge is updated in semaphore acquire (`Inc()`) and the deferred release (`Dec()`).

### Direct Recorder injection in capability modules

`internal/modules/{chat,embeddings,images,audio}` — each module gets a `Unit() string` method (chat/embeddings → `token`, audio.speech → `character`, audio.transcriptions → `audio_second`, images.{generations,edits} → `image_step_megapixel`). The module emits `livepeer_worker_work_units_total{capability,model,unit}` after a successful response. The middleware stays capability-agnostic — only the module knows the unit.

### Composition

`cmd/openai-worker-node/main.go` is the only place that constructs the prom impl. When `--metrics-listen` is set: build the Recorder, start the listener, wrap `payeedaemon` and `tokenizer` globally via their `WithMetrics` constructors, and inject the Recorder into modules (which wrap `backendhttp` per-`(capability, model)` per request via `backendFor(model)`). When not set: noop everywhere; no listener.

## Cross-repo notes

- The `unit` label values are stable across the worker and the bridge — `livepeer_bridge_revenue_usd_cents_total` joined with `livepeer_worker_work_units_total` over the same `(capability, model, time-window)` is the margin reconciliation. See [`../../../openai-livepeer-bridge/docs/design-docs/metrics.md`](../../../openai-livepeer-bridge/docs/design-docs/metrics.md).
- `worker.yaml` is the source of truth for the `(capability, model)` pairs that appear as labels. Adding a new model = new label values; bounded and expected.
