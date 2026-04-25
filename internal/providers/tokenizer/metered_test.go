package tokenizer

import (
	"testing"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/metrics"
)

func TestMeteredTokenizer_CountTokensIncrementsCounter(t *testing.T) {
	rec := metrics.NewCounter()
	tok := WithMetrics(NewWordCount(100), rec)
	if got := tok.CountTokens("a b c"); got != 3 {
		t.Fatalf("count = %d", got)
	}
	if rec.TokenizerCalls.Load() != 1 {
		t.Fatalf("tokenizer calls = %d", rec.TokenizerCalls.Load())
	}
	if got := rec.LastTokenizerOutcome.Load(); got != metrics.OutcomeOK {
		t.Fatalf("outcome = %v", got)
	}
}

func TestMeteredTokenizer_CountTokensForModelIncrementsCounter(t *testing.T) {
	rec := metrics.NewCounter()
	tok := WithMetrics(NewWordCount(100), rec)
	if got := tok.CountTokensForModel("gpt-4o", "a b c"); got != 3 {
		t.Fatalf("count = %d", got)
	}
	if rec.TokenizerCalls.Load() != 1 {
		t.Fatalf("tokenizer calls = %d", rec.TokenizerCalls.Load())
	}
	if got := rec.LastTokenizerOutcome.Load(); got != metrics.OutcomeOK {
		t.Fatalf("outcome = %v", got)
	}
}

func TestMeteredTokenizer_CountsAccumulate(t *testing.T) {
	rec := metrics.NewCounter()
	tok := WithMetrics(NewWordCount(100), rec)
	tok.CountTokens("a")
	tok.CountTokensForModel("gpt-4o", "b")
	tok.CountTokens("c d e f")
	if rec.TokenizerCalls.Load() != 3 {
		t.Fatalf("expected 3 calls, got %d", rec.TokenizerCalls.Load())
	}
}

func TestMeteredTokenizer_PassthroughResults(t *testing.T) {
	// The wrapper must not alter the underlying count.
	rec := metrics.NewCounter()
	inner := NewWordCount(133)
	wrapped := WithMetrics(inner, rec)
	if a, b := inner.CountTokens("hello world foo"), wrapped.CountTokens("hello world foo"); a != b {
		t.Fatalf("wrapper changed count: inner=%d wrapped=%d", a, b)
	}
}

func TestMeteredTokenizer_NilRecorderReturnsInner(t *testing.T) {
	inner := NewWordCount(100)
	if got := WithMetrics(inner, nil); got != inner {
		t.Fatal("nil recorder should return the inner tokenizer unchanged")
	}
}
