---
id: 0018
slug: payee-session-contract-v4-0-1
title: Align worker payee flow with payment-daemon v4.0.1 session contract
status: active
owner: codex
opened: 2026-05-03
---

## Goal
Update `openai-worker-node` so paid request handling conforms to the `livepeer-payment-daemon` `v4.0.1` payee contract. The worker must explicitly open a payee-side session before processing payment, debit against that session during work, and close the session at request completion for the current request/response-only operating model.

## Non-goals
- No multi-request prepaid-session reuse in this plan.
- No pricing-model redesign or payment-daemon changes.
- No edits to completed plans.

## Cross-repo dependencies
- `livepeer-modules-project` tag `v4.0.1`

## Approach
- [x] Audit the current worker vendored payee proto and provider surface against the `v4.0.1` daemon contract.
- [x] Update the vendored payee proto/stubs and provider interface to include `OpenSession` and any required close/open message types.
- [x] Refactor the paid middleware flow to resolve the request route before payment processing, open a payee session with authoritative `(capability, offering, price, work_unit)` metadata, then run `ProcessPayment`, `DebitBalance`, reconciliation, and `CloseSession`.
- [x] Add or update tests covering per-request open/process/debit/close behavior and verify the worker checks still pass.

## Decisions log
### 2026-05-03 — Use per-request payee sessions
Reason: The current worker serves OpenAI-style request/response routes, not long-lived conversational transport sessions. For correctness and minimal scope, each paid request will `OpenSession`, spend against that session, and `CloseSession` when request handling completes. Session reuse can be revisited later if prepaid multi-request balances become a product requirement.

### 2026-05-03 — Close only after ProcessPayment seals sender
Reason: The daemon's `CloseSession` RPC requires a sender-bound open session. Sessions opened before a rejected `ProcessPayment` remain pending because the daemon exposes no way to close an unsealed session. The worker therefore closes on every post-`ProcessPayment` exit path and documents the pending-session limitation rather than inventing daemon-side behavior in this repo.

## Open questions
- None at implementation close-out. Residual limitation: rejected payments leave pending sessions until the daemon gains an explicit pending-session cleanup path.

## Artifacts produced
- Code updates under:
  - `internal/proto/`
  - `internal/providers/payeedaemon/`
  - `internal/runtime/http/`
- Verification:
  - `go test ./...`
  - `make lint`
