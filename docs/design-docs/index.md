# Design docs index

A catalog of every design-doc in this repo, with verification status.

## Verification status

Each doc carries a `status:` field in its frontmatter. Values:

| Status | Meaning |
|---|---|
| `proposed` | Written, not yet reviewed or implemented |
| `accepted` | Reviewed, intended direction, not yet fully implemented |
| `verified` | Implemented and matches code; covered by tests |
| `deprecated` | Superseded or abandoned; kept for history |

A doc-gardening lint in CI flags docs with stale status, broken cross-links, or no recent touch after linked code last changed.

## Core beliefs

Non-negotiables that shape every decision in this repo.

- [core-beliefs.md](core-beliefs.md) — `accepted`

## Architectural decisions

- [architecture.md](architecture.md) — `accepted` — layer stack, domains, providers, capability-module pattern
- [capability-modules.md](capability-modules.md) — `accepted` — module registration surface, payment-middleware contract, what a module owns vs. what the middleware owns
- [metering.md](metering.md) — `accepted` — work-unit dimensions, three timing points (estimate / debit / reconcile), per-capability strategies
- [streaming.md](streaming.md) — `accepted` — SSE / raw-byte framing, backpressure, abort propagation
- [metrics.md](metrics.md) — `accepted` — Prometheus metrics catalog (Phase 1 wires Recorder provider + `/metrics` endpoint; mirrors service-registry's verified pattern)

## Cross-repo

- [`../../../livepeer-modules-conventions/metrics-conventions.md`](../../../livepeer-modules-conventions/metrics-conventions.md) — authoritative naming, label, bucket, cardinality, and provider-boundary rules shared across all repos in the fleet
- [`../../../livepeer-modules-project/service-registry-daemon/docs/design-docs/observability.md`](../../../livepeer-modules-project/service-registry-daemon/docs/design-docs/observability.md) — reference implementation of the Recorder pattern this repo's metrics.md mirrors
- [`../../../livepeer-modules-project/payment-daemon/docs/design-docs/shared-yaml.md`](../../../livepeer-modules-project/payment-daemon/docs/design-docs/shared-yaml.md) — the `worker.yaml` cross-repo contract this repo consumes

## Conventions

- Every design-doc has frontmatter: `title`, `status`, `last-reviewed`, optional `supersedes` and `superseded-by`.
- Docs may link to other docs; they may not link into `exec-plans/` (plans are transient; docs are durable).
- When implementation diverges from a doc, either the code changes to match or the doc is updated — never both out of sync.
