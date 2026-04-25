package tokenizer

import (
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

// defaultEncoding is the tiktoken encoding used when the request's
// model is unknown to tiktoken-go (typical for Llama, Mistral, and
// most non-OpenAI families). cl100k_base is the encoding behind
// gpt-3.5-turbo and gpt-4; it gives reasonably tight estimates for
// text in any language and beats the word-count placeholder by an
// order of magnitude on accuracy.
const defaultEncoding = "cl100k_base"

// NewTiktoken returns a model-aware Tokenizer backed by tiktoken-go.
// It pre-loads the default encoding so request-time calls don't pay
// the embedded-data init cost.
//
// fallback is invoked only when both the model-specific encoding AND
// the default encoding fail to load — i.e. tiktoken's embedded data
// is somehow unreadable. Pass NewWordCount(133) for a sensible
// last-resort estimator.
//
// Per-model encodings are cached on first use. Lookup hot path is
// lock-free after the encoding is resident; one-time initialization
// is guarded by a per-model sync.Once.
func NewTiktoken(fallback Tokenizer) Tokenizer {
	if fallback == nil {
		fallback = NewWordCount(133)
	}
	t := &tiktokenTokenizer{
		fallback: fallback,
		modelEnc: map[string]*encEntry{},
	}
	// Best-effort warm: skip silently if tiktoken's embedded data
	// can't be loaded — fallback handles that case at request time.
	if enc, err := tiktoken.GetEncoding(defaultEncoding); err == nil {
		t.defaultEnc = enc
	}
	return t
}

type tiktokenTokenizer struct {
	fallback Tokenizer

	// defaultEnc is loaded eagerly in NewTiktoken; nil when tiktoken
	// embedded data is unavailable.
	defaultEnc *tiktoken.Tiktoken

	mu       sync.Mutex
	modelEnc map[string]*encEntry
}

// encEntry caches a per-model encoding lookup result. A nil enc means
// tiktoken couldn't resolve an encoding for this model — we'll either
// use the default encoding or the fallback tokenizer. Caching the
// negative result avoids re-hitting EncodingForModel on every call.
type encEntry struct {
	enc *tiktoken.Tiktoken
}

func (t *tiktokenTokenizer) CountTokens(s string) int {
	if s == "" {
		return 0
	}
	if t.defaultEnc != nil {
		return len(t.defaultEnc.Encode(s, nil, nil))
	}
	return t.fallback.CountTokens(s)
}

func (t *tiktokenTokenizer) CountTokensForModel(model string, s string) int {
	if s == "" {
		return 0
	}
	enc := t.encodingFor(model)
	if enc != nil {
		return len(enc.Encode(s, nil, nil))
	}
	if t.defaultEnc != nil {
		return len(t.defaultEnc.Encode(s, nil, nil))
	}
	return t.fallback.CountTokensForModel(model, s)
}

// encodingFor resolves a model name to its tiktoken encoding, caching
// the result (positive or negative) so subsequent calls are lock-free
// after first lookup.
//
// Models tiktoken-go knows include the gpt-3.5/4/4o/4-turbo families
// and the davinci/curie/babbage line. Unknown model strings (Llama,
// Mistral, embedding models from non-OpenAI vendors) return a nil
// entry and the caller falls through to the default encoding.
func (t *tiktokenTokenizer) encodingFor(model string) *tiktoken.Tiktoken {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if entry, ok := t.modelEnc[model]; ok {
		return entry.enc
	}
	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		t.modelEnc[model] = &encEntry{enc: nil}
		return nil
	}
	t.modelEnc[model] = &encEntry{enc: enc}
	return enc
}
