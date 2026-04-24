package images_edits

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
)

// buildBody assembles a valid multipart body with the given text
// fields and a tiny fake "image" file part. Returns (body,
// contentType).
func buildBody(t *testing.T, fields map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField %q: %v", k, err)
		}
	}
	fw, err := w.CreateFormFile("image", "test.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write([]byte("\x89PNG\r\n\x1a\nfakebytes"))
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func newModule() (*Module, *backendhttp.Fake) {
	fake := backendhttp.NewFake()
	return New(fake), fake
}

func TestExtractModel(t *testing.T) {
	m, _ := newModule()
	body, _ := buildBody(t, map[string]string{"model": "sdxl-edit", "prompt": "red sky"})
	model, err := m.ExtractModel(body)
	if err != nil {
		t.Fatalf("ExtractModel: %v", err)
	}
	if model != "sdxl-edit" {
		t.Errorf("got %q", model)
	}
}

func TestExtractModel_Missing(t *testing.T) {
	m, _ := newModule()
	body, _ := buildBody(t, map[string]string{"prompt": "x"})
	if _, err := m.ExtractModel(body); err == nil {
		t.Error("expected error when model field absent")
	}
}

func TestExtractModel_MalformedBody(t *testing.T) {
	m, _ := newModule()
	if _, err := m.ExtractModel([]byte("not-multipart-at-all")); err == nil {
		t.Error("expected error on non-multipart body")
	}
}

func TestEstimateWorkUnits_Defaults(t *testing.T) {
	m, _ := newModule()
	body, _ := buildBody(t, map[string]string{"model": "m", "prompt": "x"})
	units, err := m.EstimateWorkUnits(body, "m")
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	// n=1, steps=30, 1024×1024 → ceil(31.45) = 32
	if units != 32 {
		t.Errorf("got %d, want 32", units)
	}
}

func TestEstimateWorkUnits_ExplicitFields(t *testing.T) {
	m, _ := newModule()
	body, _ := buildBody(t, map[string]string{
		"model":  "m",
		"prompt": "x",
		"n":      "2",
		"steps":  "40",
		"size":   "512x512",
	})
	units, _ := m.EstimateWorkUnits(body, "m")
	// 2 × 40 × 262_144 = 20,971,520 → ceil/1e6 = 21
	if units != 21 {
		t.Errorf("got %d, want 21", units)
	}
}

func TestEstimateWorkUnits_BadSize(t *testing.T) {
	m, _ := newModule()
	body, _ := buildBody(t, map[string]string{"model": "m", "size": "foo"})
	if _, err := m.EstimateWorkUnits(body, "m"); err == nil {
		t.Error("expected error on malformed size")
	}
}

func TestEstimateWorkUnits_NegativeN(t *testing.T) {
	m, _ := newModule()
	body, _ := buildBody(t, map[string]string{"model": "m", "n": "-3"})
	units, _ := m.EstimateWorkUnits(body, "m")
	if units != 32 {
		t.Errorf("negative n should fall back to default; got %d", units)
	}
}

func TestServe_HappyPath(t *testing.T) {
	m, backend := newModule()
	backend.JSONBody = []byte(`{"created":1,"data":[{"url":"https://cdn.example/edit.png"}]}`)
	body, contentType := buildBody(t, map[string]string{"model": "sdxl-edit", "prompt": "x"})

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/images/edits", nil)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	actual, err := m.Serve(context.Background(), rec, req, body, "sdxl-edit", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if actual != 0 {
		t.Errorf("image backends don't emit usage; actual should be 0, got %d", actual)
	}
	if backend.RawCalls != 1 {
		t.Errorf("DoRaw calls: got %d, want 1", backend.RawCalls)
	}
	if backend.LastRawContentType != contentType {
		t.Errorf("Content-Type not preserved: got %q, want %q", backend.LastRawContentType, contentType)
	}
	if !strings.Contains(rec.Body.String(), "edit.png") {
		t.Errorf("response not passed through: %s", rec.Body.String())
	}
}

func TestServe_RejectsNonMultipart(t *testing.T) {
	m, backend := newModule()
	body, _ := buildBody(t, map[string]string{"model": "m"})

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/images/edits", nil)
	req.Header.Set("Content-Type", "application/json") // wrong type
	rec := httptest.NewRecorder()

	_, err := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if err == nil {
		t.Error("expected error when Content-Type isn't multipart")
	}
	if rec.Code != nethttp.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
	if backend.RawCalls != 0 {
		t.Errorf("backend should not be called with wrong Content-Type")
	}
}

func TestServe_BackendError(t *testing.T) {
	m, backend := newModule()
	backend.JSONErr = io.ErrUnexpectedEOF
	body, contentType := buildBody(t, map[string]string{"model": "m", "prompt": "x"})

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/images/edits", nil)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

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
