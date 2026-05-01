# 0015 — Payment Ticket Params Proxy

Status: completed
Owner: Codex
Started: 2026-04-30

## Goal

Add `POST /v1/payment/ticket-params` as a thin authenticated proxy to the local receiver-mode `livepeer-payment-daemon`, matching the v3.0.1 follow-up requirement.

## Scope

- extend the in-repo payee-daemon proto copy with `GetTicketParams`
- regenerate gRPC stubs
- add provider support and metrics labeling
- add worker HTTP route, validation, auth reuse, and error mapping
- add focused tests
- update product-spec docs

## Notes

- This route is not pricing and must not revive `/quote`.
- The worker does no ticket crypto; it only validates/authenticates, proxies, and returns the daemon’s canonical response object.
