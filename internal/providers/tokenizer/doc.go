// Package tokenizer estimates token counts for billing purposes. The
// interface (CountTokens, CountTokensForModel) lets different backends
// drop in without touching modules:
//
//   - WordCount — naive word-split × 1.33 multiplier. Model-blind. Used
//     as the dev default and as the last-resort fallback inside the
//     tiktoken impl.
//   - Tiktoken — backed by github.com/pkoukk/tiktoken-go. Resolves a
//     per-model encoding for OpenAI-family models (gpt-3.5/4/4o/...) and
//     falls back to cl100k_base for unknown models. This is the
//     production default wired in cmd/openai-worker-node/main.go.
//
// # Why only an estimate matters
//
// The chat_completions module reconciles after the response: actual
// token counts come from the backend's usage.total_tokens field in
// the final SSE chunk (or the buffered response for non-streaming).
// The tokenizer's job is to produce a conservative upfront number for
// the initial DebitBalance call. Over-estimation is safe — the
// worker's over-debit policy absorbs the slack.
//
// # Choosing an implementation
//
// Production:  NewTiktoken(NewWordCount(133))
// Dev / tests: NewWordCount(133) (deterministic, no embedded data load)
package tokenizer
