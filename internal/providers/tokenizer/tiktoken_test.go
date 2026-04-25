package tokenizer

import (
	"strings"
	"sync"
	"testing"
)

// fakeFallback records every call so we can assert when (and only when)
// tiktoken delegates to the fallback path.
type fakeFallback struct {
	mu                  sync.Mutex
	countCalls          int
	countForModelCalls  int
}

func (f *fakeFallback) CountTokens(s string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.countCalls++
	// Sentinel that's distinguishable from any plausible tiktoken output.
	return -1
}

func (f *fakeFallback) CountTokensForModel(_ string, s string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.countForModelCalls++
	return -2
}

// Sanity: tiktoken-go's embedded data should load in tests, so fallback
// is never invoked. If tiktoken's init fails for any reason in CI, the
// fallback is the safety net.
func TestTiktoken_KnownModelEncodesNonZero(t *testing.T) {
	tok := NewTiktoken(NewWordCount(100))
	got := tok.CountTokensForModel("gpt-4o-mini", "Hello, world!")
	if got <= 0 {
		t.Fatalf("expected a positive token count for a known OpenAI model, got %d", got)
	}
}

func TestTiktoken_EmptyInputIsZero(t *testing.T) {
	tok := NewTiktoken(NewWordCount(100))
	if got := tok.CountTokens(""); got != 0 {
		t.Errorf("empty CountTokens: got %d, want 0", got)
	}
	if got := tok.CountTokensForModel("gpt-4o", ""); got != 0 {
		t.Errorf("empty CountTokensForModel: got %d, want 0", got)
	}
}

func TestTiktoken_UnknownModelFallsBackToDefaultEncoding(t *testing.T) {
	fb := &fakeFallback{}
	tok := NewTiktoken(fb)
	// "llama-3-70b" is not in tiktoken's model registry. The impl
	// should fall through to the default cl100k_base encoding —
	// fakeFallback should NOT be touched as long as embedded data
	// loaded successfully.
	got := tok.CountTokensForModel("llama-3-70b", "the quick brown fox")
	if got <= 0 {
		t.Fatalf("expected default-encoding fallback to produce a positive count, got %d", got)
	}
	fb.mu.Lock()
	defer fb.mu.Unlock()
	if fb.countForModelCalls > 0 {
		t.Errorf("expected NO fallback delegation when default encoding is loaded, but fallback was called %d time(s)",
			fb.countForModelCalls)
	}
}

func TestTiktoken_PerModelEncodingIsCached(t *testing.T) {
	tok := NewTiktoken(NewWordCount(100)).(*tiktokenTokenizer)
	_ = tok.CountTokensForModel("gpt-4o-mini", "a")
	_ = tok.CountTokensForModel("gpt-4o-mini", "b")
	_ = tok.CountTokensForModel("custom-unknown-model", "c")
	_ = tok.CountTokensForModel("custom-unknown-model", "d")
	tok.mu.Lock()
	defer tok.mu.Unlock()
	if _, ok := tok.modelEnc["gpt-4o-mini"]; !ok {
		t.Errorf("expected gpt-4o-mini cached")
	}
	if entry, ok := tok.modelEnc["custom-unknown-model"]; !ok || entry.enc != nil {
		t.Errorf("expected unknown-model cached as a negative entry, got %+v ok=%v", entry, ok)
	}
}

func TestTiktoken_ConcurrentSafe(t *testing.T) {
	tok := NewTiktoken(NewWordCount(100))
	var wg sync.WaitGroup
	wg.Add(8)
	for i := 0; i < 8; i++ {
		go func(i int) {
			defer wg.Done()
			model := "gpt-4o-mini"
			if i%2 == 0 {
				model = "gpt-3.5-turbo"
			}
			for j := 0; j < 50; j++ {
				_ = tok.CountTokensForModel(model, strings.Repeat("a ", j))
			}
		}(i)
	}
	wg.Wait()
}

func TestTiktoken_BlankModelUsesDefault(t *testing.T) {
	fb := &fakeFallback{}
	tok := NewTiktoken(fb)
	got := tok.CountTokensForModel("   ", "Hello world")
	if got <= 0 {
		t.Fatalf("expected default-encoding count for blank model, got %d", got)
	}
	fb.mu.Lock()
	defer fb.mu.Unlock()
	if fb.countForModelCalls > 0 {
		t.Errorf("expected NO fallback delegation for blank model, got %d call(s)", fb.countForModelCalls)
	}
}
