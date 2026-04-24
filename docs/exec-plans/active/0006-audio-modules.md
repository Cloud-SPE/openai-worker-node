---
id: 0006
slug: audio-modules
title: Capability modules — openai:/v1/audio/speech + /audio/transcriptions
status: active
owner: unclaimed
opened: 2026-04-24
---

## Goal

Add both audio capabilities. They are asymmetric enough to warrant separate modules:

- `/v1/audio/speech` (TTS) — streams audio bytes back, work-unit is output characters.
- `/v1/audio/transcriptions` (ASR) — multipart upload in, JSON out, work-unit is input audio seconds (rounded up).

## Non-goals

- No audio parsing client-side to get accurate duration. ASR reservation uses tier-max; the worker reconciles from backend-reported duration (or input-file size as a fallback heuristic).

## Cross-repo dependencies

- Library `0018-per-capability-pricing`.
- This repo: `0003-payment-middleware`, `0002-chat-completions-module`, `0005-images-module` (for multipart upload precedent).

## Approach

- [ ] Module at `internal/service/modules/audio_speech/` (TTS).
  - `EstimateWorkUnits`: len(input) characters. Deterministic.
  - `Serve`: POST to backend, stream audio bytes (content-type: `audio/mpeg` typically) through.
- [ ] Module at `internal/service/modules/audio_transcriptions/` (ASR).
  - `EstimateWorkUnits`: tier-max reservation. The request itself doesn't cheaply reveal audio duration without decoding.
  - `Serve`: POST multipart to backend. On response, extract duration (or segments sum) from backend JSON. Reconcile.
- [ ] Integration tests for both with fake backends.

## Decisions log

_Empty._

## Open questions

- **Tier-max for ASR.** Is there a config-driven cap on per-request audio seconds (e.g., `max_audio_seconds: 1800`)? Needed to bound the upfront DebitBalance. Default proposal: 1800s (30 min). Revisit as use cases firm up.
- **Partial / failed ASR responses.** If backend returns partial text with a mid-stream error, do we debit for what we got? Lean yes — actual work was done by the backend even if incomplete.

## Artifacts produced

_Not started._
