package tokenizer

import (
	"strings"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/metrics"
)

// Tokenizer produces a conservative token-count estimate for a UTF-8
// string. Used by chat_completions (sum of messages + max_tokens) and
// embeddings (input tokens).
//
// Two methods:
//
//   - CountTokens(s)               — model-agnostic count. Used when
//     the caller doesn't have a model or wants a generic estimate.
//   - CountTokensForModel(model,s) — model-aware count. Implementations
//     that know a per-model tokenizer (tiktoken for OpenAI families)
//     should use it; implementations that don't may delegate.
//
// Implementations MUST be deterministic and safe for concurrent use —
// modules call these on many goroutines against one instance.
type Tokenizer interface {
	CountTokens(s string) int
	CountTokensForModel(model string, s string) int
}

// NewWordCount returns a naive tokenizer that splits on whitespace and
// multiplies by ceilRatio to approximate BPE expansion.
//
// multiplier is fixed-point ×100 (i.e. 133 means 1.33×). OpenAI's
// published rule of thumb is ~0.75 words per token, so tokens ≈ words
// / 0.75 ≈ words × 1.33.
//
// Empty input returns 0. Whitespace-only returns 0 (no tokens).
//
// Word-count is model-agnostic — `CountTokensForModel` ignores the
// model argument and falls through to `CountTokens`.
func NewWordCount(multiplierPct int) Tokenizer {
	if multiplierPct <= 0 {
		multiplierPct = 133
	}
	return &wordCount{mul: multiplierPct}
}

type wordCount struct {
	mul int
}

func (w *wordCount) CountTokens(s string) int {
	if s == "" {
		return 0
	}
	words := 0
	inToken := false
	for _, r := range s {
		if isSep(r) {
			inToken = false
			continue
		}
		if !inToken {
			words++
			inToken = true
		}
	}
	if words == 0 {
		return 0
	}
	// Round up: (words * mul + 99) / 100 — avoids silent under-count.
	return (words*w.mul + 99) / 100
}

func (w *wordCount) CountTokensForModel(_ string, s string) int {
	return w.CountTokens(s)
}

// isSep returns true for ASCII whitespace. Keeping this inline lets us
// avoid importing unicode for a performance-sensitive path; non-ASCII
// languages will split differently on BPE anyway, so exact whitespace
// handling isn't critical for a placeholder estimator.
func isSep(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
}

// Helper for callers that want to count across a pre-joined string
// rather than many small CountTokens calls. Same math; exists so
// module code reads naturally.
func CountJoined(t Tokenizer, parts []string) int {
	return t.CountTokens(strings.Join(parts, " "))
}

// WithMetrics wraps a Tokenizer so each count emits an
// IncTokenizerCall. Latency is intentionally skipped — the tiktoken
// hot path is sub-100µs and the design doc deliberately omits an
// Observe for tokenizer.
//
// CountTokens has no model context; the model label is emitted as the
// recorder's _unset_ sentinel for that path. CountTokensForModel
// passes through the caller-supplied model.
//
// The tokenizer doesn't return errors — the underlying interface
// returns int — so outcome is always OutcomeOK in this Pass A wrapper.
// Future implementations that distinguish a tiktoken-vs-fallback path
// may emit OutcomeFallback by extending the Tokenizer interface (Pass
// B); the constants are already defined.
func WithMetrics(t Tokenizer, rec metrics.Recorder) Tokenizer {
	if rec == nil {
		return t
	}
	return &meteredTokenizer{inner: t, rec: rec}
}

type meteredTokenizer struct {
	inner Tokenizer
	rec   metrics.Recorder
}

func (m *meteredTokenizer) CountTokens(s string) int {
	n := m.inner.CountTokens(s)
	m.rec.IncTokenizerCall("", metrics.OutcomeOK)
	return n
}

func (m *meteredTokenizer) CountTokensForModel(model, s string) int {
	n := m.inner.CountTokensForModel(model, s)
	m.rec.IncTokenizerCall(model, metrics.OutcomeOK)
	return n
}
