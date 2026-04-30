---
id: 0010
slug: v3-offerings-rename
title: v3.0.1 worker surface + shared worker.yaml alignment
status: completed
owner: agent
opened: 2026-04-29
depends-on: livepeer-network-suite plan 0003 §G (openai-worker-node row)
---

## Goal

Bring `openai-worker-node` into the pinned `v3.0.1` worker contract:
shared nested `worker.yaml`, `protocol_version` parsing +
fail-closed validation, `/registry/offerings` as the canonical unpaid
surface, deleted legacy unpaid endpoints, and a repo-wide doc/example
sweep away from pre-v3 gateway/two-file config assumptions.

## Non-goals

- No backwards-compat parsing for pre-v3 worker YAML shapes.
- No changes to paid-route business logic or module semantics.
- No changes to daemon-side parsing rules beyond the worker-facing
  startup cross-check already consumed via `ListCapabilities`.

## Cross-repo dependencies

- `livepeer-network-spec-v3.md` pinned `v3.0.1` additions from
  2026-04-30, especially §§15.18–15.28.

## Approach

- [x] Replace the flat worker-only parser with the shared nested
      `worker.yaml` contract:
      `protocol_version`, optional `worker_eth_address`, optional
      `auth_token`, opaque `payment_daemon`, nested `worker`, shared
      `capabilities`.
- [x] Add worker-side validation for:
      explicit `service_registry_publisher` rejection,
      `protocol_version == CurrentProtocolVersion`,
      presence of `payment_daemon`,
      lowercase `worker_eth_address`,
      object-only `extra` / `constraints`.
- [x] Split `CurrentProtocolVersion` from `CurrentAPIVersion`, surface
      both on `/health`, and delete `/capabilities`, `/quote`, and
      `/quotes` from code/tests/docs.
- [x] Rework `/registry/offerings` to emit the canonical fragment from
      config, including optional `worker_eth_address`, optional
      `extra`, optional `constraints`, and YAML-sourced bearer auth.
- [x] Update compose/examples/docs to the shared `worker.yaml` model
      and remove the stale `payment-daemon.yaml` guidance.
- [x] Close or supersede stale in-flight planning artifacts that
      encode the obsolete two-file daemon migration assumptions.

## Decisions log

### 2026-04-30 — Fold the v3.0.1 worker cut into the existing 0010 plan
Reason: the original plan already owned the unpaid worker-surface
migration. The 2026-04-30 spec updates turned that from a simple
offerings rename into a broader worker-contract alignment, so updating
the existing plan preserves continuity without opening a duplicate.

### 2026-04-30 — Keep the provider `GetQuote` method even though the HTTP endpoints are gone
Reason: `v3.0.1` deletes the worker quote routes, but keeping the
provider method preserves compatibility with the daemon surface and
avoids a larger proto/client churn than the worker contract requires.

## Open questions

## Artifacts produced

- Shared nested `worker.yaml` parser + validation updates in
  `internal/config/`
- Deleted legacy unpaid worker endpoints and updated `/health` +
  `/registry/offerings` in `internal/runtime/http/`
- Operator-facing doc/example sweep across `README.md`, `DESIGN.md`,
  `PRODUCT_SENSE.md`, `AGENTS.md`, compose files, and
  `docs/operations/running-with-docker.md`
- Verification: `go build ./...` and `go test ./...`
