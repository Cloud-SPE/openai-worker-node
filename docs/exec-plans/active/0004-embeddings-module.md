---
id: 0004
slug: embeddings-module
title: Capability module — openai:/v1/embeddings
status: active
owner: unclaimed
opened: 2026-04-24
---

## Goal

Add the embeddings capability module. Work-unit is input tokens only; request/response is one-shot (no streaming); cost is deterministic from the request body.

## Non-goals

- No streaming plumbing (not applicable).
- No request-body schema validation beyond what the backend rejects.

## Cross-repo dependencies

- Library `0018-per-capability-pricing`.
- This repo: `0002-chat-completions-module` (establishes the Module interface), `0003-payment-middleware` (the pipeline).

## Approach

- [ ] Module at `internal/service/modules/embeddings/`.
- [ ] `EstimateWorkUnits`: tokenize `input` (string or string[]). Deterministic; `actualUnits == estimatedUnits` modulo tokenizer drift.
- [ ] `Serve`: POST to backend, buffered response, return body.
- [ ] Backend interface: assume OpenAI-compatible at `/v1/embeddings`. Same provider as chat (TEI-compatible servers also work).
- [ ] Integration test.

## Decisions log

_Empty._

## Open questions

_None._

## Artifacts produced

_Not started._
