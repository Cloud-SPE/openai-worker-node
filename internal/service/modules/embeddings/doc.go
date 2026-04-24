// Package embeddings implements the openai:/v1/embeddings capability.
// Request/response shape is OpenAI-compatible; the worker forwards the
// bridge's bytes verbatim to the backend (typically a TEI-compatible
// server or vLLM in embedding mode).
//
// Embeddings are one-shot (no streaming). Work unit = input tokens.
// The estimator counts tokens across the `input` field (which can be
// a string OR a []string OR — per OpenAI's more obscure form — a
// []int of token IDs, which we count as len(ids)).
//
// Actual work units come from the backend's `usage.total_tokens`. For
// embedding-only backends this typically equals `usage.prompt_tokens`
// since there is no completion.
package embeddings
