// Package audio_transcriptions implements the openai:/v1/audio/transcriptions
// capability (ASR). Request is multipart/form-data: `file` (required,
// audio bytes), `model`, optional `language`, `prompt`, `response_format`,
// `temperature`. Response is JSON (text + optional segments).
//
// Work unit = `audio_second`. Accurately metering requires decoding
// the audio file to learn its duration, which brings a heavyweight
// dep (ffmpeg-go). For v1 we use a tier-max reservation (configurable
// default) and rely on the backend's response to carry actual
// duration — when available — for reconciliation.
//
// The response shape differs by `response_format`: default `json`
// returns `{"text": "..."}` with no usage; `verbose_json` adds
// `duration` (in seconds). Other formats (`text`, `srt`, `vtt`) are
// opaque strings. If duration is visible we reconcile; otherwise the
// estimate stands.
package audio_transcriptions
