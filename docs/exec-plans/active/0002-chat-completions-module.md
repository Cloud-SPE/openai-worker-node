---
id: 0002
slug: chat-completions-module
title: First capability module — openai:/v1/chat/completions
status: active
owner: unclaimed
opened: 2026-04-24
---

## Goal

Ship the first real capability module end-to-end: parse config, register the route, count tokens, dispatch to a vLLM-compatible backend, stream SSE back to the caller, reconcile the final token count. This is the module pattern every other capability will mirror; get the pattern right here and the rest is replication.

## Non-goals

- No embeddings, images, or audio. Those are 0004–0006.
- No payment middleware. That is 0003, which this plan depends on.
- No request-body validation beyond what the backend itself rejects. A full OpenAI schema checker is a later plan; until then, we trust the bridge to send valid bodies.
- No load balancing to multiple backends. One URL per model per the architecture doc.

## Cross-repo dependencies

- Library `0018-per-capability-pricing` (shared YAML + `ListCapabilities` + per-model pricing).
- This repo: `0003-payment-middleware` (the paid-route adapter this module gets wrapped in).

## Approach

- [ ] Module package at `internal/service/modules/chat_completions/`.
- [ ] Request/response types in `internal/service/modules/chat_completions/types/`.
- [ ] `EstimateWorkUnits`: tokenize `messages[]` via the tokenizer provider, add `max_tokens` (default from config). Charge output price for all tokens per the metering decision.
- [ ] `Serve`: POST to backend, stream SSE through, capture `usage.total_tokens` from the final chunk.
- [ ] Streaming: `reply.hijack`-equivalent pattern, byte-for-byte proxy with abort propagation.
- [ ] Integration test: fake backend that returns a canned SSE stream, real payment middleware wired to fake PayeeDaemon, assert DebitBalance called twice (estimate + reconcile) with the right numbers.

## Decisions log

_Empty._

## Open questions

- **Tokenizer choice.** `tiktoken-go` is the obvious default for OpenAI-compatible tokenization but has per-model variance (gpt-4 vs llama3). Is there a canonical llama/mistral tokenizer we should vendor, or do we pick per-model via config?
- **Streaming abort propagation.** When the bridge client disconnects mid-stream, do we cancel the backend call or let it complete? Leaning cancel — mirrors the bridge's existing semantics.

## Artifacts produced

_Not started._
