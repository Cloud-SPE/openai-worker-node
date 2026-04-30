# AGENTS.md — openai-worker-node

This repository hosts `openai-worker-node`: the HTTP adapter that sits
between `livepeer-openai-gateway` and local inference backends (vLLM,
diffusers, whisper, TTS, …). It validates payments via a sidecar
`livepeer-payment-daemon` in `receiver` mode (consumed as a published
container image, not a source dependency), then forwards
OpenAI-compatible requests to the configured backend for each
`(capability, offering)` pair.

**Humans steer. Agents execute. Scaffolding is the artifact.**

## Start here

- Design & domains: [DESIGN.md](DESIGN.md)
- How to plan work: [PLANS.md](PLANS.md)
- Product mental model: [PRODUCT_SENSE.md](PRODUCT_SENSE.md)
- Harness philosophy: [docs/references/openai-harness.pdf](docs/references/openai-harness.pdf)
- worker.yaml schema (shared worker/receiver contract): [`internal/config/parse.go`](internal/config/parse.go) — worker validates worker-facing fields and captures `payment_daemon` opaquely from the shared file

## Knowledge base layout

- `docs/design-docs/` — catalogued design decisions (`index.md` is the entry)
- `docs/exec-plans/active/` — in-flight work with progress logs
- `docs/exec-plans/completed/` — archived plans; do not modify
- `docs/exec-plans/tech-debt-tracker.md` — known debt, append-only
- `docs/product-specs/` — HTTP contract the bridge relies on
- `docs/generated/` — auto-generated; never hand-edit
- `docs/references/` — external material (harness PDF, OpenAI API refs)

## The layer rule (non-negotiable)

Source under `internal/` follows a strict dependency stack:

```
types → config → repo → service → runtime
```

Cross-cutting concerns (payee-daemon gRPC client, logger, metrics, backend HTTP clients, tokenizer) enter through a single layer: `internal/providers/`. Nothing in `service/` may import `grpc`, a logging library, an HTTP client, etc. directly — only through a `providers/` interface.

Capability modules live at `internal/modules/<module-name>/` and follow the same layer rule internally. A module has its own `types/`, `config/`, `service/`, and exposes a single `Register(runtime.Mux)` entry point.

Lints enforce this in CI. See [docs/design-docs/architecture.md](docs/design-docs/architecture.md).

## Toolchain

- Go 1.25+
- `buf` for regenerating the in-repo `livepeer.payments.v1` proto stubs (`make proto`); sources live in `internal/proto/livepeer/payments/v1/`
- `golangci-lint` v2.11+ (v1.x is unsupported — export-data format mismatch with Go 1.25 stdlib) + custom lints in `lint/`
- `govulncheck` (CI-only, informational)

## Commands

- `make build` — build the worker binary
- `make test` — run unit tests (race-enabled)
- `make lint` — run all lints (golangci-lint + custom)
- `make doc-lint` — validate knowledge-base cross-links and freshness

## Invariants (do not break without a design-doc)

1. **Payment is auth.** Every paid HTTP route MUST pass through the payment middleware before reaching a backend. The middleware calls `PayeeDaemon.ProcessPayment` + `DebitBalance`; skipping either is a security bug, not a style issue. Enforced by a custom lint on the capability-module registration surface.
2. **Fail-closed on config.** `worker.yaml` parse errors, daemon/worker capability mismatch, or missing backend URLs cause refuse-to-start. No partial-start fallbacks.
3. **Shared config, split validation.** The worker and
   `livepeer-payment-daemon` both parse the same `worker.yaml`. The
   worker validates its own fields and captures `payment_daemon`
   opaquely; drift in the shared capability catalog is caught at
   startup via `VerifyDaemonCatalog`, not by the compiler.
4. **Providers boundary.** No cross-cutting dependency is imported outside `internal/providers/`.
5. **No code without a plan.** Non-trivial work starts with an entry in `docs/exec-plans/active/`.
6. **Test coverage ≥ 75% per package.** CI fails below this threshold. See `core-beliefs.md`.

## Where to look for X

| Question | Go to |
|---|---|
| What does the worker-node do? | [DESIGN.md](DESIGN.md) |
| Why is X done this way? | `docs/design-docs/` |
| What's in flight? | `docs/exec-plans/active/` |
| What HTTP routes does it serve? | `docs/product-specs/index.md` |
| How do capability modules work? | `docs/design-docs/capability-modules.md` (planned) |
| What's the YAML contract? | [`internal/config/parse.go`](internal/config/parse.go) — worker-side schema and validation |
| Known debt? | `docs/exec-plans/tech-debt-tracker.md` |
