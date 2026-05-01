# Tech-debt tracker

Append-only log of known debt. Mark resolved entries with strikethrough and a resolving-plan link. Never delete entries — history matters.

## Format

```
### <slug>
**Opened:** YYYY-MM-DD (plan <NNNN>)
**Context:** What was deferred and why.
**Resolution target:** Plan number, or "unclaimed".
```

## Open

### layer-check-full-impl
**Opened:** 2026-04-24 (plan 0001)
**Context:** Custom lint `lint/layer-check` is a README placeholder. Full AST-walking `go vet` analyzer deferred until the source tree has real code and the concrete import patterns are known.
**Resolution target:** Unclaimed — now unblocked (0002 + 0003 landed real code). Targeted for a dedicated plan when a second module introduces the risk of cross-module imports.

### ~~payment-middleware-check-impl~~ — resolved 2026-04-24
**Opened:** 2026-04-24 (plan 0001)
**Context:** Custom lint that verifies every capability-module registration passes through `runtime/http.RegisterPaidRoute`. Placeholder README in place. One paid module (chat) exists today; the lint needs a second module before its detection logic is meaningfully exercised.
**Resolved:** Implemented at `lint/payment-middleware-check/`. AST-walking Go program; flags `.Register(method, path, handler)` calls with a string-literal path starting with `/v1/`. Seven self-tests including a regression guard against the live repo. Wired into `make lint-custom`.

### ~~quote-quotes-unpaid-routes~~ — resolved 2026-04-24
**Opened:** 2026-04-24 (plan 0003)
**Context:** `GET /quote?sender=&capability=` and `GET /quotes?sender=` are part of the worker HTTP contract (`docs/product-specs/index.md`). Not implemented in 0003 because they need a new `GetQuote` method on the `payeedaemon.Client` provider and a corresponding fake method. Small scope but its own plan.
**Resolved:** `GetQuote` added to `payeedaemon.Client` (+ gRPC impl + Fake). `/quote` and `/quotes` handlers live in `internal/runtime/http/handlers.go`; both registered by `RegisterUnpaidHandlers`. Byte-fields rendered as `0x`-prefixed hex for bridge compatibility. 7 tests in `quote_test.go` covering happy/missing-sender/bad-sender/missing-capability/daemon-error + /quotes happy + fail-closed-on-any-error.

### ~~concurrency-limiter~~ — resolved 2026-04-24
**Opened:** 2026-04-24 (plan 0003)
**Context:** `worker.max_concurrent_requests` is parsed and surfaced in `/health` but not enforced. The 503 capacity_exhausted response path in `docs/product-specs/index.md` has no producer. A semaphore-wrapped paid-route handler closes the gap.
**Resolved:** `Mux.paidSem` is a buffered channel sized from `cfg.Worker.MaxConcurrentRequests` (fallback 1 on non-positive). `paymentMiddleware` attempts a non-blocking send on entry; failure returns 503 `capacity_exhausted`. Unpaid routes (/health, /capabilities, /quote, /quotes) bypass the cap. /health now reports live `inflight` + `max_concurrent`. 5 limiter tests including concurrent pin-and-reject via a blocking fake module.

### recipient-rand-hash-work-id
**Opened:** 2026-04-24 (plan 0003)
**Context:** `runtime/http.deriveWorkID` hashes paymentBytes with sha256 for the daemon's work_id field. The daemon derives its own session key (RecipientRandHash) from the payment's ticket_params; using that directly in work_id would align worker/daemon on a single session identity and make log correlation trivial. Requires the middleware to unmarshal the Payment proto locally.
**Resolution target:** Low priority — current scheme works; revisit if log correlation becomes a pain point.

### ~~real-tokenizer~~ — resolved 2026-04-25
**Opened:** 2026-04-24 (plan 0002)
**Context:** `providers/tokenizer/NewWordCount` is a word-split + 1.33× multiplier placeholder. tiktoken-go / sentencepiece provider swap would tighten metering accuracy for models where over-debit drag becomes measurable.
**Resolved:** New `tokenizer.NewTiktoken(fallback)` impl backed by `github.com/pkoukk/tiktoken-go` (`internal/providers/tokenizer/tiktoken.go`). Uses per-model encodings for OpenAI families (gpt-3.5/4/4o, embedding-3-*); unknown models fall through to cl100k_base; only initialization failure delegates to the word-count fallback. The `Tokenizer` interface gained `CountTokensForModel(model, s)`; `chat_completions` and `embeddings` modules now thread the model into their estimates. Wired in `cmd/openai-worker-node/main.go` as `NewTiktoken(NewWordCount(133))`. Tests: 5 new in `tiktoken_test.go` covering known model + blank model + unknown-model-default-fallback + per-model cache + concurrent safety; one new word-count test asserts the model-blind delegation invariant.

### ~~multipart-capability-handling~~ — resolved 2026-04-24
**Opened:** 2026-04-24 (plans 0005, 0006)
**Context:** `/v1/images/edits` (image+mask upload) and `/v1/audio/transcriptions` (audio upload) both take multipart/form-data bodies. The current Module.ExtractModel signature assumes `body []byte` is JSON; multipart needs parsing to extract the `model` field and (for transcriptions) audio duration metadata for metering. Rather than ship two divergent implementations, capture the shared requirement and design the multipart path once.
**Resolved:** Shared helper `internal/service/modules/multipartutil/` — boundary extraction from raw body (no Module interface change), + `FormField` reader. `backendhttp.Client` gained `DoRaw(ctx, url, contentType, body)` so modules forward the caller's multipart Content-Type + boundary to the backend unchanged. Landed `/v1/images/edits` (same metering formula as images_generations, sourced from form fields) and `/v1/audio/transcriptions` (tier-max reservation; reconciles via `duration` in the backend's verbose_json response when present).

