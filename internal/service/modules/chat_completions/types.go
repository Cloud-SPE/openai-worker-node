package chat_completions

// request is the narrow slice of OpenAI's chat completions request
// body the worker needs to parse. Extra fields (messages, tools,
// temperature, etc.) are NOT declared — they ride through verbatim
// as raw JSON bytes since we never modify them. This mirror exists
// only to pull out model + max_tokens + stream for routing and
// metering.
type request struct {
	Model     string           `json:"model"`
	MaxTokens *int             `json:"max_tokens,omitempty"`
	Stream    bool             `json:"stream,omitempty"`
	Messages  []requestMessage `json:"messages"`
}

// requestMessage captures just enough to feed the tokenizer — role
// plus content. Multi-part content (images, tool calls) is flattened
// to its JSON rendering for counting purposes; that over-estimates,
// which the over-debit policy absorbs.
type requestMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// response is the narrow slice of OpenAI's chat completions response
// we consume. Only usage matters — everything else streams through.
type response struct {
	Usage *usage `json:"usage,omitempty"`
}

// streamChunk is the per-SSE-event JSON shape. Usage lives on the
// final chunk emitted by most OpenAI-compatible backends (vLLM, the
// official API, ollama ≥ 0.5).
type streamChunk struct {
	Usage *usage `json:"usage,omitempty"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
