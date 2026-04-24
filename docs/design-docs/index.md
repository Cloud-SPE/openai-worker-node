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

## Planned

- `capability-modules.md` — deep-dive on the module registration surface and payment-middleware contract. Planned with the first real module plan.
- `metering.md` — per-capability work-unit computation and reconciliation semantics.
- `streaming.md` — SSE proxying, backpressure, abort propagation.

## Conventions

- Every design-doc has frontmatter: `title`, `status`, `last-reviewed`, optional `supersedes` and `superseded-by`.
- Docs may link to other docs; they may not link into `exec-plans/` (plans are transient; docs are durable).
- Cross-repo links are fine — especially to [`../../../livepeer-payment-library/docs/design-docs/shared-yaml.md`](../../../livepeer-payment-library/docs/design-docs/shared-yaml.md), which is the YAML contract this repo consumes.
- When implementation diverges from a doc, either the code changes to match or the doc is updated — never both out of sync.
