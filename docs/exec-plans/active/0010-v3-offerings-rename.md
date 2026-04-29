---
id: 0010
slug: v3-offerings-rename
title: v3.0.0 offerings rename + /registry/offerings endpoint
status: active
owner: agent
opened: 2026-04-29
depends-on: livepeer-network-suite plan 0003 §G (openai-worker-node row)
---

## Goal

Bring the openai-worker-node into the v3.0.0 contract: rename
`models[]` → `offerings[]` everywhere it appears (worker.yaml parser,
`/capabilities` HTTP response, internal config types), and add a new
`/registry/offerings` endpoint emitting the modules-canonical
capability fragment so the orch-coordinator can scrape it into a draft
roster row.

## Non-goals

- No backwards-compat parsing of `models:`-shaped worker.yaml — old
  syntax errors at parse time.
- No customer-facing pricing changes — wholesale-rate plumbing lives
  in the bridge, not here.
- No payment-daemon protocol changes.

## Cross-repo dependencies

- livepeer-modules-project plan 0004 (v3.0.0 schema bump + proto
  rename) — must land before this so the modules-canonical fragment
  shape is stable.
- livepeer-orch-coordinator plan 0002 (consumes
  `/registry/offerings`).

## Approach

- [ ] Rename `capabilities[].models[]` → `capabilities[].offerings[]`
      in `worker.example.yaml`.
- [ ] Update `internal/config/parse.go` and `internal/config/verify.go`
      (and their tests) to parse `offerings:` and reject `models:` as
      an unknown field.
- [ ] Rename internal types under `internal/types/` and
      `internal/config/`: `Model` → `Offering`,
      `Models` field → `Offerings`.
- [ ] Update `/capabilities` HTTP response shape in
      `internal/runtime/http/handlers.go` (and tests under
      `internal/runtime/http/`) — this worker's `/capabilities` shape
      closely tracks modules canonical, so the rename cascades through.
- [ ] Implement `/registry/offerings` handler in
      `internal/runtime/http/` (mirror of `/capabilities` re-shaped
      into the modules-canonical fragment — `name`, `work_unit`,
      `offerings[].id`, `offerings[].price_per_work_unit_wei`).
      `backend_url` stays omitted, same as today's `/capabilities`.
- [ ] Optional bearer auth via a new `OFFERINGS_AUTH_TOKEN` env. If
      set, the endpoint requires `Authorization: Bearer <token>`;
      otherwise plain HTTP. Default off — data ends up in the public
      manifest anyway, but the env hook lets operators add a barrier
      when they want one.
- [ ] Update `README.md`: "registry-invisible by design; bridge owns
      customer-facing routing; archetype A on the operator side. The
      orch-coordinator scrapes `/registry/offerings` to pre-fill the
      operator's roster."
- [ ] Strike-through any registry-related entries in the tech-debt
      tracker.
- [ ] Tag `v3.0.0` (currently pinned at `v1.1.3`).

## Decisions log

## Open questions

- **Modules-project version tag** — assume `v3.0.0`; confirm with
  modules-project plan 0004 before regenerating any
  modules-canonical-shape JSON schema we vendor.
- **Manifest `schema_version` integer** — CONFIRMED `3` (operator answered 2026-04-29); surfaced in the
  `/registry/offerings` body if we choose to include it (master plan
  body shape doesn't, so default no).
- **Daemon image pinning** — CONFIRMED hardcoded `v3.0.0` (every component lands at v3.0.0 in this wave; no tech-debt entry needed).
- Should the `/registry/offerings` endpoint share the HTTP mux with
  `/capabilities` and the workload routes, or sit behind the metrics
  listener? Default: same mux as `/capabilities`.

## Artifacts produced
