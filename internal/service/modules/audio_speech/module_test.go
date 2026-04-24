package audio_speech

import (
	"bytes"
	"context"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
)

func newModule() (*Module, *backendhttp.Fake) {
	fake := backendhttp.NewFake()
	return New(fake), fake
}

func TestExtractModel(t *testing.T) {
	m, _ := newModule()
	model, err := m.ExtractModel([]byte(`{"model":"tts-1","input":"hi","voice":"alloy"}`))
	if err != nil {
		t.Fatalf("ExtractModel: %v", err)
	}
	if model != "tts-1" {
		t.Errorf("got %q", model)
	}
}

func TestExtractModel_Missing(t *testing.T) {
	m, _ := newModule()
	if _, err := m.ExtractModel([]byte(`{"input":"x"}`)); err == nil {
		t.Error("expected error when model missing")
	}
}

func TestEstimateWorkUnits_ASCII(t *testing.T) {
	m, _ := newModule()
	body := []byte(`{"model":"tts-1","input":"hello world"}`)
	units, err := m.EstimateWorkUnits(body, "m")
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if units != 11 {
		t.Errorf("got %d, want 11 (`hello world` length)", units)
	}
}

func TestEstimateWorkUnits_Unicode(t *testing.T) {
	m, _ := newModule()
	// Two emoji (4 bytes each in UTF-8, but each is one rune) + "hi"
	body := []byte(`{"model":"tts-1","input":"👋🌍hi"}`)
	units, _ := m.EstimateWorkUnits(body, "m")
	// 👋 + 🌍 + h + i = 4 runes.
	if units != 4 {
		t.Errorf("emoji count: got %d, want 4", units)
	}
}

func TestEstimateWorkUnits_Empty(t *testing.T) {
	m, _ := newModule()
	body := []byte(`{"model":"tts-1","input":""}`)
	units, _ := m.EstimateWorkUnits(body, "m")
	if units != 0 {
		t.Errorf("empty: got %d, want 0", units)
	}
}

func TestServe_HappyPath(t *testing.T) {
	m, backend := newModule()
	// Simulate a short mp3-like byte blob.
	backend.StreamChunks = [][]byte{[]byte("\xff\xfb\x90\x00fake-audio-bytes")}

	body := []byte(`{"model":"tts-1","input":"hi","voice":"alloy"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/audio/speech", nil)
	actual, err := m.Serve(context.Background(), rec, req, body, "tts-1", "http://backend.local:9000")
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if actual != 0 {
		t.Errorf("TTS returns 0 actual units (deterministic); got %d", actual)
	}
	if backend.LastStreamURL != "http://backend.local:9000/v1/audio/speech" {
		t.Errorf("URL: got %q", backend.LastStreamURL)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Errorf("content-type: got %q, want audio/mpeg", ct)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("fake-audio-bytes")) {
		t.Errorf("audio bytes not piped through: %q", rec.Body.String())
	}
}

func TestServe_RelaysBackendContentType(t *testing.T) {
	// When the backend sets its own Content-Type (e.g. audio/wav),
	// the worker relays it verbatim rather than hardcoding audio/mpeg.
	m, backend := newModule()
	backend.StreamChunks = [][]byte{[]byte("RIFF....WAVE-bytes")}
	backend.StreamHeaders = nethttp.Header{}
	backend.StreamHeaders.Set("Content-Type", "audio/wav")

	body := []byte(`{"model":"tts-1","input":"hi","response_format":"wav"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/audio/speech", nil)
	if _, err := m.Serve(context.Background(), rec, req, body, "tts-1", "http://backend.local:9000"); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "audio/wav" {
		t.Errorf("content-type relay: got %q, want audio/wav", ct)
	}
}

func TestServe_FallsBackToAudioMpegOnEmptyHeaders(t *testing.T) {
	m, backend := newModule()
	backend.StreamChunks = [][]byte{[]byte("fake-audio")}
	// No StreamHeaders set — Fake returns an empty http.Header.
	body := []byte(`{"model":"tts-1","input":"hi"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/audio/speech", nil)
	if _, err := m.Serve(context.Background(), rec, req, body, "tts-1", "http://backend.local:9000"); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Errorf("fallback content-type: got %q, want audio/mpeg", ct)
	}
}

func TestServe_BackendError(t *testing.T) {
	m, backend := newModule()
	backend.StreamErr = io.ErrUnexpectedEOF
	body := []byte(`{"model":"tts-1","input":"x"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/audio/speech", nil)
	_, err := m.Serve(context.Background(), rec, req, body, "tts-1", "http://backend.local:9000")
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
