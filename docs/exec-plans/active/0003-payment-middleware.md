---
id: 0003
slug: payment-middleware
title: Payment middleware + paid-route registration surface
status: active
owner: unclaimed
opened: 2026-04-24
---

## Goal

Land the `runtime/http` package with the routing + payment-middleware pipeline every capability module plugs into. After this lands, adding a capability is a small, contained change (implement the `Module` interface, call `RegisterPaidRoute`). Before this lands, nothing paid can ship.

This is the repo's highest-stakes piece because core belief #3 ("payment is auth") is implemented here. The code path from "HTTP request arrives" to "module gets called" is the trust boundary — if a path around the middleware exists, every capability that ever lands is compromised.

## Non-goals

- No capability modules. Those are 0002 / 0004 / 0005 / 0006.
- No backend HTTP client. Lives in `providers/backendhttp/`; first consumer (0002) wires its default.
- No tokenizer. Lives in `providers/tokenizer/`; first consumer (0002) wires its default.

## Cross-repo dependencies

- Library `0018-per-capability-pricing` (the `PayeeDaemonClient` this middleware uses must speak the new RPC set).

## Approach

- [x] `internal/providers/payeedaemon/` interface + `grpc` impl over unix socket.
  - `interface.go` — Client interface (ListCapabilities, ProcessPayment, DebitBalance, Close).
  - `grpc.go` — unix-socket client; maps proto ↔ domain types at the boundary.
  - `types.go` — domain projections (ProcessPaymentResult, DebitBalanceResult, ListCapabilitiesResult, Capability, ModelPrice).
  - `fake.go` — concurrency-safe in-memory fake; running ledger; error-injection knobs.
- [x] `internal/types/` — CapabilityID, ModelID, WorkUnit, WorkID, PaymentHeaderName constants.
- [x] `internal/config/` — worker-side Config with flat (capability, model) → ModelRoute map; `FromShared`, `Load`, `Lookup`, `VerifyDaemonCatalog`.
- [x] `internal/service/modules/` — Module interface (capability-level; ExtractModel + EstimateWorkUnits + Serve).
- [x] `internal/runtime/http/` package:
  - `Server` — net/http wrapper with `Start` + `Shutdown`.
  - `Mux` — wraps `http.ServeMux`; `Register` (unpaid) + `RegisterPaidRoute` (wrapped in middleware); duplicate-route panic; `HasPaidCapability` tracker.
  - `paymentMiddleware` — full pipeline: body → header → base64 → work_id → ProcessPayment → ExtractModel → Lookup → Estimate → DebitBalance(est) → Serve → DebitBalance(delta) if actual > est.
  - Reconciliation: over-debit only. If `actual > est`, second `DebitBalance(delta)`. If `actual < est`, no-op (accepted as payee profit per PRODUCT_SENSE).
  - `workid.go` — `deriveWorkID(paymentBytes) = sha256-hex` (swappable once we unmarshal the Payment proto to use RecipientRandHash directly — tracked).
  - `handlers.go` — `/health` + `/capabilities` unpaid handlers; backend_url intentionally omitted from `/capabilities` output.
- [ ] Startup cross-check: `PayeeDaemon.ListCapabilities` called once at main wiring; `config.VerifyDaemonCatalog` compares. *(Function in place; the actual startup orchestration lives in cmd/openai-worker-node — follow-up with the Server wiring.)*
- [x] Error contract per `docs/product-specs/index.md`: 402 (missing/rejected payment, insufficient balance), 404 (capability_not_found), 400 (invalid_request / estimator failure), 502 (DebitBalance transport error / backend_unavailable). 503 (capacity_exhausted) lands with the concurrency limiter — follow-up.
- [x] Tests: 17 middleware cases covering every error branch + happy path + work_id stability (same/different blobs) + duplicate route panic + HasPaidCapability + health + capabilities. 6 config tests for projection + daemon-catalog verification.
- [ ] cmd/openai-worker-node main wiring — assemble Config + PayeeDaemon + Mux + register unpaid handlers + register capability modules + Start. *(Gated on the first capability module landing in 0002.)*

Coverage after this plan: runtime/http 83.3%, config 72.5%. No custom lints added yet — `payment-middleware-check` is deferred until a second capability module exists to validate the lint's detection logic against.

## Decisions log

_Empty._

## Open questions

- **`work_id` derivation.** Libraries treat it as opaque; here we need to pick a value. Strawman: `RecipientRandHash.Hex()` from the payment bytes (matches what the payer daemon uses). Confirm.
- **Concurrency limit enforcement.** `max_concurrent_requests` → reject with 503 or queue? Leaning reject; queueing adds tail-latency debt we don't want.

## Artifacts produced

In-flight. Files landed:

- `internal/providers/payeedaemon/{doc,interface,types,grpc,fake}.go`
- `internal/types/{doc,capability,payment}.go`
- `internal/config/{doc,config,verify,config_test}.go`
- `internal/service/modules/{doc,module}.go`
- `internal/runtime/http/{doc,server,mux,middleware,handlers,workid,middleware_test}.go`
- `go.mod` / `go.sum` — library dep materialized via the `replace` directive.

Deferred for follow-up plans:

- cmd/openai-worker-node main wiring (gated on first capability module in 0002).
- `/quote` and `/quotes` unpaid routes (thin proxy on top of PayeeDaemon.GetQuote).
- Concurrency limiter + 503 path.
- Payment-middleware-check custom lint.
- RecipientRandHash-based work_id (once middleware unmarshals Payment proto).
