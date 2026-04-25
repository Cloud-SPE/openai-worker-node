---
id: 0008
slug: metrics-phase-1
title: Metrics Phase 1 — Recorder provider, /metrics listener, request + payment + capacity + backend instrumentation
status: active
owner: agent
opened: 2026-04-25
---

## Goal

Wire a `Recorder` provider (Prometheus impl backed by `prometheus/client_golang`), expose it on a configurable TCP `/metrics` listener, and instrument the request middleware + capability modules + backend HTTP client + payee-daemon gRPC client with the Phase 1 catalog from [`docs/design-docs/metrics.md`](../../design-docs/metrics.md). Mirrors the verified pattern in [`livepeer-service-registry`](../../../../livepeer-service-registry/docs/design-docs/observability.md) wholesale — same package layout, same flag names, same dual-histogram pattern, same cardinality cap.

This is the worker-side third of the cross-repo metrics rollout. Pairs with [`livepeer-payment-library/docs/exec-plans/0019-metrics-phase-1.md`](../../../../livepeer-payment-library/docs/exec-plans/0019-metrics-phase-1.md) and [`openai-livepeer-bridge/docs/exec-plans/active/0021-metrics-phase-1.md`](../../../../openai-livepeer-bridge/docs/exec-plans/active/0021-metrics-phase-1.md). Each ships independently — no compile-time cross-repo deps — but consistent label keys make the bridge's reconciliation panels work.

Authoritative cross-repo conventions: [`../../../../livepeer-modules-conventions/metrics-conventions.md`](../../../../livepeer-modules-conventions/metrics-conventions.md).

## Non-goals

- No streaming TTFT histogram, no separate stream-lifecycle counter (Phase 2). Phase 1 rolls stream cancels into `livepeer_worker_requests_total{outcome=canceled}`.
- No reconcile-delta histogram (Phase 2).
- No `livepeer_worker_backend_request_duration_seconds` per-backend-URL labeling — `capability + model` is the right grouping.
- No auth / TLS on `/metrics`. Reverse-proxy if needed.

## Approach

Package layout follows service-registry: `internal/providers/metrics/` (Recorder + impls) + `internal/runtime/metrics/` (TCP listener). Per-provider decorators live next to the provider they wrap. Per the conventions doc, **no service or repo package may import `prometheus/client_golang` directly** — emissions go through the Recorder interface, enforced by depguard.

### Provider package

- [ ] `internal/providers/metrics/recorder.go` — `Recorder` interface (`Counter`, `Gauge`, `Histogram` constructors with `name + help + labels` signature). Returned types expose `WithLabelValues(...)` returning `Inc()` / `Add(float64)` / `Set(float64)` / `Observe(float64)`.
- [ ] `internal/providers/metrics/cardinality.go` — `sync.Map`-backed wrapper that drops new label tuples beyond `--metrics-max-series-per-metric` (default `10000`, `0` = disabled). `atomic.CompareAndSwap` on `unix-second/60` stamp gates one WARN per (metric, ~1-min violation block). Wrapper is the testable surface.
- [ ] `internal/providers/metrics/buckets.go` — `DefaultBuckets = prometheus.DefBuckets`; `FastBuckets = []float64{0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 1}`. Plus a `RegisterDualHistogram(name, help, labels)` helper that constructs both `<name>_seconds` (Default) and `<name>_seconds_fast` (Fast) and returns an Observer that writes to both.
- [ ] `internal/providers/metrics/names.go` — exported metric-name constants used by direct-injection sites and decorators.
- [ ] `internal/providers/metrics/noop.go` — default for tests + endpoint-off mode.
- [ ] `internal/providers/metrics/prometheus.go` — `prometheus/client_golang` impl with custom `*prometheus.Registry` + standard collectors.

### Runtime package (TCP listener)

- [ ] `internal/runtime/metrics/listener.go` — separate stdlib `net/http` server (NOT mounted on the existing `/health` mux). Routes: `GET /metrics` + `GET /healthz`. Graceful shutdown integrated with the existing worker lifecycle.
- [ ] **Flag** `--metrics-listen` (string, default empty = OFF). Recommended `:9093` per [`port-allocation.md`](../../../../livepeer-modules-conventions/port-allocation.md).
- [ ] **Flag** `--metrics-max-series-per-metric` (int, default `10000`).
- [ ] **`.env.example` + `compose.yaml` + `compose.prod.yaml`** — `METRICS_PORT=9093` (commented). Compose entrypoint composes `--metrics-listen=:${METRICS_PORT}` when set.

