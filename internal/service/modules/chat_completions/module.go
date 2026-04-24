package chat_completions

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/tokenizer"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

// The canonical capability string + HTTP route this module serves.
// Declared as package constants so other packages (lints, tests) can
// reference them without instantiating a Module.
const (
	Capability = types.CapabilityID("openai:/v1/chat/completions")
	HTTPPath   = "/v1/chat/completions"
)

// Module is the chat completions capability adapter. Constructed
// once at worker startup (see New) and handed to
// runtime/http.Mux.RegisterPaidRoute.
type Module struct {
	tok     tokenizer.Tokenizer
	backend backendhttp.Client

	// DefaultMaxCompletionTokens is what EstimateWorkUnits reserves
	// when the request body omits max_tokens. OpenAI's own default is
	// "model-dependent"; 2048 is a practical ceiling for most llama
	// and mistral variants without being wildly wasteful.
	DefaultMaxCompletionTokens int
}

// New wires a chat completions module against the shared tokenizer
// and backend HTTP client providers.
func New(tok tokenizer.Tokenizer, backend backendhttp.Client) *Module {
	return &Module{
		tok:                        tok,
		backend:                    backend,
		DefaultMaxCompletionTokens: 2048,
	}
}

// Assert Module satisfies the modules.Module interface at compile time.
var _ modules.Module = (*Module)(nil)

func (m *Module) Capability() types.CapabilityID { return Capability }
func (m *Module) HTTPMethod() string             { return nethttp.MethodPost }
func (m *Module) HTTPPath() string               { return HTTPPath }

// ExtractModel pulls model out of the request body. Returns an error
// if the body isn't JSON or `model` is missing/empty — the middleware
// converts that into a 400 invalid_request.
func (m *Module) ExtractModel(body []byte) (types.ModelID, error) {
	var r request
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("chat_completions: parse request: %w", err)
	}
	if r.Model == "" {
		return "", errors.New("chat_completions: request is missing `model`")
	}
	return types.ModelID(r.Model), nil
}

// EstimateWorkUnits returns a conservative upper bound on tokens for
// the upfront DebitBalance call. Input-side: tokenize each message's
// stringified content and sum. Output-side: max_tokens if set, else
// DefaultMaxCompletionTokens. The sum is what we reserve upfront.
//
// The model argument is reserved for future per-model tokenizer
// dispatch (see docs/design-docs/ note on per-family tokenizers).
// Currently unused — the tokenizer provider is global.
func (m *Module) EstimateWorkUnits(body []byte, _ types.ModelID) (int64, error) {
	var r request
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, fmt.Errorf("chat_completions: parse request: %w", err)
	}
	input := 0
	for _, msg := range r.Messages {
		input += m.tok.CountTokens(msg.Role)
		input += m.tok.CountTokens(messageContentString(msg.Content))
	}
	output := m.DefaultMaxCompletionTokens
	if r.MaxTokens != nil && *r.MaxTokens > 0 {
		output = *r.MaxTokens
	}
	return int64(input + output), nil
}

// Serve dispatches the request to the configured backend. When the
// bridge requests streaming (stream=true), the SSE response is piped
// through chunk-for-chunk and the usage field on the final chunk
// feeds reconciliation. Otherwise the JSON response is buffered and
// its usage.total_tokens drives reconciliation.
func (m *Module) Serve(
	ctx context.Context,
	w nethttp.ResponseWriter,
	_ *nethttp.Request,
	body []byte,
	_ types.ModelID,
	backendURL string,
) (int64, error) {
	var r request
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, fmt.Errorf("chat_completions: parse request: %w", err)
	}
	targetURL := strings.TrimRight(backendURL, "/") + HTTPPath
	if r.Stream {
		return m.serveStream(ctx, w, targetURL, body)
	}
	return m.serveJSON(ctx, w, targetURL, body)
}

func (m *Module) serveJSON(ctx context.Context, w nethttp.ResponseWriter, url string, body []byte) (int64, error) {
	status, respBody, err := m.backend.DoJSON(ctx, url, body)
	if err != nil {
		nethttp.Error(w, "backend error", nethttp.StatusBadGateway)
		return 0, fmt.Errorf("chat_completions: backend DoJSON: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(respBody)

	// Parse usage for reconciliation. Tolerate missing/malformed
	// usage — fall back to 0 (which means "no over-debit needed").
	var parsed response
	if err := json.Unmarshal(respBody, &parsed); err != nil || parsed.Usage == nil {
		return 0, nil
	}
	return int64(parsed.Usage.TotalTokens), nil
}

func (m *Module) serveStream(ctx context.Context, w nethttp.ResponseWriter, url string, body []byte) (int64, error) {
	status, stream, err := m.backend.DoStream(ctx, url, body)
	if err != nil {
		nethttp.Error(w, "backend error", nethttp.StatusBadGateway)
		return 0, fmt.Errorf("chat_completions: backend DoStream: %w", err)
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(status)

	flusher, _ := w.(nethttp.Flusher)

	var lastUsage *usage
	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Pass the raw line (including the trailing \n) to the
			// client. We mustn't rewrite SSE framing — a single \n
			// elsewhere in a data payload would be a protocol bug
			// caused by the backend, not us.
			if _, werr := w.Write(line); werr != nil {
				// Client disconnected; stop reading upstream.
				return usageToUnits(lastUsage), werr
			}
			if flusher != nil {
				flusher.Flush()
			}
			// Peek the data portion for usage. SSE lines can carry
			// comments (": keep-alive") and named events; we only
			// look at `data: {...}` lines with JSON payloads.
			if u := tryParseStreamUsage(line); u != nil {
				lastUsage = u
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return usageToUnits(lastUsage), fmt.Errorf("chat_completions: read backend stream: %w", err)
		}
	}
	return usageToUnits(lastUsage), nil
}

// usageToUnits converts an optional usage block to the int64
// work_units value the middleware reconciles against.
func usageToUnits(u *usage) int64 {
	if u == nil {
		return 0
	}
	return int64(u.TotalTokens)
}

// tryParseStreamUsage extracts the `usage` block from a single SSE
// line if present. Returns nil for non-data lines, lines without
// usage, or malformed JSON.
func tryParseStreamUsage(line []byte) *usage {
	line = bytes.TrimSpace(line)
	if !bytes.HasPrefix(line, []byte("data:")) {
		return nil
	}
	payload := bytes.TrimSpace(line[len("data:"):])
	// [DONE] terminator.
	if bytes.Equal(payload, []byte("[DONE]")) {
		return nil
	}
	var chunk streamChunk
	if err := json.Unmarshal(payload, &chunk); err != nil {
		return nil
	}
	return chunk.Usage
}

// messageContentString coerces a message's content field (which can
// be a string, or an array of content-part objects for multimodal) to
// a single string for tokenization purposes. Approximation, not
// canonicalization — the backend sees the original bytes.
func messageContentString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		// Multi-part content: concat text parts; render others as
		// their JSON string, which over-counts but is safe.
		var b strings.Builder
		for _, part := range x {
			if pm, ok := part.(map[string]any); ok {
				if t, ok := pm["text"].(string); ok {
					b.WriteString(t)
					b.WriteByte(' ')
					continue
				}
			}
			raw, _ := json.Marshal(part)
			b.Write(raw)
			b.WriteByte(' ')
		}
		return b.String()
	default:
		raw, _ := json.Marshal(v)
		return string(raw)
	}
}
