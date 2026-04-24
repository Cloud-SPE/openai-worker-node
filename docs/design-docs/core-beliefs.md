---
title: Core beliefs
status: accepted
last-reviewed: 2026-04-24
---

# Core beliefs

Non-negotiable invariants. Every design-doc, exec-plan, and line of code in this repo respects these. Changing any of them requires its own design-doc.

## 1. Scaffolding is the artifact

The value we produce is the repository structure, the lints, the CI, the docs, and the exec-plans. Code is generated to fit the scaffold. If a change makes the scaffold weaker, reject it — even if the code is fine.

## 2. Repository knowledge is the system of record

Anything not in-repo doesn't exist. Slack threads, Google Docs, and tribal knowledge are invisible to the agents that maintain this codebase. If a decision matters, it lives in `docs/design-docs/`. If a plan matters, it lives in `docs/exec-plans/`.

## 3. Payment is authentication

Every paid HTTP route passes through the payment middleware — `ProcessPayment` then `DebitBalance`, both over gRPC — before reaching a backend. Skipping either is a security bug, not a style issue. This is the single most important invariant in this repo; it is enforced by a custom lint on the module-registration surface.

## 4. Fail closed, never partial

Config parse error → refuse to start. Daemon/worker catalog mismatch → refuse to start. Missing backend URL for an advertised model → refuse to start. There is no "partial mode" where some capabilities work and others don't — the worker either serves its full advertised catalog or serves nothing.

## 5. Shared YAML comes from the library, not from here

This repo consumes `livepeer-payment-library/config/sharedyaml` as a Go module dependency. We do not define a YAML schema. Drift between what the daemon parses and what the worker parses is the exact failure mode this dependency exists to prevent.

## 6. The providers boundary is the only cross-cutting boundary

`service/*` may not import `google.golang.org/grpc`, an HTTP client, a tokenizer library, or any external cross-cutting dependency directly. Everything external goes through `internal/providers/`. Enforced mechanically.

## 7. Enforce invariants, not implementations

Lints check structural properties (layer dependencies, payment-middleware coverage, structured logging, no-secrets-in-logs). They do not prescribe libraries, variable names, or stylistic preferences. Agents get freedom within the boundaries.

## 8. Humans steer; agents execute

Humans author design-docs, open exec-plans, and review outcomes. Agents do the implementation. If an agent is struggling, the fix is almost always to make the environment more legible — not to push harder on the task.

## 9. No code without a plan

Non-trivial changes start with an entry in `docs/exec-plans/active/`. Bugs, drive-by fixes, and one-line changes are exempt (see `PLANS.md`). This is how we keep progress visible and reviewable.

## Project-specific invariants

- **Stateless adapter.** This process holds no ledger, no queue, no durable state. The payee daemon is the system of record for balances; the inference backend is the system of record for model state.
- **Unix-socket gRPC to the daemon.** No TCP. The daemon and worker share a host; IPC stays on the filesystem.
- **One backend URL per (capability, model).** No fan-out in v1. When fan-out becomes necessary, it enters via the providers/ boundary, not by forking the module interface.
- **Capability modules are self-contained.** A module never imports another module. Shared code goes in `internal/` above the modules layer, not cross-wired between siblings.
- **Test coverage ≥ 75% for every package.** Non-negotiable. CI fails if any package falls below 75% statement coverage. Packages with inherent test difficulty are exempt only when explicitly listed with a written reason and a tracking issue.
