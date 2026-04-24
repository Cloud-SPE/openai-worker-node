package audio_transcriptions

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

// buildBody writes a multipart body with text fields + a fake audio
// file. Returns (body, contentType).
func buildBody(t *testing.T, fields map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	fw, err := w.CreateFormFile("file", "audio.mp3")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write([]byte("\xff\xfb\x90\x00fake-audio"))
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
	body, _ := buildBody(t, map[string]string{"model": "whisper-large-v3", "language": "en"})
	model, err := m.ExtractModel(body)
	if err != nil {
		t.Fatalf("ExtractModel: %v", err)
	}
	if model != "whisper-large-v3" {
		t.Errorf("got %q", model)
	}
}

func TestExtractModel_Missing(t *testing.T) {
	m, _ := newModule()
	body, _ := buildBody(t, map[string]string{"language": "en"})
	if _, err := m.ExtractModel(body); err == nil {
		t.Error("expected error when model absent")
	}
}

func TestEstimateWorkUnits_DefaultMax(t *testing.T) {
	m, _ := newModule()
	body, _ := buildBody(t, map[string]string{"model": "m"})
	units, err := m.EstimateWorkUnits(body, "m")
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if units != int64(DefaultMaxAudioSeconds) {
		t.Errorf("got %d, want %d", units, DefaultMaxAudioSeconds)
	}
}

func TestEstimateWorkUnits_CustomMax(t *testing.T) {
	m, _ := newModule()
	m.MaxAudioSecondsCeil = 600
	body, _ := buildBody(t, map[string]string{"model": "m"})
	units, _ := m.EstimateWorkUnits(body, "m")
	if units != 600 {
		t.Errorf("got %d, want 600", units)
	}
}

func TestEstimateWorkUnits_NonPositiveCeilingFallsBackToDefault(t *testing.T) {
	m, _ := newModule()
	m.MaxAudioSecondsCeil = -1
	body, _ := buildBody(t, map[string]string{"model": "m"})
	units, _ := m.EstimateWorkUnits(body, "m")
	if units != int64(DefaultMaxAudioSeconds) {
		t.Errorf("got %d, want %d (fall back when ceil is non-positive)", units, DefaultMaxAudioSeconds)
	}
}

func TestServe_NoDurationInResponse(t *testing.T) {
	m, backend := newModule()
	backend.JSONBody = []byte(`{"text":"hello world"}`)
	body, contentType := buildBody(t, map[string]string{"model": "whisper-large-v3"})

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/audio/transcriptions", nil)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	actual, err := m.Serve(context.Background(), rec, req, body, "whisper-large-v3", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if actual != 0 {
		t.Errorf("no duration → 0 (estimate stands); got %d", actual)
	}
	if backend.LastRawContentType != contentType {
		t.Errorf("Content-Type not preserved: got %q", backend.LastRawContentType)
	}
	if !strings.Contains(rec.Body.String(), "hello world") {
		t.Errorf("response not passed through: %s", rec.Body.String())
	}
}

func TestServe_DurationReconciles(t *testing.T) {
	m, backend := newModule()
	// verbose_json response shape carries duration (seconds).
	backend.JSONBody = []byte(`{"task":"transcribe","language":"en","duration":12.34,"text":"..."}`)
	body, contentType := buildBody(t, map[string]string{"model": "whisper-large-v3", "response_format": "verbose_json"})

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/audio/transcriptions", nil)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	actual, err := m.Serve(context.Background(), rec, req, body, "whisper-large-v3", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	// 12.34 → ceil = 13 seconds.
	if actual != 13 {
		t.Errorf("duration reconcile: got %d, want 13 (ceil(12.34))", actual)
	}
}

func TestServe_NegativeDurationIgnored(t *testing.T) {
	m, backend := newModule()
	backend.JSONBody = []byte(`{"duration":-5,"text":"garbage"}`)
	body, contentType := buildBody(t, map[string]string{"model": "m"})

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/audio/transcriptions", nil)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	actual, _ := m.Serve(context.Background(), rec, req, body, "m", "http://backend.local:9000")
	if actual != 0 {
		t.Errorf("negative duration must be ignored; got %d", actual)
	}
}

func TestServe_BackendError(t *testing.T) {
	m, backend := newModule()
	backend.JSONErr = io.ErrUnexpectedEOF
	body, contentType := buildBody(t, map[string]string{"model": "m"})

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/audio/transcriptions", nil)
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

func TestServe_RejectsNonMultipart(t *testing.T) {
	m, _ := newModule()
	body, _ := buildBody(t, map[string]string{"model": "m"})

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/audio/transcriptions", nil)
	req.Header.Set("Content-Type", "application/json") // wrong type
	rec := httptest.NewRecorder()

	if _, err := m.Serve(context.Background(), rec, req, body, "m", "http://b"); err == nil {
		t.Error("expected error for wrong Content-Type")
	}
	if rec.Code != nethttp.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
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
