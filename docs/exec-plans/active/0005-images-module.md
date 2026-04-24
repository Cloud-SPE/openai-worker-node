---
id: 0005
slug: images-module
title: Capability modules — openai:/v1/images/generations + /images/edits
status: active
owner: unclaimed
opened: 2026-04-24
---

## Goal

Add both image capabilities. They share a common internal implementation (same backend shape, same metering formula) and two thin public module wrappers — one per route.

## Non-goals

- No image-upload multipart handling optimization. Edits take multipart; we forward the body verbatim without trying to parse or re-encode.

## Cross-repo dependencies

- Library `0018-per-capability-pricing`.
- This repo: `0003-payment-middleware`, `0002-chat-completions-module` (pattern reference).

## Approach

- [ ] Shared internal package `internal/service/modules/images/common/` with the estimator + dispatcher.
- [ ] Two module packages, `internal/service/modules/images/generations/` and `internal/service/modules/images/edits/`, each a thin wrapper registering its route through the common implementation.
- [ ] `EstimateWorkUnits`: `n × steps × megapixels`, rounded to int. Cost is fully deterministic from the request.
- [ ] `Serve`: POST to backend, buffered response (no streaming).
- [ ] Integration tests for both routes.

## Decisions log

_Empty._

## Open questions

- **Multipart body handling for edits.** Does the backend expect the same multipart shape the bridge forwards, or do we need to re-encode? Default assumption: pass-through is sufficient (diffusers img2img servers accept OpenAI-format multipart).

## Artifacts produced

_Not started._