### ~~audio-speech-content-type-passthrough~~ — resolved 2026-04-24
**Opened:** 2026-04-24 (plan 0006)
**Context:** `audio_speech.Module.Serve` hardcodes `Content-Type: audio/mpeg` because `backendhttp.DoStream` doesn't currently expose the backend's response headers. Backends that emit WAV/Opus/etc. get mislabeled for the client. The fix is to widen `DoStream` to return the response `http.Header` or at least the `Content-Type`.
**Resolved:** `backendhttp.Client.DoStream` now returns `(status, headers, stream, err)`. `audio_speech.Module.Serve` reads `Content-Type` from the returned headers, falling back to `audio/mpeg` when absent. Chat completions ignores headers (always sets `text/event-stream`). Fetch impl + Fake updated to carry the header through. Two new tests assert relay + fallback.

### ~~no-prometheus-metrics~~ — resolved 2026-04-25
**Opened:** 2026-04-25 (Phase 1 metrics)
**Context:** The worker exposed `/health` + `/capabilities` only. Operators had no Prometheus surface — no request throughput, no work-units counter, no daemon-RPC latency, no backend health, no payment-rejection breakdown. The bridge's margin reconciliation needs `livepeer_worker_work_units_total` to join against revenue.
**Resolved:** Phase 1 metrics rolled out across two passes. Pass A (commits `413eaa5`, `ed9d97e`, `ce30419`) shipped the scaffold: `Recorder` interface (`internal/providers/metrics`), Prometheus + Noop impls, TCP listener (`internal/runtime/metrics`), and three `WithMetrics(...)` decorators on payeedaemon, backendhttp (per-(capability, model)), tokenizer. Pass B (this commit) activated the scaffold: `--metrics-listen` + `--metrics-max-series-per-metric` flags, composition root in `cmd/openai-worker-node/main.go`, Recorder injection through the `Mux` into `paymentMiddleware` (request lifecycle, capacity rejection, payment rejection, inflight gauge, work units), `Unit() string` on every capability module, and per-call backend wrapping inside each module's Serve. `.env.example`, `compose.yaml`, `compose.prod.yaml` and `docs/operations/running-with-docker.md` updated. Phase 2 (streaming TTFT, reconcile-delta histograms) tracked separately.

### coverage-gate-exemptions
**Opened:** 2026-04-27 (plan 0009)
**Context:** Plan 0009 enforces `core-beliefs.md` invariant 6 (≥75% coverage per package) as a CI gate via `.github/workflows/test.yml`. At gate-introduction time, four packages sit below 75% and are explicitly exempted in the workflow's `EXEMPT` env var. Each is a separate small effort to address — bundling them into 0009 would have explode-bucket-3'd the plan. Per the invariant's wording ("exempt only when explicitly listed with a written reason and a tracking issue"), the exemption is recorded here.

Exempted packages and reasons:

| Package | Coverage at gate-introduction | Reason |
|---|---|---|
| `internal/providers/payeedaemon` | 63.1% | Production gRPC client; remaining uncovered surface is connection-error / Dial paths. Needs a real gRPC server (or richer mock) to exercise. |
| `internal/providers/backendhttp` | 66.7% | HTTP client wrapper; uncovered paths are streaming-error and non-2xx body propagation. Needs httptest fixtures. |
| `internal/config` | 72.5% | Config parsing + projection. Uncovered paths are mostly `Validate` error branches; closing the gap is mechanical fixture work. |
| `lint/payment-middleware-check` | 73.3% | Custom AST analyzer. Uncovered paths are degenerate-AST safety branches. Hard to construct synthetic ASTs that hit them. |

**Resolution target:** One small dedicated plan per package (or one plan covering all four). Plan 0010 is the natural successor. Any package brought above 75% should be removed from the `EXEMPT` env var in `.github/workflows/test.yml` in the same PR that adds the tests.

### payment-ticket-params-proxy
**Opened:** 2026-04-30 (unclaimed)
**Context:** The payments/modules side clarified that v3 keeps manifest/resolver for pricing, but workers still need one cryptographic helper route: `POST /v1/payment/ticket-params`. This is **not** a pricing endpoint and must not revive `/quote` semantics. The worker should:

- require the same optional bearer-token pattern as `/registry/offerings` (reuse `auth_token` when configured)
- validate a request body shaped like:
  - `sender_eth_address`
  - `recipient_eth_address`
  - `face_value_wei`
  - `capability`
  - `offering`
- proxy the request over the local unix socket to receiver-mode `livepeer-payment-daemon`
- return the daemon's full canonical ticket-params response unchanged enough for sender-side ticket minting
- avoid any worker-local crypto, pricing, manifest lookup, or caching

Expected HTTP behavior:

- `401` on missing/mismatched bearer token when auth is enabled
- `400` on malformed request
- `503` when the local payment-daemon is unavailable
- `5xx` when the receiver daemon cannot issue valid params

Upstream dependency: `livepeer-modules-project/payment-daemon` must first expose the new receiver-mode RPC and own the canonical request/response schema. This repo should implement only the thin HTTP-to-daemon proxy once that contract lands.
**Resolved:** Implemented in worker plan `0015-payment-ticket-params-proxy`. The worker now exposes `POST /v1/payment/ticket-params`, reuses `auth_token`, validates the JSON request, proxies to `PayeeDaemon.GetTicketParams`, and returns the daemon-issued canonical `ticket_params` object. Custom lint updated to allow this one unpaid `/v1/*` helper route.

## Resolved

_None yet._
