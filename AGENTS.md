# AGENTS.md — openai-worker-node

This repository hosts `openai-worker-node`: the HTTP adapter that sits between `openai-livepeer-bridge` and local inference backends (vLLM, diffusers, whisper, TTS, …). It validates payments via a sidecar [`livepeer-payment-daemon`](../livepeer-modules-project/payment-daemon) in `receiver` mode, then forwards OpenAI-compatible requests to the configured backend for each (capability, model) pair.

**Humans steer. Agents execute. Scaffolding is the artifact.**

## Start here

- Design & domains: [DESIGN.md](DESIGN.md)
- How to plan work: [PLANS.md](PLANS.md)
- Product mental model: [PRODUCT_SENSE.md](PRODUCT_SENSE.md)
- Harness philosophy: [docs/references/openai-harness.pdf](docs/references/openai-harness.pdf)
- Cross-repo YAML contract: [livepeer-modules-project/payment-daemon/docs/design-docs/shared-yaml.md](../livepeer-modules-project/payment-daemon/docs/design-docs/shared-yaml.md)

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
- `buf` for consuming generated `livepeer.payments.v1.*` protos from the library
- `golangci-lint` + custom lints in `lint/`

## Commands

- `make build` — build the worker binary
- `make test` — run unit tests (race-enabled)
- `make lint` — run all lints (golangci-lint + custom)
- `make doc-lint` — validate knowledge-base cross-links and freshness

## Invariants (do not break without a design-doc)

1. **Payment is auth.** Every paid HTTP route MUST pass through the payment middleware before reaching a backend. The middleware calls `PayeeDaemon.ProcessPayment` + `DebitBalance`; skipping either is a security bug, not a style issue. Enforced by a custom lint on the capability-module registration surface.
2. **Fail-closed on config.** `worker.yaml` parse errors, daemon/worker capability mismatch, or missing backend URLs cause refuse-to-start. No partial-start fallbacks.
3. **Shared YAML is authoritative.** This repo does not define a YAML schema. It consumes [`livepeer-modules-project/payment-daemon/config/sharedyaml`](../livepeer-modules-project/payment-daemon/config/sharedyaml) as a Go module dep. Do not copy-paste the types.
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
| What's the YAML contract? | `../livepeer-modules-project/payment-daemon/docs/design-docs/shared-yaml.md` |
| Known debt? | `docs/exec-plans/tech-debt-tracker.md` |
