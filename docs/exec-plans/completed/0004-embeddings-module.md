---
id: 0004
slug: embeddings-module
title: Capability module — openai:/v1/embeddings
status: completed
owner: agent
opened: 2026-04-24
completed: 2026-04-24
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

- [x] Module at `internal/service/modules/embeddings/`.
- [x] `EstimateWorkUnits`: tokenize `input` (string, []string, or []int token IDs). Token-id array contributes `len(ids)` directly; mixed-shape safe fallback of 0 for unknown shapes so the backend (not the worker) owns validation.
- [x] `Serve`: POST to backend via `backendhttp.Client.DoJSON`, buffered JSON response written through; `usage.total_tokens` scraped for reconciliation (falls back to 0 when absent — embeddings backends sometimes omit usage on error).
- [x] Backend interface: shared with chat completions (same `backendhttp.Client` provider); each model has its own configured backend URL via `worker.yaml`.
- [x] Integration tests: 9 cases — ExtractModel happy + missing, Estimate across string/[]string/[]int/null/bad-JSON, Serve happy/no-usage/backend-error, Capability-path accessors. 94.4% coverage.
- [x] cmd wiring: case added to `registerModules` switch in `cmd/openai-worker-node/main.go`.

## Decisions log

### 2026-04-24 — `input` as `any` with runtime dispatch

OpenAI's `input` accepts three shapes: string, []string, []int. Modeling it as `any` and branching at tokenization time is simpler than writing custom UnmarshalJSON for a union. The type ends up as `string` / `[]any` (from json.Unmarshal) which we switch on. Malformed shapes return 0 tokens — the backend owns validation, not the worker.

### 2026-04-24 — Token-id array counts as `len(ids)`

When `input` is a []int (pre-tokenized by the caller), each entry is already one token regardless of numeric value. `len` is exact. This undercounts encoder expansion (some models pad, some add BOS/EOS) but the over-debit policy absorbs it.

## Open questions

All resolved.

## Artifacts produced

Files landed:

- `internal/service/modules/embeddings/{doc,types,module,module_test}.go`
- `cmd/openai-worker-node/main.go` — embeddings case added to `registerModules`.
