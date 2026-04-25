package audio_transcriptions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	nethttp "net/http"
	"strings"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/metrics"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules/multipartutil"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

const (
	Capability = types.CapabilityID("openai:/v1/audio/transcriptions")
	HTTPPath   = "/v1/audio/transcriptions"
)

// DefaultMaxAudioSeconds is the upfront reservation used when the
// worker cannot yet observe the actual audio duration. Chosen to
// cover a full hour of audio — long enough for podcast-scale use
// cases, short enough that an abusive request can be caught by
// tier-level balance limits.
const DefaultMaxAudioSeconds = 3600

// Module adapts openai:/v1/audio/transcriptions.
type Module struct {
	backend             backendhttp.Client
	recorder            metrics.Recorder
	MaxAudioSecondsCeil int64
}

func New(backend backendhttp.Client) *Module {
	return &Module{backend: backend, MaxAudioSecondsCeil: DefaultMaxAudioSeconds}
}

// WithRecorder injects the metrics recorder used to wrap the backend
// client per-(capability, model) inside Serve. Optional.
func (m *Module) WithRecorder(rec metrics.Recorder) *Module {
	m.recorder = rec
	return m
}

var _ modules.Module = (*Module)(nil)

func (m *Module) Capability() types.CapabilityID { return Capability }
func (m *Module) HTTPMethod() string             { return nethttp.MethodPost }
func (m *Module) HTTPPath() string               { return HTTPPath }
func (m *Module) Unit() string                   { return metrics.UnitAudioSecond }

func (m *Module) backendFor(model types.ModelID) backendhttp.Client {
	return backendhttp.WithMetrics(m.backend, m.recorder, string(Capability), string(model))
}

func (m *Module) ExtractModel(body []byte) (types.ModelID, error) {
	val, ok, err := multipartutil.FormField(body, "model")
	if err != nil {
		return "", fmt.Errorf("audio_transcriptions: parse multipart: %w", err)
	}
	if !ok || val == "" {
		return "", errors.New("audio_transcriptions: multipart body missing `model` field")
	}
	return types.ModelID(val), nil
}

// EstimateWorkUnits returns the configured tier-max reservation. The
// actual count comes from the backend's response `duration` field
// when response_format is `verbose_json` — the middleware reconciles
// via a second DebitBalance for the delta (over-debit only).
//
// Body is accepted but not inspected: audio duration cannot be
// determined without decoding the file, which we avoid. Operators who
// want tighter metering configure a smaller MaxAudioSecondsCeil.
func (m *Module) EstimateWorkUnits(body []byte, _ types.ModelID) (int64, error) {
	if m.MaxAudioSecondsCeil <= 0 {
		return DefaultMaxAudioSeconds, nil
	}
	return m.MaxAudioSecondsCeil, nil
}

// Serve forwards the multipart body to the backend via DoRaw
// (preserving the caller's Content-Type + boundary). Reconciliation:
// parse the response JSON for `duration`; if present and sensible,
// round up to the nearest second and return — the middleware uses
// the value to reconcile over the upfront estimate. When duration
// is absent (response_format: text|srt|vtt, or a plain `{"text":...}`
// response), return 0 so the estimate stands as the final charge.
func (m *Module) Serve(
	ctx context.Context,
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	body []byte,
	model types.ModelID,
	backendURL string,
) (int64, error) {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		nethttp.Error(w, "Content-Type must be multipart/form-data", nethttp.StatusBadRequest)
		return 0, errors.New("audio_transcriptions: Content-Type is not multipart/form-data")
	}
	targetURL := strings.TrimRight(backendURL, "/") + HTTPPath
	status, respBody, err := m.backendFor(model).DoRaw(ctx, targetURL, contentType, body)
	if err != nil {
		nethttp.Error(w, "backend error", nethttp.StatusBadGateway)
		return 0, fmt.Errorf("audio_transcriptions: backend DoRaw: %w", err)
	}
	// Forward the backend's Content-Type too — the bridge might
	// receive plain text (response_format=text) or SRT, not JSON.
	// For v1 we don't have DoRaw exposing the backend Content-Type;
	// default to application/json and accept a small
	// inaccuracy for non-JSON formats. Tracked as tech-debt via
	// audio-speech-content-type-passthrough (same root cause).
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(respBody)

	return durationFromResponse(respBody), nil
}

// durationFromResponse reads a `duration` field from the backend's
// response body and returns it rounded up to the nearest second.
// Returns 0 when duration isn't present, negative, or not parseable
// — the middleware interprets 0 as "no reconciliation needed."
func durationFromResponse(respBody []byte) int64 {
	var parsed struct {
		Duration any `json:"duration"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return 0
	}
	// JSON numbers arrive as float64 through any. Strings allow for
	// backends that encode duration as a string.
	switch v := parsed.Duration.(type) {
	case float64:
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return 0
		}
		return int64(math.Ceil(v))
	case int:
		if v <= 0 {
			return 0
		}
		return int64(v)
	default:
		return 0
	}
}
