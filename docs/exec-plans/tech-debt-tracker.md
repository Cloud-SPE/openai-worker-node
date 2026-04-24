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
**Resolution target:** Unclaimed — lift once 0002-0003 have landed.

### payment-middleware-check-impl
**Opened:** 2026-04-24 (plan 0001)
**Context:** Custom lint that verifies every capability-module registration passes through `runtime/http.RegisterPaidRoute`. Placeholder README in place. Full implementation deferred until the module registration surface exists in code.
**Resolution target:** Unclaimed — targeted for a plan after 0003-payment-middleware lands.

## Resolved

_None yet._
