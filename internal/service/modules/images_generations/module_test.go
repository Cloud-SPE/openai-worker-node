package images_generations

import (
	"context"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
)

func newModule() (*Module, *backendhttp.Fake) {
	fake := backendhttp.NewFake()
	return New(fake), fake
}

func TestExtractModel(t *testing.T) {
	m, _ := newModule()
	model, err := m.ExtractModel([]byte(`{"model":"sdxl-turbo","prompt":"a cat"}`))
	if err != nil {
		t.Fatalf("ExtractModel: %v", err)
	}
	if model != "sdxl-turbo" {
		t.Errorf("got %q", model)
	}
}

func TestExtractModel_MissingModel(t *testing.T) {
	m, _ := newModule()
	if _, err := m.ExtractModel([]byte(`{"prompt":"x"}`)); err == nil {
		t.Error("expected error when model missing")
	}
}

func TestEstimateWorkUnits_Defaults(t *testing.T) {
	m, _ := newModule()
	body := []byte(`{"model":"m","prompt":"x"}`)
	units, err := m.EstimateWorkUnits(body, "m")
	if err != nil {
		t.Fatalf("EstimateWorkUnits: %v", err)
	}
	// n=1, steps=30, size=1024×1024 → 1 × 30 × 1,048,576 ≈ 31.45 MP
	// → ceil = 32.
	if units != 32 {
		t.Errorf("defaults: got %d, want 32 (n=1 × steps=30 × 1.048576 MP)", units)
	}
}

func TestEstimateWorkUnits_ExplicitAll(t *testing.T) {
	m, _ := newModule()
	body := []byte(`{"model":"m","prompt":"x","n":4,"steps":50,"size":"512x512"}`)
	units, _ := m.EstimateWorkUnits(body, "m")
	// 4 × 50 × 262_144 = 52,428,800 → /1e6 → ceil = 53
	if units != 53 {
		t.Errorf("got %d, want 53", units)
	}
}

func TestEstimateWorkUnits_SizeAuto(t *testing.T) {
	m, _ := newModule()
	body := []byte(`{"model":"m","prompt":"x","size":"auto"}`)
	units, err := m.EstimateWorkUnits(body, "m")
	if err != nil {
		t.Fatalf("auto size should be accepted: %v", err)
	}
	// "auto" → defaultSize (1024×1024) → same as defaults = 32
	if units != 32 {
		t.Errorf("auto: got %d, want 32", units)
	}
}

func TestEstimateWorkUnits_BadSize(t *testing.T) {
	m, _ := newModule()
	body := []byte(`{"model":"m","prompt":"x","size":"foo"}`)
	if _, err := m.EstimateWorkUnits(body, "m"); err == nil {
		t.Error("malformed size should error")
	}
}

func TestEstimateWorkUnits_NegativeN(t *testing.T) {
	m, _ := newModule()
	// Negative n → treat as default (1).
	body := []byte(`{"model":"m","prompt":"x","n":-3}`)
	units, _ := m.EstimateWorkUnits(body, "m")
	if units != 32 {
		t.Errorf("negative n should fall back to 1; got %d", units)
	}
}

func TestServe_HappyPath(t *testing.T) {
	m, backend := newModule()
	backend.JSONBody = []byte(`{"created":1,"data":[{"url":"https://cdn.example/img.png"}]}`)
	body := []byte(`{"model":"m","prompt":"a cat","n":1,"size":"512x512","steps":30}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/images/generations", nil)
	actual, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	// Image backends don't emit usage blocks; actual always 0 (skip
	// reconciliation).
	if actual != 0 {
		t.Errorf("actual units: got %d, want 0 (no reconcile for images)", actual)
	}
	if backend.LastJSONURL != "http://backend.local:9000/v1/images/generations" {
		t.Errorf("URL: got %q", backend.LastJSONURL)
	}
	if !strings.Contains(rec.Body.String(), "img.png") {
		t.Errorf("response body not passed through: %s", rec.Body.String())
	}
}

func TestServe_BackendError(t *testing.T) {
	m, backend := newModule()
	backend.JSONErr = io.ErrUnexpectedEOF
	body := []byte(`{"model":"m","prompt":"x"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/images/generations", nil)
	_, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err == nil {
		t.Error("expected backend error")
	}
	if rec.Code != nethttp.StatusBadGateway {
		t.Errorf("status: got %d, want 502", rec.Code)
	}
}

func TestCapabilityAndPath(t *testing.T) {
	m, _ := newModule()
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
