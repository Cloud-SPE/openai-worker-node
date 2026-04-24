package multipartutil

import (
	"bytes"
	"mime/multipart"
	"strings"
	"testing"
)

// buildMultipart writes a valid multipart body and returns (body,
// boundary). Used across tests so we don't hand-roll framing.
func buildMultipart(t *testing.T, fields map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return buf.Bytes(), w.Boundary()
}

func TestBoundaryFromBody_HappyPath(t *testing.T) {
	body, boundary := buildMultipart(t, map[string]string{"model": "m"})
	got, err := BoundaryFromBody(body)
	if err != nil {
		t.Fatalf("BoundaryFromBody: %v", err)
	}
	if got != boundary {
		t.Errorf("boundary: got %q, want %q", got, boundary)
	}
}

func TestBoundaryFromBody_TooShort(t *testing.T) {
	if _, err := BoundaryFromBody([]byte("ab")); err == nil {
		t.Error("expected error on tiny body")
	}
}

func TestBoundaryFromBody_NoLineTerminator(t *testing.T) {
	body := bytes.Repeat([]byte("--"), 200) // no \n
	if _, err := BoundaryFromBody(body); err == nil {
		t.Error("expected error when first line never terminates")
	}
}

func TestBoundaryFromBody_MissingDashDash(t *testing.T) {
	body := []byte("plain-text\r\nstuff")
	if _, err := BoundaryFromBody(body); err == nil {
		t.Error("expected error when first line lacks `--` prefix")
	}
}

func TestBoundaryFromBody_EmptyBoundary(t *testing.T) {
	body := []byte("--\r\nstuff")
	if _, err := BoundaryFromBody(body); err == nil {
		t.Error("expected error on empty boundary")
	}
}

func TestFormField_HappyPath(t *testing.T) {
	body, _ := buildMultipart(t, map[string]string{
		"model":  "whisper-large-v3",
		"prompt": "ignore-this",
	})
	val, ok, err := FormField(body, "model")
	if err != nil {
		t.Fatalf("FormField: %v", err)
	}
	if !ok {
		t.Fatal("field should be present")
	}
	if val != "whisper-large-v3" {
		t.Errorf("value: got %q", val)
	}
}

func TestFormField_Missing(t *testing.T) {
	body, _ := buildMultipart(t, map[string]string{"prompt": "x"})
	_, ok, err := FormField(body, "model")
	if err != nil {
		t.Fatalf("FormField: %v", err)
	}
	if ok {
		t.Error("field should be absent")
	}
}

func TestFormField_WhitespaceTrimmed(t *testing.T) {
	body, _ := buildMultipart(t, map[string]string{"model": "  m\n"})
	val, ok, _ := FormField(body, "model")
	if !ok || val != "m" {
		t.Errorf("expected trimmed value 'm', got %q (ok=%v)", val, ok)
	}
}

func TestFormField_MalformedBody(t *testing.T) {
	if _, _, err := FormField([]byte("not-multipart"), "model"); err == nil {
		t.Error("expected error on non-multipart body")
	}
}

func TestFormField_MultipleParts(t *testing.T) {
	// Manually write a body with two text fields, to confirm we skip
	// non-matching parts rather than stopping at the first.
	body, _ := buildMultipart(t, map[string]string{
		"language": "en",
		"model":    "whisper-1",
		"prompt":   "hello",
	})
	val, ok, _ := FormField(body, "model")
	if !ok || val != "whisper-1" {
		t.Errorf("model lookup with multiple parts: got (%q, %v)", val, ok)
	}
	// Also confirm non-first fields resolve.
	val, ok, _ = FormField(body, "prompt")
	if !ok || val != "hello" {
		t.Errorf("prompt lookup: got (%q, %v)", val, ok)
	}
}

// TestBoundaryFromBody_LineFeedOnly — some producers emit \n (not \r\n)
// on the first line. RFC 2046 allows either; we should handle both.
func TestBoundaryFromBody_LineFeedOnly(t *testing.T) {
	body := []byte("--some-boundary-value\nrest-of-body")
	got, err := BoundaryFromBody(body)
	if err != nil {
		t.Fatalf("BoundaryFromBody: %v", err)
	}
	if got != "some-boundary-value" {
		t.Errorf("boundary: got %q", got)
	}
	if strings.Contains(got, "\r") {
		t.Errorf("stray \\r in boundary: %q", got)
	}
}
