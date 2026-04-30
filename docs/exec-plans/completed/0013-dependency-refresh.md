---
id: 0013
slug: dependency-refresh
title: Refresh stale image pins and indirect Go dependencies
status: completed
owner: agent
opened: 2026-04-30
---

## Goal

Bring the repo's stale dependency surfaces up to date:

- default Docker image tags in compose files
- outdated indirect Go modules in `go.mod` / `go.sum`

## Non-goals

- No feature work.
- No major-version library migrations beyond what `go get` resolves safely.
- No cross-repo dependency changes in sibling repos.

## Approach

- [x] Update compose defaults from `v3.0.0` to `v3.0.1`.
- [x] Fix the stale Docker upgrade doc wording.
- [x] Refresh the flagged indirect Go modules to current releases.
- [x] Run `go build ./...`, `go test ./...`, `make lint`, and `make doc-lint`.

## Decisions log

### 2026-04-30 — Treat image-tag drift as a dependency issue
Reason: the repo code and docs already target `v3.0.1`; leaving compose
defaults at `v3.0.0` is operationally stale in the same way an old
library pin is stale.

### 2026-04-30 — Limit the Go refresh to declared module entries
Reason: `go list -m -u all` still reports newer versions for deep
transitive modules outside `go.mod`, but the repo-owned direct and
indirect declarations are now current. Chasing the entire transitive
graph without a product or security driver would create unnecessary
change surface.
