package embeddings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/tokenizer"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

// Canonical identifiers for this capability. Other packages (tests,
// main wiring, lints) reference these rather than string literals.
const (
	Capability = types.CapabilityID("openai:/v1/embeddings")
	HTTPPath   = "/v1/embeddings"
)

// Module adapts openai:/v1/embeddings. Stateless; safe for concurrent
// use — all state is in ctx + body + providers.
type Module struct {
	tok     tokenizer.Tokenizer
	backend backendhttp.Client
}

// New wires the module against the shared tokenizer and backend HTTP
// provider. Called once from cmd/openai-worker-node/main.go.
func New(tok tokenizer.Tokenizer, backend backendhttp.Client) *Module {
	return &Module{tok: tok, backend: backend}
}

// Compile-time interface check.
var _ modules.Module = (*Module)(nil)

func (m *Module) Capability() types.CapabilityID { return Capability }
func (m *Module) HTTPMethod() string             { return nethttp.MethodPost }
func (m *Module) HTTPPath() string               { return HTTPPath }

// ExtractModel pulls `model` out of the request JSON.
func (m *Module) ExtractModel(body []byte) (types.ModelID, error) {
	var r request
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("embeddings: parse request: %w", err)
	}
	if r.Model == "" {
		return "", errors.New("embeddings: request is missing `model`")
	}
	return types.ModelID(r.Model), nil
}

// EstimateWorkUnits counts tokens across the `input` field. Supports:
//
//   - string           → tokenize directly.
//   - []string         → sum tokenize(each).
//   - []any of numbers → token-id array; return len as token count.
//
// Any other shape returns 0 (conservative; the backend will reject
// the malformed request).
//
// The model argument is forwarded to the tokenizer so per-family
// encodings (tiktoken's text-embedding-3-* uses cl100k_base) can be
// applied.
func (m *Module) EstimateWorkUnits(body []byte, model types.ModelID) (int64, error) {
	var r request
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, fmt.Errorf("embeddings: parse request: %w", err)
	}
	return int64(m.countTokens(string(model), r.Input)), nil
}

// Serve posts the request to the backend verbatim and writes the
// buffered response through. Reads usage.total_tokens for
// reconciliation; falls back to 0 when usage is missing.
func (m *Module) Serve(
	ctx context.Context,
	w nethttp.ResponseWriter,
	_ *nethttp.Request,
	body []byte,
	_ types.ModelID,
	backendURL string,
) (int64, error) {
	targetURL := strings.TrimRight(backendURL, "/") + HTTPPath
	status, respBody, err := m.backend.DoJSON(ctx, targetURL, body)
	if err != nil {
		nethttp.Error(w, "backend error", nethttp.StatusBadGateway)
		return 0, fmt.Errorf("embeddings: backend DoJSON: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(respBody)

	var parsed response
	if err := json.Unmarshal(respBody, &parsed); err != nil || parsed.Usage == nil {
		return 0, nil
	}
	return int64(parsed.Usage.TotalTokens), nil
}

// countTokens walks whatever shape `input` happens to be and returns a
// token-count estimate. The model parameter routes to the tokenizer's
// per-model encoding when one is known.
func (m *Module) countTokens(model string, input any) int {
	switch v := input.(type) {
	case string:
		return m.tok.CountTokensForModel(model, v)
	case []any:
		// Two legal shapes: []string (wrapped as []any by json) or
		// []int token IDs (also []any). For token IDs, len is the
		// canonical count; for strings, sum tokenize(each).
		total := 0
		for _, item := range v {
			switch iv := item.(type) {
			case string:
				total += m.tok.CountTokensForModel(model, iv)
			case float64:
				// JSON numbers decode to float64 through `any`. A
				// token-id entry contributes exactly one token to
				// the count, regardless of numeric value.
				total++
			default:
				// Unknown shape — safer to count as one than zero.
				total++
			}
		}
		return total
	default:
		return 0
	}
}
