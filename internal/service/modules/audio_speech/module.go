package audio_speech

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"unicode/utf8"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

const (
	Capability = types.CapabilityID("openai:/v1/audio/speech")
	HTTPPath   = "/v1/audio/speech"
)

// request captures the two fields we need: model (for routing) and
// input (for metering). Voice, response_format, speed, etc. ride
// through to the backend verbatim.
type request struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// Module implements the audio_speech capability. Stateless.
type Module struct {
	backend backendhttp.Client
}

// New wires the module against the shared backend HTTP provider. TTS
// metering is dimensional (character count of input) — no tokenizer
// needed.
func New(backend backendhttp.Client) *Module {
	return &Module{backend: backend}
}

var _ modules.Module = (*Module)(nil)

func (m *Module) Capability() types.CapabilityID { return Capability }
func (m *Module) HTTPMethod() string             { return nethttp.MethodPost }
func (m *Module) HTTPPath() string               { return HTTPPath }

func (m *Module) ExtractModel(body []byte) (types.ModelID, error) {
	var r request
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("audio_speech: parse request: %w", err)
	}
	if r.Model == "" {
		return "", errors.New("audio_speech: request is missing `model`")
	}
	return types.ModelID(r.Model), nil
}

// EstimateWorkUnits returns the count of runes in `input`. UTF-8 rune
// count is the natural unit for TTS billing — matches how OpenAI's
// own API priced TTS historically. An empty input counts as 0 (the
// backend will probably reject it anyway).
func (m *Module) EstimateWorkUnits(body []byte, _ types.ModelID) (int64, error) {
	var r request
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, fmt.Errorf("audio_speech: parse request: %w", err)
	}
	return int64(utf8.RuneCountInString(r.Input)), nil
}

// Serve forwards the request to the backend and streams audio bytes
// back. Uses DoStream so the caller can begin playback before the
// full audio file is rendered; content-type and headers come from the
// backend verbatim.
//
// Returns 0 actual units — metering is deterministic from the
// request, so no reconciliation is needed.
func (m *Module) Serve(
	ctx context.Context,
	w nethttp.ResponseWriter,
	_ *nethttp.Request,
	body []byte,
	_ types.ModelID,
	backendURL string,
) (int64, error) {
	targetURL := strings.TrimRight(backendURL, "/") + HTTPPath
	status, backendHeaders, stream, err := m.backend.DoStream(ctx, targetURL, body)
	if err != nil {
		nethttp.Error(w, "backend error", nethttp.StatusBadGateway)
		return 0, fmt.Errorf("audio_speech: backend DoStream: %w", err)
	}
	defer stream.Close()

	// Relay the backend's Content-Type if present (handles
	// audio/mpeg vs audio/wav vs audio/opus vs audio/flac across
	// different TTS backends). Fall back to audio/mpeg — OpenAI's
	// own default — when the backend doesn't set one.
	contentType := backendHeaders.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if _, err := io.Copy(w, stream); err != nil {
		// Client disconnect or backend hangup mid-stream; logged by
		// middleware, nothing to write.
		return 0, fmt.Errorf("audio_speech: pipe stream: %w", err)
	}
	return 0, nil
}
