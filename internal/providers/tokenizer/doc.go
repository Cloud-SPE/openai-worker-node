// Package tokenizer estimates token counts for billing purposes. The
// interface is minimal (CountTokens(string) int) so different backends
// (tiktoken for OpenAI-family models, sentencepiece for llama-family,
// a simple word-split approximation for dev) can drop in without
// touching modules.
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
// # v1 default
//
// The default implementation is WordCount — it splits on whitespace
// and applies a 1.33× multiplier (≈ OpenAI's words-to-tokens ratio).
// Good enough for dev and for over-debiting production traffic; a
// tiktoken-go-backed impl is a future provider swap.
package tokenizer
