---
title: Capability modules
status: accepted
last-reviewed: 2026-04-25
---

# Capability modules

The capability-module pattern is how this worker adds OpenAI-shaped (or any other) endpoints. One module = one capability string = one HTTP route = one billing dimension.

## The contract

Every module satisfies `internal/service/modules.Module`:

```go
type Module interface {
    Capability() types.CapabilityID
    HTTPMethod() string
    HTTPPath()   string

    ExtractModel(body []byte) (types.ModelID, error)
    EstimateWorkUnits(body []byte, model types.ModelID) (int64, error)

    Serve(ctx, w, r, body, model, backendURL string) (actualUnits int64, err error)
}
```

The capability string is the source-of-truth identifier shared with the daemon (`sharedyaml.CapabilityConfig.Capability`) and the bridge (`capabilityString(...)`). Mismatches are caught at startup by the catalog cross-check.

## Layer rule

Modules live under `internal/service/modules/<name>/` and may import:

- `internal/types`
- `internal/providers/...` (the cross-cutting interfaces — tokenizer, backendhttp)
- `internal/service/modules` (the parent interface)
- `internal/service/modules/multipartutil` (shared multipart helpers)

Modules MUST NOT import `internal/runtime/...`, `internal/providers/payeedaemon`, `internal/config`, or sibling capability packages. The dependency rule is enforced by the `lint/layer-check` analyzer; see [architecture.md](architecture.md).

## The middleware seam

`internal/runtime/http.RegisterPaidRoute(module)` is the only public way to mount a paid route. It wires the module into `paymentMiddleware` (`internal/runtime/http/middleware.go`), which closes over the daemon client, config, semaphore, and logger.

The middleware's flow per request:

```
1. Acquire paid-route semaphore (non-blocking; 503 on miss)
2. Read body (bounded by maxPaidRequestBodyBytes = 16 MiB)
3. Decode `livepeer-payment` header (base64 → proto bytes)
4. Derive work_id = sha256(payment bytes)
5. PayeeDaemon.ProcessPayment(payment, work_id) → { sender, balance, ... }
6. module.ExtractModel(body) → ModelID
7. config.Lookup(capability, model) → { backend_url }
8. module.EstimateWorkUnits(body, model) → int64
9. PayeeDaemon.DebitBalance(sender, work_id, estimate) → balance check (negative ⇒ 402)
10. module.Serve(ctx, w, r, body, model, backendURL) → actualUnits
11. Reconcile: actualUnits > estimate ⇒ DebitBalance(delta)
```

A module that fails at step 10 may have already written response bytes — the middleware logs and returns rather than try to rewrite the wire. This is deliberate: streaming modules cannot un-emit headers.

## What a module owns

| Concern                           | Module | Middleware |
| --------------------------------- | :----: | :--------: |
| Body parsing                      |   ✓    |            |
| Model selection                   |   ✓    |            |
| Work-unit estimation              |   ✓    |            |
| Backend dispatch (URL construction, request shape) | ✓ |   |
| Response writing (headers + body) |   ✓    |            |
| Streaming framing                 |   ✓    |            |
| Multipart parsing (if applicable) |   ✓    |            |
| Payment validation                |        |     ✓      |
| Concurrency cap                   |        |     ✓      |
| Body byte cap (16 MiB)            |        |     ✓      |
| Reconciliation debit              |        |     ✓      |
| Error envelope shape              |        |     ✓      |

## What's deliberately not in the contract

- **No tier knowledge.** Modules don't know whether a request is on free or prepaid tier — that's the bridge's responsibility. A free request that survives the bridge's tier gate gets to the worker the same as any prepaid request.
- **No customer identity.** Modules see `sender` (the bridge's payer ETH address) only via the middleware; they never see end-customer identifiers.
- **No retry logic.** If the backend errors, the module returns; the middleware logs. The bridge owns retry at its layer.
- **No refund path on the worker.** The over-debit-accepted policy means actual < estimate is silently ignored. Only actual > estimate triggers a second debit.

## Adding a new capability

1. Create `internal/service/modules/<name>/`.
2. Implement `Module`. Constants for `Capability` and `HTTPPath` go at package top so lints + tests reference them by name.
3. Add a request-shape file (`types.go`) if the body parsing is non-trivial.
4. Wire into `cmd/openai-worker-node/main.go` after the existing modules: `mux.RegisterPaidRoute(<name>.New(backend))`.
5. Add an entry in [`docs/product-specs/index.md`](../product-specs/index.md) and a per-route spec.
6. Add an exec-plan in `docs/exec-plans/active/` if the module brings a new request shape (multipart, streaming bytes, websocket) — the convention from [PLANS.md](../../PLANS.md) applies.

## Cross-references

- Per-capability metering: [metering.md](metering.md).
- Streaming framing: [streaming.md](streaming.md).
- Cross-repo capability-string namespace: [`../../../livepeer-payment-library/docs/design-docs/shared-yaml.md`](../../../livepeer-payment-library/docs/design-docs/shared-yaml.md).