### Per-provider decorators (each in its own `metered.go`)

- [ ] `internal/providers/payeedaemon/metered.go` → `livepeer_worker_daemon_rpc_calls_total{method,outcome}` (counter), `livepeer_worker_daemon_rpc_duration_seconds{method,outcome}` (Default buckets), `livepeer_worker_daemon_rpc_duration_seconds_fast{method,outcome}` (Fast buckets) — dual-histogram via `RegisterDualHistogram` since unix-socket gRPC. `method` ∈ {`ProcessPayment`, `DebitBalance`, `GetQuote`, `ListCapabilities`}.
- [ ] `internal/providers/backendhttp/metered.go` → `livepeer_worker_backend_requests_total{capability,model,outcome}`, `livepeer_worker_backend_request_duration_seconds{capability,model}` (Default buckets), `livepeer_worker_backend_errors_total{capability,model,error_class}`, `livepeer_worker_backend_last_success_timestamp_seconds{capability,model}`. The `error_class` taxonomy (`timeout`, `5xx`, `malformed`, `connect`) already exists implicitly in the slog calls; Phase 1 promotes it to a label.
- [ ] `internal/providers/tokenizer/metered.go` → `livepeer_worker_tokenizer_calls_total{model,outcome}`. Latency intentionally skipped — typically <100 µs.

### HTTP middleware (customer-facing surface)

- [ ] `internal/runtime/http/middleware.go` — extend `paymentMiddleware` with a metrics observation pass. Emits per-request: `livepeer_worker_requests_total{capability,model,outcome}`, `livepeer_worker_request_duration_seconds{capability,model,outcome}` (Default buckets), `livepeer_worker_payment_rejections_total{reason}`, `livepeer_worker_capacity_rejections_total{capability}`. The `livepeer_worker_inflight_requests` gauge is updated in semaphore acquire (`Inc()`) and the deferred release (`Dec()`).

### Direct Recorder injection in capability modules

- [ ] Add `Unit() string` method to the capability module surface. Values: `chat`/`embeddings` → `token`, `audio.speech` → `character`, `audio.transcriptions` → `audio_second`, `images.{generations,edits}` → `image_step_megapixel`. The middleware stays capability-agnostic — only the module knows the unit.
- [ ] In each module's `Serve`, after a successful response, emit `livepeer_worker_work_units_total{capability,model,unit}` with the actual units consumed.
- [ ] `livepeer_worker_build_info` gauge=1 with labels `version` (from `runtime/build`), `protocol_version` (from the existing health handler), `go_version`. Set once at construction.
- [ ] `livepeer_worker_max_concurrent` gauge set once at construction from `Config.MaxConcurrentRequests`.

### Composition root

- [ ] `cmd/openai-worker-node/main.go` — when `--metrics-listen` is set: build the prom Recorder, start the listener, wrap each provider with its `metered.go` constructor. Otherwise pass the noop Recorder and skip the listener.

### Tests

- [ ] Unit: `cardinality.go` wrapper (drops at threshold, one WARN per block — uses fake clock).
- [ ] Unit: prom impl (custom registry, `FastBuckets` and `DefaultBuckets` correctly applied; `RegisterDualHistogram` wires both observers).
- [ ] Unit: each `metered.go` decorator (table-driven, one row per method × outcome).
- [ ] Unit: backend-error classification — timeout / 5xx / connect / malformed each map to the right `error_class` label.
- [ ] Middleware integration: each `outcome` value (2xx, 4xx, 402, 5xx, canceled) emitted by the right path. Existing harness mocks the payee daemon.
- [ ] End-to-end: boot worker with `--metrics-listen=127.0.0.1:0`, drive one chat request, assert `GET /metrics` returns 200 + contains `livepeer_worker_requests_total{outcome="2xx"}`, `livepeer_worker_work_units_total{unit="token"}`, `livepeer_worker_daemon_rpc_calls_total{method="ProcessPayment",outcome="ok"}`, AND both histograms (`_seconds` and `_seconds_fast`) for the daemon RPC.

### Docs + tracker

- [ ] `docs/operations/running-with-docker.md` — `--metrics-listen` and `--metrics-max-series-per-metric` rows; new "Observability" subsection with sample scrape config + link to the conventions doc.
- [ ] `docs/design-docs/architecture.md` — add `internal/providers/metrics/` and `internal/runtime/metrics/` to the package layout.
- [ ] `worker.example.yaml` — note that `METRICS_PORT` is set in the worker's deployment env, not in `worker.yaml` (the shared daemon-worker config).
- [ ] `docs/exec-plans/tech-debt-tracker.md` — append an entry pointing to this plan, closing the implicit "no metrics" gap for Phase 1 scope.

