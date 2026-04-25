---
title: POST /v1/images/generations & POST /v1/images/edits
status: live
last-reviewed: 2026-04-25
modules: internal/service/modules/images_generations, internal/service/modules/images_edits
---

# POST /v1/images/generations & POST /v1/images/edits

OpenAI-compatible image-synthesis endpoints. Two capability modules, one shared metering formula. Generations takes a JSON body; edits takes multipart (image + mask + form fields).

## Capabilities

| Capability                       | HTTP path                  | Body          |
| -------------------------------- | -------------------------- | ------------- |
| `openai:/v1/images/generations`  | `/v1/images/generations`   | JSON          |
| `openai:/v1/images/edits`        | `/v1/images/edits`         | multipart     |

## Metering — shared formula

- **Work unit**: `image_step_megapixel`
- **Formula**: `ceil((n × steps × W × H) / 1_000_000)`
- **Defaults** (when the field is absent or invalid): `n=1`, `steps=30`, `size="1024x1024"`
- **Reservation**: equal to the formula output. Reconciliation is a no-op — image backends don't emit `usage` blocks.

`size` accepts `"WxH"` (e.g. `"1024x1024"`); `"auto"` and the empty string fall back to the default. Malformed sizes (`"foo"`, `"512"`, negative dimensions) reject the request with `400 invalid_request` before any debit is attempted.

The formula deliberately ceilings the megapixel calculation so any fractional remainder over-charges by ≤ 1 unit per request. Matches the worker's over-debit-accepted policy.

## Request shapes

### `/v1/images/generations` (JSON)

| Field     | Type                  | Notes                              |
| --------- | --------------------- | ---------------------------------- |
| `model`   | string                | Required                           |
| `prompt`  | string                | Forwarded verbatim                 |
| `n`       | int (optional)        | Default 1; gates `n × steps × MP`  |
| `steps`   | int (optional)        | Default 30                         |
| `size`    | string (optional)     | `"WxH"`, default `"1024x1024"`     |

Other fields (`quality`, `response_format`, `style`, …) ride through.

### `/v1/images/edits` (multipart/form-data)

Standard OpenAI edit shape: `image`, `mask`, `prompt`, `model`, `n`, `size`, `response_format`. The worker forwards the raw multipart body to the backend with the caller's `Content-Type` (including the boundary) intact. `model`, `n`, `steps`, `size` are read out of the form for metering and capability lookup.

The multipart body is bounded by the worker's universal `maxPaidRequestBodyBytes` (16 MiB).

## Response

`Content-Type: application/json`. Standard OpenAI image-response shape:

```json
{ "created": 1735689600, "data": [{ "url": "..." }] }
```

Response body is buffered and forwarded with the backend's status code.

## Errors

| Condition                                                                  | Status | Body                                                |
| -------------------------------------------------------------------------- | ------ | --------------------------------------------------- |
| Body not JSON (generations) / not multipart (edits)                        | 400    | `{ "error": "invalid_request", "detail": "..." }`  |
| `model` missing                                                            | 400    | `{ "error": "invalid_request", "detail": "..." }`  |
| `size` not parseable as `WxH`                                              | 400    | `{ "error": "invalid_request", "detail": "..." }`  |
| `(capability, model)` not configured                                        | 404    | `{ "error": "capability_not_found" }`               |
| Backend connection / 5xx                                                    | 502    | `{ "error": "backend_unavailable" }`                |
| Worker at `max_concurrent_requests`                                         | 503    | `{ "error": "capacity_exhausted" }`                 |

## Implementation notes

- Files: `internal/service/modules/images_generations/module.go` and `internal/service/modules/images_edits/module.go`.
- Multipart parsing for `/v1/images/edits` uses `internal/service/modules/multipartutil` so the same boundary-extraction code services `/v1/audio/transcriptions`.
- Image backends typically don't return a `usage` block; when one IS present it's currently ignored (the formula above stands as the final charge).
