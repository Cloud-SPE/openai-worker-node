---
id: 0011
slug: v3-daemon-migration
title: Migrate worker to payment-daemon v3 config and catalog contract
status: active
owner: agent
opened: 2026-04-30
depends-on: livepeer-modules-project payment-daemon plan 0020
---

## Goal

Migrate `openai-worker-node` off the pre-v2 shared `worker.yaml` and
`ListCapabilities.protocol_version` assumptions so the repo can run
against `livepeer-payment-daemon:v3.0.0` by default. The worker should
own a worker-only config file, verify daemon compatibility via catalog
equality, and ship compose/docs examples that match the current daemon
flag/env surface.

## Non-goals

- No backward-compat parsing for the old shared `worker.yaml` shape.
- No automatic generation of `payment-daemon.yaml` from `worker.yaml`.
- No changes to paid-route business logic or module semantics.

## Cross-repo dependencies

- `livepeer-modules-project/payment-daemon` plan 0020
  (`config-package-simplification`) — source of truth for the daemon's
  post-v2 config and `ListCapabilities` contract.

## Approach

- [x] Replace the worker config parser with a worker-only YAML shape.
- [x] Drop daemon `protocol_version` verification and update the local
      payee-daemon client/proto stubs to the v3 contract.
- [x] Update startup wiring, health/capabilities responses, and tests to
      use the worker's own v3 protocol constant instead of a YAML field.
- [x] Add `payment-daemon.example.yaml` and update compose defaults to
      mount separate worker and daemon config files.
- [x] Update operator docs and env templates to the v3 deployment model.
- [x] Run targeted tests and proto generation.

### 2026-04-30 — Scope landed
Reason: the worker now owns `worker.yaml`, the daemon contract no
longer expects `protocol_version`, local proto stubs were regenerated,
compose defaults pin `v3.0.0`, and `go test ./...` passed after the
migration.

## Decisions log

### 2026-04-30 — Keep `worker.yaml` as the worker-side filename
Reason: the daemon already moved to `payment-daemon.yaml`; renaming the
worker file as well would add churn without new information. A
two-file model with `worker.yaml` + `payment-daemon.yaml` is explicit
enough and keeps the worker's historical operator surface stable.

### 2026-04-30 — Retain a worker-local `protocol_version`
Reason: the shared daemon/worker protocol version is gone, but the
worker still benefits from a stable version marker on its own HTTP
surface and metrics. The value becomes a worker-owned constant instead
of a cross-process YAML field.
