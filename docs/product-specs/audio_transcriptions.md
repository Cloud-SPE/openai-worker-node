---
title: POST /v1/audio/transcriptions
status: live
last-reviewed: 2026-04-25
module: internal/service/modules/audio_transcriptions
---

# POST /v1/audio/transcriptions

OpenAI-compatible speech-to-text endpoint. **Multipart** request in (audio file + form fields), JSON or text response out.

## Request

`Content-Type: multipart/form-data`. Form fields:

| Field             | Type   | Used for                                                  |
| ----------------- | ------ | --------------------------------------------------------- |
| `model`           | string | Capability/model lookup → backend URL                     |
| `file`            | binary | The audio. Forwarded raw to the backend                   |
| `prompt`          | string | (optional) Forwarded verbatim                             |
| `response_format` | string | (optional) `json` \| `text` \| `srt` \| `verbose_json` \| `vtt`. Forwarded verbatim |
| `temperature`     | number | (optional) 0.0–1.0. Forwarded verbatim                    |
| `language`        | string | (optional) ISO-639-1. Forwarded verbatim                  |

The worker reads `model` only; everything else is forwarded by relaying the raw multipart body (boundary preserved) via `backendhttp.DoRaw`.

The multipart body is bounded by the worker's universal `maxPaidRequestBodyBytes` (16 MiB).

## Metering

- **Capability**: `openai:/v1/audio/transcriptions`
- **Work unit**: `audio_second`
- **Upfront estimate**: `Module.MaxAudioSecondsCeil` — defaults to 3600 (one hour). The worker cannot determine duration without decoding the file, which is left to the backend.
- **Actual**: parsed from the backend response's `duration` field (present when `response_format=verbose_json`). Rounded up to the nearest second.
- **Reconciliation**: when the backend reports a duration, the middleware reconciles in either direction:
  - `actual > estimate` → second `DebitBalance(delta)`. (Only happens if operators set `MaxAudioSecondsCeil` below 3600 and a longer file slipped past.)
  - `actual < estimate` → no refund (over-debit policy stands; the bridge's customer-facing refund is the bridge's responsibility).
- **Missing duration** (response is plain `{"text":...}`, or response_format=text/srt/vtt without verbose_json) → estimate stands as the final charge. The bridge's contract requires a duration header in this case; the worker passes the body through and the bridge's reconciliation handles the missing-duration → 503 + refund flow on its side.

## Response

`Content-Type: application/json` (relayed Content-Type from the backend is a known limitation; see implementation notes). Body is buffered and forwarded with the backend's status code.

`response_format=verbose_json` example:

```json
{
  "task": "transcribe",
  "language": "en",
  "duration": 12.34,
  "text": "Hello world.",
  "segments": [...]
}
```

`response_format=text` returns plain text in the body; the worker forwards it but currently labels it `application/json` (limitation tracked under the same content-type-passthrough debt as resolved for `/v1/audio/speech`; transcriptions still rides the older path).

## Errors

| Condition                                                   | Status | Body                                                |
| ----------------------------------------------------------- | ------ | --------------------------------------------------- |
| `Content-Type` not `multipart/form-data`                    | 400    | `{ "error": "invalid_request", "detail": "..." }`  |
| Body not parseable as multipart, or `model` field missing   | 400    | `{ "error": "invalid_request", "detail": "..." }`  |
| `(capability, model)` not configured                         | 404    | `{ "error": "capability_not_found" }`               |
| Backend connection / 5xx                                     | 502    | `{ "error": "backend_unavailable" }`                |
| Worker at `max_concurrent_requests`                          | 503    | `{ "error": "capacity_exhausted" }`                 |

The backend is the authoritative oracle for "is this file's MIME / codec supported?" — a 400 from the backend is forwarded to the bridge and on to the customer.

## Implementation notes

- File: `internal/service/modules/audio_transcriptions/module.go`.
- `MaxAudioSecondsCeil` is set on the Module struct after `New(backend)`; operators with tighter expectations can lower it to e.g. 600 (10 minutes) for tier-segmented deployments.
- `durationFromResponse` accepts either a JSON number or a string-valued `duration` field for tolerance.
- Multipart form-field reads use `internal/service/modules/multipartutil` so the same boundary-extraction code services `/v1/images/edits`.
