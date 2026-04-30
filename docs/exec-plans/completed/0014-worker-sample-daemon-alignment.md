---
id: 0014
slug: worker-sample-daemon-alignment
title: Align worker sample/docs with payment-daemon config ownership
status: completed
owner: agent
opened: 2026-04-30
---

## Goal

Correct the worker sample and Docker runbook so they match the actual
`livepeer-payment-daemon` contract in `livepeer-modules-project`:

- keystore and store path stay CLI / compose owned
- shared `worker.yaml` still carries per-offering `backend_url`

## Non-goals

- No daemon code changes in this repo.
- No compose contract changes unless they are wrong.

## Approach

- [x] Remove `payment_daemon.keystore` and `payment_daemon.store` from
      `worker.example.yaml`.
- [x] Update Docker docs to point operators at compose/CLI flags for
      keystore + store ownership.
- [x] Verify the compose files already reflect the daemon's CLI
      contract and that no stale references remain.

## Decisions log

### 2026-04-30 — Compose already had the right ownership split
Reason: this repo's `compose.yaml` and `compose.prod.yaml` already pass
`--store-path`, `--keystore-path`, and `--keystore-password-file`
directly to `livepeer-payment-daemon`, which matches the sibling
daemon's current CLI contract. The only stale surfaces were the sample
`worker.yaml` and the operator runbook.
