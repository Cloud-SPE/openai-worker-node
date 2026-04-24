---
id: 0001
slug: repo-scaffold
title: Stand up initial repository scaffolding
status: active
owner: human
opened: 2026-04-24
---

## Goal

Lay down the full scaffolding for `openai-worker-node` before any implementation code is written: directory structure, layer placeholders, lints, CI, initial design-docs, and supporting tooling. The repo should be ready for `0002-chat-completions-module` to start writing code inside a well-defined frame.

This is a new repo; the harness layout mirrors `livepeer-payment-library` and `openai-livepeer-bridge` so an agent familiar with either sibling can navigate this one on arrival.

## Non-goals

- No capability-module code. That begins with `0002-chat-completions-module`.
- No payment-middleware implementation. That is `0003-payment-middleware`.
- No lints beyond stubs. The custom lints (`layer-check`, `payment-middleware-check`) are placeholder READMEs; full AST implementations are deferred and tracked as tech debt.
- No Go module implementation of `sharedyaml` parse. Consumed from `livepeer-payment-library` once that package exists under `0018-per-capability-pricing`.

## Cross-repo dependencies

- [`livepeer-payment-library` `0018-per-capability-pricing`](../../../../livepeer-payment-library/docs/exec-plans/active/0018-per-capability-pricing.md)
  - Must land the `config/sharedyaml` public Go package.
  - Must land the `ListCapabilities` RPC + per-model pricing in `GetQuoteResponse`.
  - This plan's "wire go.mod dep" step blocks on both.

## Approach

- [x] Root-level harness docs: `AGENTS.md`, `DESIGN.md`, `PLANS.md`, `PRODUCT_SENSE.md`, `README.md`
- [x] `docs/design-docs/` seeded: `index.md`, `core-beliefs.md`, `architecture.md`
- [x] `docs/exec-plans/` seeded: `active/0001-repo-scaffold.md` (this file), `tech-debt-tracker.md`
- [x] `docs/product-specs/index.md` seeded
- [x] `docs/generated/` placeholder with `.gitkeep`
- [x] `docs/operations/` placeholder with `.gitkeep`
- [x] Source layer placeholders (`internal/{types,config,repo,service,service/modules,runtime,providers}/`) with `.gitkeep`
- [x] `cmd/openai-worker-node/main.go` placeholder with package declaration + TODO pointing to 0003
- [x] `lint/README.md` describing planned lints (`layer-check`, `payment-middleware-check`, `no-raw-log`, `no-secrets-in-logs`, `doc-gardener`)
- [x] `go.mod` for Go 1.25; module path `github.com/Cloud-SPE/openai-worker-node`
- [x] `Makefile` with targets: `build`, `test`, `lint`, `lint-custom`, `doc-lint`, `docker-build`, `clean`
- [x] `.gitignore` (binaries, coverage, `.env`, `worker.yaml` but not `worker.example.yaml`)
- [x] `.env.example` with `LIVEPEER_WORKER_CONFIG=/etc/livepeer/worker.yaml`
- [x] `worker.example.yaml` at repo root — full annotated sample, all six v1 capabilities populated
- [x] GitHub Actions workflows: `lint.yml`, `test.yml`, `doc-lint.yml` — stub-level
- [x] Follow-up exec-plan stubs: `0002-chat-completions-module`, `0003-payment-middleware`, `0004-embeddings-module`, `0005-images-module`, `0006-audio-modules`, `0007-docker-packaging`

## Decisions log

### 2026-04-24 — Mirror the library's Go module path style

Module path `github.com/Cloud-SPE/openai-worker-node` matches `github.com/Cloud-SPE/livepeer-payment-library`. Keeps the go.mod dep on the library straightforward and groups the payment-ecosystem repos under one org.

### 2026-04-24 — `sharedyaml` is a go.mod dep, not a submodule or copy

Importing the library as a Go module is the cheapest way to keep YAML parsing in lockstep. Alternatives considered: git submodule (heavy), code copy (drift risk — exactly what this contract exists to prevent), code-gen from a `.cue` or `.json-schema` source (over-engineered for two consumers).

### 2026-04-24 — Module path `internal/service/modules/<name>/`, not top-level `internal/modules/`

Modules are service-layer concepts; they are business logic, not a cross-cutting concern. Placing them under `service/` preserves the layer rule (modules depend on `providers/`, `config/`, `types/`; they are depended on by `runtime/`). A separate `internal/modules/` path outside the layer stack would weaken the architecture for no gain.

### 2026-04-24 — No frontend

Unlike the bridge, this service has no UI. There is no `FRONTEND.md`, no `src/ui/` stub. Admin views (if any) live in the bridge or in operator dashboards fed from metrics/logs.

### 2026-04-24 — No `FRONTEND.md`

Same reason as above. Documented here so future doc-gardening lints don't flag the absence.

## Open questions

- **License.** TBD. Library is TBD as well. Will resolve as a follow-up before any tagged release.
- **CI provider.** GitHub Actions is the default; confirming alignment with the sibling repos once those CI configs are inspected.
- **Coverage threshold enforcement tooling.** Library uses a `lint/coverage-gate/` directory; will reuse the same pattern here but postpone full integration until there's code to cover.

## Artifacts produced

Scaffold landed in a single agent session:

- Root docs: `AGENTS.md`, `DESIGN.md`, `PLANS.md`, `PRODUCT_SENSE.md`, `README.md`
- Design-docs: `docs/design-docs/{index,core-beliefs,architecture}.md`
- Product specs: `docs/product-specs/index.md`
- Exec plan stubs: `docs/exec-plans/active/000{2,3,4,5,6,7}-*.md`
- Tech-debt: `docs/exec-plans/tech-debt-tracker.md`
- Tooling: `go.mod`, `Makefile`, `.gitignore`, `.env.example`, `worker.example.yaml`, `lint/README.md`
- CI: `.github/workflows/{lint,test,doc-lint}.yml`
- Source layer placeholders: `internal/{types,config,repo,service,service/modules,runtime,providers}/.gitkeep`, `cmd/openai-worker-node/main.go`

Outstanding before closing this plan:

- `livepeer-payment-library` is wired via a `replace` directive in `go.mod` (local sibling checkout). The `require` line appears automatically via `go mod tidy` once a source file imports from the library; until then the dep stays latent. Library plan 0018 landed as commit `7f81543`, unblocking this.
