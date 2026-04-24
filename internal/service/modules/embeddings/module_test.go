package embeddings

import (
	"context"
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
	model, err := m.ExtractModel([]byte(`{"model":"text-embedding-3-small","input":"hi"}`))
	if err != nil {
		t.Fatalf("ExtractModel: %v", err)
	}
	if model != "text-embedding-3-small" {
		t.Errorf("got %q", model)
	}
}

func TestExtractModel_Missing(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	if _, err := m.ExtractModel([]byte(`{"input":"hi"}`)); err == nil {
		t.Error("expected error when model missing")
	}
}

func TestEstimateWorkUnits_StringInput(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	body := []byte(`{"model":"m","input":"hello world foo"}`)
	units, err := m.EstimateWorkUnits(body, "m")
	if err != nil {
		t.Fatalf("EstimateWorkUnits: %v", err)
	}
	// 3 words × 1.0 multiplier = 3.
	if units != 3 {
		t.Errorf("got %d, want 3", units)
	}
}

func TestEstimateWorkUnits_StringArrayInput(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	body := []byte(`{"model":"m","input":["hello world","foo bar baz"]}`)
	units, _ := m.EstimateWorkUnits(body, "m")
	// 2 + 3 = 5.
	if units != 5 {
		t.Errorf("got %d, want 5", units)
	}
}

func TestEstimateWorkUnits_TokenIDArrayInput(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	body := []byte(`{"model":"m","input":[101,102,103,104]}`)
	units, _ := m.EstimateWorkUnits(body, "m")
	// len(ids) = 4.
	if units != 4 {
		t.Errorf("got %d, want 4", units)
	}
}

func TestEstimateWorkUnits_UnknownInputShape(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	body := []byte(`{"model":"m","input":null}`)
	units, _ := m.EstimateWorkUnits(body, "m")
	if units != 0 {
		t.Errorf("unknown shape should estimate 0, got %d", units)
	}
}

func TestEstimateWorkUnits_BadJSON(t *testing.T) {
	m := newModule(backendhttp.NewFake())
	if _, err := m.EstimateWorkUnits([]byte(`not-json`), "m"); err == nil {
		t.Error("expected parse error")
	}
}

func TestServe_HappyPath(t *testing.T) {
	backend := backendhttp.NewFake()
	backend.JSONBody = []byte(`{"object":"list","data":[{"index":0,"embedding":[0.1,0.2]}],"model":"text-embedding-3-small","usage":{"prompt_tokens":6,"total_tokens":6}}`)

	m := newModule(backend)
	body := []byte(`{"model":"text-embedding-3-small","input":"hello world foo"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/embeddings", nil)
	actual, err := m.Serve(context.Background(), rec, req, body, "text-embedding-3-small", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if actual != 6 {
		t.Errorf("actual units: got %d, want 6", actual)
	}
	if backend.LastJSONURL != "http://backend.local:9000/v1/embeddings" {
		t.Errorf("backend URL: got %q", backend.LastJSONURL)
	}
	if !strings.Contains(rec.Body.String(), `"total_tokens":6`) {
		t.Errorf("response missing usage: %s", rec.Body.String())
	}
}

func TestServe_NoUsage(t *testing.T) {
	backend := backendhttp.NewFake()
	backend.JSONBody = []byte(`{"object":"list","data":[{"index":0,"embedding":[]}]}`)

	m := newModule(backend)
	body := []byte(`{"model":"m","input":"hi"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/embeddings", nil)
	actual, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if actual != 0 {
		t.Errorf("no-usage should yield 0; got %d", actual)
	}
}

func TestServe_BackendError(t *testing.T) {
	backend := backendhttp.NewFake()
	backend.JSONErr = io.ErrUnexpectedEOF

	m := newModule(backend)
	body := []byte(`{"model":"m","input":"hi"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/embeddings", nil)
	_, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err == nil {
		t.Error("expected backend error")
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