## Decisions log

### 2026-04-25 — Mirror service-registry's verified pattern wholesale

Same call as the daemon's 0019. service-registry has shipped this exact shape at `status: verified`. Same package split, same flag names, same dual-histogram pattern, same cardinality cap default. Rule of three: 1 verified + 2 anticipated copies (this + daemon) doesn't trigger extraction yet.

### 2026-04-25 — Provider/runtime package split

Recorder is a provider; listener is runtime. Two layers, two responsibilities. Matches the existing depguard layering — services depend on the provider's interface, only `cmd/` and `runtime/` touch the impl + listener. Earlier draft of this plan combined them into one `internal/runtime/metrics/`; corrected to mirror service-registry.

### 2026-04-25 — Per-provider `metered.go`, NOT centralized decorators

Each provider package owns its own observability surface — `payeedaemon/metered.go`, `backendhttp/metered.go`, `tokenizer/metered.go`. Adding a method to the payee daemon interface = update `metered.go` in the same package. Matches service-registry's `internal/providers/{chain,manifestfetcher,audit}/metered.go` layout.

### 2026-04-25 — Per-domain decorators with `method`/`op` labels

`livepeer_worker_daemon_rpc_*` for unix-socket gRPC, `livepeer_worker_backend_*` for over-the-wire upstream HTTP, `livepeer_worker_tokenizer_*` for the in-process tokenizer. Each is its own dashboard section. By the Prometheus rule of thumb, `sum()` over each metric should be meaningful — these all pass.

### 2026-04-25 — Dual-histogram for `livepeer_worker_daemon_rpc_duration_seconds`

Same `Observe()` writes to both `_seconds` (Default) and `_seconds_fast` (Fast). Matches the conventions doc's gRPC pattern. Daemon RPC is unix-socket-only today (sub-ms typical) but consistency with the fleet wins; cost is 2× histogram series for one specific metric. Other histograms (request, backend) stay single — over-the-wire HTTP doesn't benefit from sub-ms resolution.

### 2026-04-25 — Separate TCP listener (not mounted on the existing API port)

Earlier draft had `/metrics` on the same port as `/health` and `/capabilities`. Switched for parity with the daemon and bridge — operators can bind `/metrics` to localhost-only or a security zone without touching customer-facing config. Default `:9093` per port-allocation doc.

### 2026-04-25 — Cardinality cap as a wrapper

Same shape as daemon and service-registry: `sync.Map`-backed wrapper, default 10000, lock-free WARN gate. Catches slipped high-cardinality labels (e.g., a slipped `customer_id`) without breaking the metric.

### 2026-04-25 — `livepeer_worker_*` prefix (not `worker_*`)

Earlier draft used `worker_*` for compactness. Switched to `livepeer_worker_*` for fleet consistency — matches `livepeer_registry_*`, `livepeer_payment_*`, `livepeer_bridge_*`. A single Grafana datasource scraping all four services uses `livepeer_*` as the umbrella.

### 2026-04-25 — `unit` label sourced from the capability module

Module owns the unit dimension (chat = `token`, audio.transcriptions = `audio_second`, etc.). `Module.Unit() string` keeps the middleware capability-agnostic. Adding a new capability doesn't need a middleware change. Required also for the cross-repo reconciliation: `livepeer_bridge_revenue_usd_cents_total ÷ rate-card` joins with `livepeer_worker_work_units_total` only when the `unit` value matches.

### 2026-04-25 — `outcome=canceled` is the catch-all for stream aborts in Phase 1

Distinguishing "client disconnected" from "backend hung up" requires plumbing context cancellation through the streaming modules — Phase 2. Phase 1 buckets both as `canceled` and lets `livepeer_worker_backend_errors_total{error_class}` catch the backend cause separately.

### 2026-04-25 — `livepeer_worker_inflight_requests` set inside semaphore acquire/release

The existing semaphore is a buffered channel; `len()` already gives the depth and `/health` reports it. Phase 1 mirrors that as a gauge — `Inc()` on acquire, `Dec()` on the deferred release. The existing `defer semaphore.Release()` pattern is panic-safe.

## Open questions

All resolved.

## Artifacts produced

(To be filled in on close.)
