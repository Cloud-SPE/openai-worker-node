---
id: 0005
slug: images-module
title: Capability modules — openai:/v1/images/generations + /images/edits
status: completed
owner: agent
opened: 2026-04-24
completed: 2026-04-24
---

## Goal

Add the image capabilities to the worker. This plan landed
`/v1/images/generations` (JSON in, JSON out) end-to-end; `/v1/images/edits`
is deferred under `multipart-capability-handling` tech-debt because it
shares a multipart-body-parsing need with `/v1/audio/transcriptions`.

## Non-goals

- Multipart handling for `/v1/images/edits`. Moved to tech-debt as
  `multipart-capability-handling` alongside `/v1/audio/transcriptions`
  — they share the body-parser work and should land together.

## Cross-repo dependencies

- Library `0018-per-capability-pricing` — landed at `7f81543`.
- This repo: `0003-payment-middleware` (landed), `0002-chat-completions-module` (landed).

## Approach

- [x] Module package at `internal/service/modules/images_generations/`.
- [x] `EstimateWorkUnits`: `ceil((n × steps × W × H) / 1_000_000)` — rounded up so fractional megapixels never under-charge. Defaults applied per OpenAI spec (n=1, size=1024x1024) and a sensible steps default (30).
- [x] `Serve`: POST to backend via `DoJSON`, buffered response passed through. Returns 0 actual units — image backends don't emit usage blocks and metering is deterministic; the middleware skips reconciliation.
- [x] Integration tests (10): ExtractModel happy/missing, Estimate across defaults/explicit/auto-size/malformed-size/negative-n, Serve happy/backend-error, capability-path accessors.
- [x] cmd wiring: case added to `registerModules` switch.

Deferred (own plan / tech-debt):

- [ ] `/v1/images/edits` — multipart body. Tracked as `multipart-capability-handling`.
- [ ] Shared `images/common/` package — currently not needed since only one image module exists. Revisit when edits lands.

## Decisions log

### 2026-04-24 — Megapixels denominator is 1_000_000 (decimal), not 2^20

User-facing math. Operators think in "1 megapixel = 1 million pixels"; the binary-megapixel (2^20 = 1,048,576) would silently diverge from the advertised work-unit meaning. Marginal cost difference; clarity wins.

### 2026-04-24 — Steps is non-OpenAI but accepted

OpenAI's own API doesn't expose a `steps` field (their scheduler picks). SDXL-style backends DO — and operators price per-step. We read it optimistically; request without it falls back to 30.

### 2026-04-24 — `size: "auto"` maps to `defaultSize`

OpenAI added `auto` as a legal size value in 2025. Rather than refuse it, we treat `auto` as the default 1024×1024 for metering purposes. If the backend interprets auto differently we'd over-charge; acceptable.

### 2026-04-24 — Image edits split off to its own plan

The multipart body handling for `/v1/images/edits` and `/v1/audio/transcriptions` is the same problem and deserves one shared answer rather than two ad-hoc implementations. Tracked as tech-debt; expect a dedicated plan once we have an operator asking for it.

## Open questions

All resolved.

## Artifacts produced

Files landed:

- `internal/service/modules/images_generations/{doc,types,module,module_test}.go`
- `cmd/openai-worker-node/main.go` — images_generations case added to `registerModules`.
