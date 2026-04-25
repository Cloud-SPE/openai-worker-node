package images_edits

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"strconv"
	"strings"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/metrics"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules/multipartutil"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

const (
	Capability = types.CapabilityID("openai:/v1/images/edits")
	HTTPPath   = "/v1/images/edits"
)

// Defaults mirror images_generations — same metering formula, same
// work unit, just sourced from a multipart body instead of JSON.
const (
	defaultN     = 1
	defaultSteps = 30
	defaultSize  = "1024x1024"
)

// Module adapts openai:/v1/images/edits.
type Module struct {
	backend  backendhttp.Client
	recorder metrics.Recorder
}

func New(backend backendhttp.Client) *Module {
	return &Module{backend: backend}
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
func (m *Module) Unit() string                   { return metrics.UnitImageStepMegapixel }

func (m *Module) backendFor(model types.ModelID) backendhttp.Client {
	return backendhttp.WithMetrics(m.backend, m.recorder, string(Capability), string(model))
}

// ExtractModel reads the `model` form field out of the multipart body.
func (m *Module) ExtractModel(body []byte) (types.ModelID, error) {
	val, ok, err := multipartutil.FormField(body, "model")
	if err != nil {
		return "", fmt.Errorf("images_edits: parse multipart: %w", err)
	}
	if !ok || val == "" {
		return "", errors.New("images_edits: multipart body missing `model` field")
	}
	return types.ModelID(val), nil
}

// EstimateWorkUnits meters `ceil((n × steps × W × H) / 1_000_000)`.
// N and size come from the form (when present); fallbacks mirror
// OpenAI's documented defaults. Missing or malformed values get the
// default — the backend will reject bad values before we see them.
func (m *Module) EstimateWorkUnits(body []byte, _ types.ModelID) (int64, error) {
	n := defaultN
	if raw, ok, _ := multipartutil.FormField(body, "n"); ok {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			n = v
		}
	}
	steps := defaultSteps
	if raw, ok, _ := multipartutil.FormField(body, "steps"); ok {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			steps = v
		}
	}
	size := defaultSize
	if raw, ok, _ := multipartutil.FormField(body, "size"); ok && raw != "" && raw != "auto" {
		size = raw
	}
	w, h, err := parseSize(size)
	if err != nil {
		return 0, fmt.Errorf("images_edits: %w", err)
	}
	pixels := int64(n) * int64(steps) * int64(w) * int64(h)
	return (pixels + 999_999) / 1_000_000, nil
}

// Serve forwards the raw multipart body to the backend, preserving
// the caller's Content-Type (including boundary) via DoRaw. The
// buffered JSON response is written back; no reconciliation (image
// backends don't emit usage blocks).
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
		return 0, errors.New("images_edits: Content-Type is not multipart/form-data")
	}
	targetURL := strings.TrimRight(backendURL, "/") + HTTPPath
	status, respBody, err := m.backendFor(model).DoRaw(ctx, targetURL, contentType, body)
	if err != nil {
		nethttp.Error(w, "backend error", nethttp.StatusBadGateway)
		return 0, fmt.Errorf("images_edits: backend DoRaw: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(respBody)
	return 0, nil
}

func parseSize(size string) (int, int, error) {
	parts := strings.Split(strings.ToLower(size), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("size %q: want WxH (e.g. 1024x1024)", size)
	}
	ww, werr := strconv.Atoi(strings.TrimSpace(parts[0]))
	hh, herr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if werr != nil || herr != nil || ww <= 0 || hh <= 0 {
		return 0, 0, fmt.Errorf("size %q: width and height must be positive integers", size)
	}
	return ww, hh, nil
}
