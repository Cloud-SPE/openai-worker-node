// Package audio_speech implements the openai:/v1/audio/speech
// capability (TTS). Request is OpenAI-compatible JSON: model + input
// + voice + response_format (optional). Response is raw audio bytes
// streamed back to the caller; Content-Type is backend-determined
// (audio/mpeg, audio/wav, audio/opus, etc.).
//
// Work unit = `character`. Actual == estimate: the request body's
// `input` length (in UTF-8 bytes counted as characters) is the final
// cost. This is deterministic and doesn't need reconciliation.
//
// The ASR sibling /v1/audio/transcriptions is multipart/form-data and
// tracked separately under `multipart-capability-handling` tech-debt.
package audio_speech
