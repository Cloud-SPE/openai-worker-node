---
id: 0011
slug: v3-daemon-migration
title: Migrate worker to payment-daemon v3 config and catalog contract
status: abandoned
owner: agent
opened: 2026-04-30
depends-on: livepeer-modules-project payment-daemon plan 0020
---

## Goal

Historical record of the now-rejected two-file migration. The plan
proposed a worker-owned `worker.yaml` plus separate
`payment-daemon.yaml`; `v3.0.1` pinned the opposite direction and
restored the shared-file model.

## Non-goals

- No backward-compat parsing for the old shared `worker.yaml` shape.
- No automatic generation of `payment-daemon.yaml` from `worker.yaml`.
- No changes to paid-route business logic or module semantics.

## Cross-repo dependencies

- `livepeer-modules-project/payment-daemon` plan 0020
  (`config-package-simplification`) — source of truth for the daemon's
  post-v2 config and `ListCapabilities` contract.

## Approach

- [x] Capture the abandoned assumptions in-repo so later readers can
      see why the code no longer follows this path.
- [x] Hand off the real implementation work to
      `0010-v3-offerings-rename.md`, which now owns the v3.0.1 worker
      alignment.

### 2026-04-30 — Scope landed
Reason: this reflected the short-lived local implementation direction
before the 2026-04-30 `v3.0.1` spec lock-ins restored the shared
`worker.yaml` model.

## Decisions log

### 2026-04-30 — Keep `worker.yaml` as the worker-side filename
Reason: superseded. The final `v3.0.1` spec says the receiver daemon
and worker consume the same `worker.yaml`, so the separate
`payment-daemon.yaml` path is no longer valid here.

### 2026-04-30 — Retain a worker-local `protocol_version`
Reason: superseded. `v3.0.1` restored `protocol_version` as a shared
`worker.yaml` field and split it from the worker HTTP `api_version`.

### 2026-04-30 — Abandon the two-file migration
Reason: the updated network + worker specs explicitly require shared
`worker.yaml`, optional YAML-sourced scrape auth, and parser-level
`protocol_version` validation. Keeping this plan active would point the
repo in the wrong direction.

## Artifacts produced

- Superseded by `0010-v3-offerings-rename.md` after the 2026-04-30
  v3.0.1 spec lock-ins
