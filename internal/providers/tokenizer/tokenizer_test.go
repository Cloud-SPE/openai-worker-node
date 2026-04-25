package tokenizer

import "testing"

func TestWordCount_Empty(t *testing.T) {
	tok := NewWordCount(133)
	if got := tok.CountTokens(""); got != 0 {
		t.Errorf("empty: got %d, want 0", got)
	}
	if got := tok.CountTokens("   "); got != 0 {
		t.Errorf("whitespace-only: got %d, want 0", got)
	}
}

func TestWordCount_SingleWord(t *testing.T) {
	tok := NewWordCount(133)
	// 1 word × 1.33 → ceil = 2
	if got := tok.CountTokens("hello"); got != 2 {
		t.Errorf("single word: got %d, want 2 (1 × 1.33 → 2)", got)
	}
}

func TestWordCount_MultiWord(t *testing.T) {
	tok := NewWordCount(133)
	// "hello world foo" → 3 words × 1.33 = 3.99 → 4
	if got := tok.CountTokens("hello world foo"); got != 4 {
		t.Errorf("three words: got %d, want 4", got)
	}
}

func TestWordCount_MixedWhitespace(t *testing.T) {
	tok := NewWordCount(133)
	// 4 words across mixed separators
	got := tok.CountTokens("a\tb\nc  d")
	// 4 × 1.33 = 5.32 → 6
	if got != 6 {
		t.Errorf("mixed whitespace: got %d, want 6", got)
	}
}

func TestWordCount_DefaultMultiplier(t *testing.T) {
	tok := NewWordCount(0) // invalid → falls back to 133
	if got := tok.CountTokens("one two three four"); got != 6 {
		t.Errorf("default multiplier: got %d, want 6 (4 × 1.33)", got)
	}
}

func TestWordCount_ExactMultiplier100(t *testing.T) {
	tok := NewWordCount(100)
	// 1× multiplier: token count == word count
	if got := tok.CountTokens("one two three"); got != 3 {
		t.Errorf("1.0× multiplier: got %d, want 3", got)
	}
}

func TestCountJoined(t *testing.T) {
	tok := NewWordCount(100)
	// "a b c" → 3 words × 1.0 = 3
	if got := CountJoined(tok, []string{"a", "b", "c"}); got != 3 {
		t.Errorf("joined: got %d, want 3", got)
	}
}

func TestWordCount_CountTokensForModel_IgnoresModel(t *testing.T) {
	tok := NewWordCount(100)
	a := tok.CountTokensForModel("gpt-4o", "one two three")
	b := tok.CountTokensForModel("", "one two three")
	c := tok.CountTokens("one two three")
	if a != b || b != c || c != 3 {
		t.Errorf("expected model-blind: got a=%d b=%d c=%d, want all 3", a, b, c)
	}
}
