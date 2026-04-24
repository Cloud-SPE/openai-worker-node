package embeddings

// request captures the fields we need from an OpenAI embeddings
// request: model + input. Input is either a string, a []string, or a
// token-id array — we accept `any` and branch at tokenization time.
type request struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

// response captures only the usage block for reconciliation. Every
// other field (the embedding vectors themselves) streams through the
// worker unmodified.
type response struct {
	Usage *usage `json:"usage,omitempty"`
}

type usage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
