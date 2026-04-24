---
id: 0006
slug: audio-modules
title: Capability modules — openai:/v1/audio/speech + /audio/transcriptions
status: completed
owner: agent
opened: 2026-04-24
completed: 2026-04-24
---

## Goal

Add the audio capabilities. This plan landed `/v1/audio/speech` (TTS)
end-to-end. `/v1/audio/transcriptions` (ASR) is deferred to the shared
`multipart-capability-handling` plan alongside `/v1/images/edits`.

## Non-goals

- `/v1/audio/transcriptions` — multipart body. Shared with `/v1/images/edits`; tracked under `multipart-capability-handling` tech-debt.

## Cross-repo dependencies

- Library `0018-per-capability-pricing` (landed).
- This repo: `0003-payment-middleware` (landed), `0002-chat-completions-module` (landed), `0005-images-module` (precedent for multipart deferral).

## Approach

- [x] Module at `internal/service/modules/audio_speech/`.
  - `EstimateWorkUnits`: rune count of `input` (UTF-8). Deterministic.
  - `Serve`: POST to backend via `DoStream`, pipe audio bytes back with `Content-Type: audio/mpeg` default (backend-specific content types are a follow-up once we add header forwarding from DoStream responses).
- [x] Integration tests: 7 cases — Extract happy/missing, Estimate ASCII/unicode/empty, Serve happy/backend-error, capability-path.
- [x] cmd wiring: case added to `registerModules`.

Deferred:

- [ ] `/v1/audio/transcriptions` — multipart. Owned by the shared multipart plan.
- [ ] Content-type passthrough from backend to caller. Currently defaults to `audio/mpeg`; should relay the backend's actual `Content-Type` header. Small; tracked under `audio-speech-content-type-passthrough` tech-debt.

## Decisions log

### 2026-04-24 — Rune count, not byte count, for TTS metering

Emoji and multi-byte scripts would wildly over-charge on byte count. `utf8.RuneCountInString` matches what a human reader perceives as "characters." Matches OpenAI's historical TTS billing model.

### 2026-04-24 — Default Content-Type to audio/mpeg

`backendhttp.DoStream` doesn't currently expose the backend's response headers to the module. Defaulting to `audio/mpeg` (OpenAI's TTS default) is correct for the common case; overriding formats (opus, wav) will need a provider-surface change. Small follow-up, tracked.

### 2026-04-24 — ASR split to the multipart plan

Same reasoning as images/edits: one shared answer is better than two ad-hoc ones. Tracked as `multipart-capability-handling`.

## Open questions

All resolved.

## Artifacts produced

Files landed:

- `internal/service/modules/audio_speech/{doc,module,module_test}.go`
- `cmd/openai-worker-node/main.go` — audio_speech case added to `registerModules`.
