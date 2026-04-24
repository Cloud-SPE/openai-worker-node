package chat_completions

import (
	"context"
	"encoding/json"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/tokenizer"
)

func newModule(backend backendhttp.Client) *Module {
	return New(tokenizer.NewWordCount(100), backend)
}

func TestExtractModel(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	model, err := m.ExtractModel([]byte(`{"model":"llama-3.3-70b","messages":[]}`))
	if err != nil {
		t.Fatalf("ExtractModel: %v", err)
	}
	if model != "llama-3.3-70b" {
		t.Errorf("got %q, want llama-3.3-70b", model)
	}
}

func TestExtractModel_Missing(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	if _, err := m.ExtractModel([]byte(`{"messages":[]}`)); err == nil {
		t.Error("expected error when model missing")
	}
}

func TestExtractModel_BadJSON(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	if _, err := m.ExtractModel([]byte(`not json`)); err == nil {
		t.Error("expected error on invalid JSON")
	}
}

func TestEstimateWorkUnits_DefaultMaxTokens(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hello world"}]}`)
	units, err := m.EstimateWorkUnits(body, "m")
	if err != nil {
		t.Fatalf("EstimateWorkUnits: %v", err)
	}
	// With word-count at 1.0×:
	//   "user" → 1 word; "hello world" → 2 words; total input = 3.
	//   Default max_tokens output = 2048.
	//   Expected: 3 + 2048 = 2051.
	if units != 2051 {
		t.Errorf("got %d, want 2051 (3 input + 2048 default max)", units)
	}
}

func TestEstimateWorkUnits_ExplicitMaxTokens(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"max_tokens":50}`)
	units, _ := m.EstimateWorkUnits(body, "m")
	// "user" → 1, "hi" → 1, total input = 2. Output = 50. → 52.
	if units != 52 {
		t.Errorf("got %d, want 52", units)
	}
}

func TestEstimateWorkUnits_MultiPartContent(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	body := []byte(`{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"hello world foo"}]}],"max_tokens":10}`)
	units, _ := m.EstimateWorkUnits(body, "m")
	// "user" → 1; "hello world foo " (with trailing space) → 3 words; input = 4. Output = 10. → 14.
	if units != 14 {
		t.Errorf("got %d, want 14", units)
	}
}

func TestServe_NonStreamingHappyPath(t *testing.T) {
	backend := backendhttp.NewFake()
	backend.JSONBody = []byte(`{"id":"cmpl-1","choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}}`)

	m := newModule(backend)
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hello"}]}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/chat/completions", nil)
	actual, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if actual != 8 {
		t.Errorf("actual units: got %d, want 8 (from usage.total_tokens)", actual)
	}
	if backend.LastJSONURL != "http://backend.local:9000/v1/chat/completions" {
		t.Errorf("backend URL: got %q", backend.LastJSONURL)
	}
	if !jsonEq(backend.LastJSONBody, body) {
		t.Errorf("backend body not forwarded verbatim:\n  got:  %s\n  want: %s", backend.LastJSONBody, body)
	}
	if rec.Code != 200 {
		t.Errorf("status: got %d", rec.Code)
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, `"total_tokens":8`) {
		t.Errorf("response missing usage: %s", respBody)
	}
}

func TestServe_NonStreamingNoUsageFallsBackToZero(t *testing.T) {
	backend := backendhttp.NewFake()
	backend.JSONBody = []byte(`{"id":"cmpl-1","choices":[{"message":{"content":"hi"}}]}`)

	m := newModule(backend)
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hello"}]}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/chat/completions", nil)
	actual, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if actual != 0 {
		t.Errorf("no-usage response should yield 0 actual units (no over-debit); got %d", actual)
	}
}

func TestServe_NonStreamingBackendError(t *testing.T) {
	backend := backendhttp.NewFake()
	backend.JSONErr = io.ErrUnexpectedEOF

	m := newModule(backend)
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/chat/completions", nil)
	actual, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err == nil {
		t.Error("expected error on backend failure")
	}
	if actual != 0 {
		t.Errorf("backend error should yield 0 actual units; got %d", actual)
	}
	if rec.Code != nethttp.StatusBadGateway {
		t.Errorf("status: got %d, want 502", rec.Code)
	}
}

func TestServe_StreamingHappyPath(t *testing.T) {
	backend := backendhttp.NewFake()
	// Five chunks; the 4th carries usage, the 5th is the [DONE] marker.
	backend.StreamChunks = [][]byte{
		[]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n"),
		[]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"),
		[]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n"),
		[]byte("data: {\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":22,\"total_tokens\":25}}\n\n"),
		[]byte("data: [DONE]\n\n"),
	}

	m := newModule(backend)
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/chat/completions", nil)
	actual, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if actual != 25 {
		t.Errorf("actual units: got %d, want 25 (from final usage chunk)", actual)
	}
	if backend.LastStreamURL != "http://backend.local:9000/v1/chat/completions" {
		t.Errorf("backend URL: got %q", backend.LastStreamURL)
	}
	if !jsonEq(backend.LastStreamBody, body) {
		t.Errorf("backend body not forwarded verbatim")
	}
	// Response should contain every chunk + [DONE] concatenated.
	out := rec.Body.String()
	for _, marker := range []string{`"Hel"`, `"lo"`, `" world"`, `"total_tokens":25`, `[DONE]`} {
		if !strings.Contains(out, marker) {
			t.Errorf("response missing %q:\n%s", marker, out)
		}
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type: got %q, want text/event-stream", ct)
	}
}

func TestServe_StreamingNoUsageInChunks(t *testing.T) {
	backend := backendhttp.NewFake()
	backend.StreamChunks = [][]byte{
		[]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"),
		[]byte("data: [DONE]\n\n"),
	}

	m := newModule(backend)
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/chat/completions", nil)
	actual, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if actual != 0 {
		t.Errorf("no-usage stream should yield 0 actual units; got %d", actual)
	}
}

func TestServe_StreamingBackendError(t *testing.T) {
	backend := backendhttp.NewFake()
	backend.StreamErr = io.ErrUnexpectedEOF

	m := newModule(backend)
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/chat/completions", nil)
	_, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err == nil {
		t.Error("expected error on backend stream failure")
	}
	if rec.Code != nethttp.StatusBadGateway {
		t.Errorf("status: got %d, want 502", rec.Code)
	}
}

func TestCapabilityAndPath(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	if m.Capability() != Capability {
		t.Error("capability drift")
	}
	if m.HTTPMethod() != nethttp.MethodPost {
		t.Errorf("method: got %q", m.HTTPMethod())
	}
	if m.HTTPPath() != HTTPPath {
		t.Errorf("path: got %q", m.HTTPPath())
	}
}

// jsonEq compares two JSON byte slices for semantic equivalence. Used
// to assert body pass-through even if whitespace differs.
func jsonEq(a, b []byte) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	am, _ := json.Marshal(av)
	bm, _ := json.Marshal(bv)
	return string(am) == string(bm)
}
