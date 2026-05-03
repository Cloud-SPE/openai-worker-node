---
id: 0017
slug: modules-v4-0-1-alignment
title: Align worker repo defaults with livepeer-modules-project v4.0.1
status: active
owner: codex
opened: 2026-05-03
---

## Goal
Update repo-owned operator-facing defaults and examples so `openai-worker-node` matches the current published `livepeer-modules-project` daemon release (`v4.0.1`) without changing the already-stable shared worker contract semantics.

## Non-goals
- No protocol or schema redesign.
- No edits to historical plans under `docs/exec-plans/completed/`.
- No speculative bumps to worker image tags that this repo has not published.

## Cross-repo dependencies
- `livepeer-modules-project` tag `v4.0.1` (already cut)

## Approach
- [x] Audit repo-local version pins, config examples, and operator docs for stale daemon defaults.
- [x] Update compose/examples/docs to pin the payment daemon to `v4.0.1` where appropriate and remove stale pre-shared-config guidance.
- [x] Verify the repo still passes relevant checks and record cross-repo alignment findings.

## Decisions log
### 2026-05-03 — Treat v4.0.1 as a release-pin refresh, not a schema migration
Reason: The upstream `livepeer-modules-project` diff from `v4.0.0` to `v4.0.1` is CI/dependency oriented and does not change the worker/payment-daemon shared `worker.yaml` contract. This repo should therefore update image/default-version references and stale examples, but keep code comments that correctly describe the v3.0.1-era contract shape.

### 2026-05-03 — Do not bump the worker image default beyond this repo's published tags
Reason: `openai-worker-node` currently has local tags through `v3.0.1`. Bumping `WORKER_IMAGE_TAG` defaults to an unpublished `v4.0.1` would create a broken compose default. Only the payment-daemon default should move to the new modules-project release in this pass.

## Open questions
- Whether `openai-livepeer-bridge` should independently move its daemon image defaults from `v3.0.2` to `v4.0.1` in a repo-local follow-up plan.

## Artifacts produced
- Repo-local updates:
  - `compose.yaml`
  - `compose.prod.yaml`
  - `.env.example`
  - `worker.example.yaml`
  - `docs/operations/running-with-docker.md`
- Verification:
  - `make test`
  - `make doc-lint`
  - `docker compose -f compose.yaml config`
  - `docker compose -f compose.prod.yaml --env-file .env.example config`
