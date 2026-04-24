---
id: 0007
slug: docker-packaging
title: Docker image + compose example for joint worker+daemon deployment
status: active
owner: unclaimed
opened: 2026-04-24
---

## Goal

Ship a container image for `openai-worker-node` and a `compose.yaml` that runs it alongside `livepeer-payment-daemon`, with the shared `worker.yaml` bind-mounted into both containers. Operators should be able to `git clone` + `docker compose up` to get a working (fake-broker dev mode) worker on their laptop within minutes.

## Non-goals

- No production-ready Helm chart. Deferred; operators typically run this on a GPU host, not Kubernetes, for v1.
- No GPU runtime wiring in this image. The inference backends (vLLM, diffusers, whisper) are separate images; this worker is stateless and CPU-only.

## Cross-repo dependencies

- Library `0018-per-capability-pricing` (so `sharedyaml` is importable for the build).
- This repo: 0001 (scaffold), 0003 (middleware — minimum viable runnable binary).

## Approach

- [ ] `Dockerfile` — multi-stage Go build, distroless final image, `CMD ["/openai-worker-node", "--config", "/etc/livepeer/worker.yaml"]`.
- [ ] `compose.yaml` at repo root:
  - `worker` service from this image.
  - `payment-daemon` service from the library's image.
  - `worker.yaml` bind-mounted read-only into both at `/etc/livepeer/worker.yaml`.
  - A named volume for the daemon's BoltDB.
  - Example keystore + env for dev-mode (fake broker, fake sender balances).
- [ ] `docs/operations/running-with-docker.md` — step-by-step, ported in style from the library's version.
- [ ] CI: build the image on tag pushes.

## Decisions log

_Empty._

## Open questions

- **Image registry.** Matches library's `tztcloud/` prefix for now; confirm before any public push.
- **Base image.** `gcr.io/distroless/static-debian12` (library's choice) vs `scratch`. Default to distroless for parity.

## Artifacts produced

_Not started._
