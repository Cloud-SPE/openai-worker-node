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

### payment-middleware-check-impl
**Opened:** 2026-04-24 (plan 0001)
**Context:** Custom lint that verifies every capability-module registration passes through `runtime/http.RegisterPaidRoute`. Placeholder README in place. One paid module (chat) exists today; the lint needs a second module before its detection logic is meaningfully exercised.
**Resolution target:** Unclaimed — fold into the plan that adds 0004-embeddings-module.

### quote-quotes-unpaid-routes
**Opened:** 2026-04-24 (plan 0003)
**Context:** `GET /quote?sender=&capability=` and `GET /quotes?sender=` are part of the worker HTTP contract (`docs/product-specs/index.md`). Not implemented in 0003 because they need a new `GetQuote` method on the `payeedaemon.Client` provider and a corresponding fake method. Small scope but its own plan.
**Resolution target:** Unclaimed — short follow-up plan.

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

## Resolved

_None yet._
