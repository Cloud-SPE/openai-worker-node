---
id: 0012
slug: docker-publish-workflow
title: Add GitHub Actions Docker publish workflow
status: active
owner: agent
opened: 2026-04-30
---

## Goal

Add a GitHub Actions workflow that builds and pushes
`tztcloud/livepeer-openai-worker-node` on release tag pushes so Docker
publishing works in CI the same way it does in the sibling repos.

## Non-goals

- No multi-arch build in this change.
- No PR-time Docker publish; tags only.
- No change to the existing local `make docker-build` workflow.

## Cross-repo dependencies

- `livepeer-secure-orch-console` `.github/workflows/deploy.yml`
- `livepeer-modules-project` `.github/workflows/docker.yml`

## Approach

- [ ] Add `.github/workflows/docker.yml` triggered by `push.tags`.
- [ ] Re-run the repo's Go quality gates inside the publish job before
      pushing.
- [ ] Push `<version>` on every `v*` tag and `latest` only on stable
      `v<major>.<minor>.<patch>` tags.
- [ ] Document the required Docker Hub secrets in the operator docs.

## Decisions log

### 2026-04-30 — Use Docker Hub secrets directly
Reason: the published image name is already Docker Hub specific
(`tztcloud/livepeer-openai-worker-node`), so `DOCKERHUB_USERNAME` and
`DOCKERHUB_TOKEN` are the least surprising operator surface for this
repo.
