---
title: POST /v1/embeddings
status: live
last-reviewed: 2026-04-25
module: internal/service/modules/embeddings
---

# POST /v1/embeddings

OpenAI-compatible embeddings endpoint. Pure request/response — no streaming variant exists for embeddings.

## Request

| Field   | Type                                | Used for                                |
| ------- | ----------------------------------- | --------------------------------------- |
| `model` | string                              | Capability/model lookup → backend URL   |
| `input` | string \| string[] \| int[] \| int[][] | Upfront token estimate                |

Other OpenAI fields (`encoding_format`, `dimensions`, `user`) ride through verbatim.

## Metering

- **Capability**: `openai:/v1/embeddings`
- **Work unit**: `token`
- **Upfront estimate**:
  - `string` → `tokenize(input)`
  - `string[]` → `Σ tokenize(s)` for each `s`
  - `int[]` (token-id array) → `len(input)` (each element is one token)
  - `int[][]` (batched token-ids) → sum of lengths
- **Actual**: `usage.total_tokens` from the backend response. Falls back to the upfront estimate when the backend omits the `usage` block.
- **Reconciliation**: middleware second-debits `actual − estimate` when `actual > estimate`. No refund.

## Response

`Content-Type: application/json`. Body is buffered and forwarded with the backend's status code.

```json
{
  "object": "list",
  "data": [{ "object": "embedding", "embedding": [...], "index": 0 }],
  "model": "text-embedding-3-small",
  "usage": { "prompt_tokens": 8, "total_tokens": 8 }
}
```

## Errors

| Condition                                                | Status | Body                                                |
| -------------------------------------------------------- | ------ | --------------------------------------------------- |
| Body not JSON, or `model` missing                        | 400    | `{ "error": "invalid_request", "detail": "..." }`  |
| `(capability, model)` not configured                      | 404    | `{ "error": "capability_not_found" }`               |
| Backend connection / 5xx                                  | 502    | `{ "error": "backend_unavailable" }`                |
| Worker at `max_concurrent_requests`                       | 503    | `{ "error": "capacity_exhausted" }`                 |

## Implementation notes

- File: `internal/service/modules/embeddings/module.go`.
- `input` shapes containing values the worker can't classify (e.g. nested objects) contribute one token each. The intent is to over-estimate when in doubt; backends will reject malformed inputs anyway.
