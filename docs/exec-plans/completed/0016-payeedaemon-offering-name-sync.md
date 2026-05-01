# 0016 — Payee Daemon Offering Name Sync

Status: completed
Owner: Codex
Started: 2026-05-01

## Goal

Sync the worker repo's local payee-daemon proto copy and worker-side domain naming from `model/models` to `offering/offerings` so it matches `livepeer-modules-project/payment-daemon`.

## Scope

- rename proto messages and repeated fields in the in-repo copy
- regenerate gRPC stubs
- rename worker-side provider/domain types and verifier references
- update direct tests and docs that mention the old names

## Non-goals

- changing OpenAI workload request fields that still legitimately use `model`
- changing backend module behavior
