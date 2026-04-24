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

### concurrency-limiter
**Opened:** 2026-04-24 (plan 0003)
**Context:** `worker.max_concurrent_requests` is parsed and surfaced in `/health` but not enforced. The 503 capacity_exhausted response path in `docs/product-specs/index.md` has no producer. A semaphore-wrapped paid-route handler closes the gap.
**Resolution target:** Unclaimed — small dedicated plan.

### recipient-rand-hash-work-id
**Opened:** 2026-04-24 (plan 0003)
**Context:** `runtime/http.deriveWorkID` hashes paymentBytes with sha256 for the daemon's work_id field. The daemon derives its own session key (RecipientRandHash) from the payment's ticket_params; using that directly in work_id would align worker/daemon on a single session identity and make log correlation trivial. Requires the middleware to unmarshal the Payment proto locally.
**Resolution target:** Low priority — current scheme works; revisit if log correlation becomes a pain point.

### real-tokenizer
**Opened:** 2026-04-24 (plan 0002)
**Context:** `providers/tokenizer/NewWordCount` is a word-split + 1.33× multiplier placeholder. tiktoken-go / sentencepiece provider swap would tighten metering accuracy for models where over-debit drag becomes measurable.
**Resolution target:** Low priority — over-debit policy absorbs the drift; swap when real production traffic shows a customer-visible impact.

### ~~multipart-capability-handling~~ — resolved 2026-04-24
**Opened:** 2026-04-24 (plans 0005, 0006)
**Context:** `/v1/images/edits` (image+mask upload) and `/v1/audio/transcriptions` (audio upload) both take multipart/form-data bodies. The current Module.ExtractModel signature assumes `body []byte` is JSON; multipart needs parsing to extract the `model` field and (for transcriptions) audio duration metadata for metering. Rather than ship two divergent implementations, capture the shared requirement and design the multipart path once.
**Resolved:** Shared helper `internal/service/modules/multipartutil/` — boundary extraction from raw body (no Module interface change), + `FormField` reader. `backendhttp.Client` gained `DoRaw(ctx, url, contentType, body)` so modules forward the caller's multipart Content-Type + boundary to the backend unchanged. Landed `/v1/images/edits` (same metering formula as images_generations, sourced from form fields) and `/v1/audio/transcriptions` (tier-max reservation; reconciles via `duration` in the backend's verbose_json response when present).

### audio-speech-content-type-passthrough
**Opened:** 2026-04-24 (plan 0006)
**Context:** `audio_speech.Module.Serve` hardcodes `Content-Type: audio/mpeg` because `backendhttp.DoStream` doesn't currently expose the backend's response headers. Backends that emit WAV/Opus/etc. get mislabeled for the client. The fix is to widen `DoStream` to return the response `http.Header` or at least the `Content-Type`.
**Resolution target:** Small follow-up; bundle with any future `DoStream` surface change.

## Resolved

_None yet._
