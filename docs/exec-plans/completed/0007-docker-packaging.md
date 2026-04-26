---
id: 0007
slug: docker-packaging
title: Docker image + compose example for joint worker+daemon deployment
status: completed
owner: agent
opened: 2026-04-24
completed: 2026-04-24
---

> **Path note (2026-04-26):** Sibling-path references previously naming the standalone `livepeer-payment-library` repo were retargeted to its `livepeer-modules-project/payment-daemon` successor after the modules-project consolidation. Substantive plan content unchanged.

## Goal

Ship a container image for `openai-worker-node` and a `compose.yaml` that runs it alongside `livepeer-payment-daemon`, with the shared `worker.yaml` bind-mounted into both containers. Operators should be able to `git clone` + `docker compose up` to get a working (fake-broker dev mode) worker on their laptop within minutes.

## Non-goals

- No production-ready Helm chart. Deferred; operators typically run this on a GPU host, not Kubernetes, for v1.
- No GPU runtime wiring in this image. The inference backends (vLLM, diffusers, whisper) are separate images; this worker is stateless and CPU-only.

## Cross-repo dependencies

- Library `0018-per-capability-pricing` (landed).
- This repo: 0001 (scaffold), 0003 (middleware), 0002 (first capability). All landed.

## Approach

- [x] `Dockerfile` — multi-stage Go build → `gcr.io/distroless/static:nonroot`. Uses `-tags 'netgo osusergo'` for a pure-Go static binary. Rewrites the local `replace` directive to an in-container path so builds work while the library is still a sibling checkout.
- [x] `compose.yaml` at repo root:
  - `payment-daemon` service built from `../livepeer-modules-project/payment-daemon`. Flags: `--mode=receiver`, unix-socket under a shared named volume, BoltDB state, `--config=/etc/livepeer/worker.yaml`.
  - `openai-worker` service built from `.`, with `additional_contexts: library: ../livepeer-modules-project/payment-daemon` for the `replace` directive's sibling access.
  - `worker.yaml` bind-mounted read-only into BOTH services at `/etc/livepeer/worker.yaml`.
  - Unix-socket volume shared (read-only from the worker side).
- [x] `docs/operations/running-with-docker.md` — prerequisites, first-run walk-through (fake broker dev mode), production-mode notes, upgrade path once the library ships a tagged release.
- [ ] CI: build the image on tag pushes. *(Deferred — no release tags yet in this repo; add when a first tag is cut.)*

## Decisions log

### 2026-04-24 — Distroless:nonroot over scratch

Distroless adds the CA bundle + /etc/passwd (for the nonroot UID) at ~2 MB. Scratch would be ~20 MB smaller but has no CAs — any TLS call from the worker (e.g., to a hosted inference backend) would fail. Not worth the savings.

### 2026-04-24 — `additional_contexts` for the library, not submodules

Submodules would require operators to `git submodule update --init` before every build; that trips up non-git-savvy operators and makes CI noisier. Additional build contexts are a per-project docker feature that stays invisible in day-to-day operation and vanishes as soon as the library tags a release and we drop the `replace` directive.

### 2026-04-24 — sed-rewrite the replace path inside the Dockerfile

Keeping the local `../livepeer-modules-project/payment-daemon` in go.mod means developers can run `go build ./...` on the host without any extra flags. The Dockerfile's sed rewrite swaps it to `/sibling/livepeer-modules-project/payment-daemon` only inside the build container. One line, no external tooling, self-documenting.

## Open questions

- **Image registry prefix.** Using `tztcloud/` to match the library's convention. Confirm before any public push.
- **GPU backends in the compose stack.** Out of scope for v1 but worth documenting once an operator needs a stand-alone dev vLLM container.

## Artifacts produced

Files landed:

- `Dockerfile` — multi-stage build, static binary, distroless:nonroot.
- `compose.yaml` — two services sharing `worker.yaml` and the payment-daemon unix socket.
- `docs/operations/running-with-docker.md` — end-to-end dev walk-through.
