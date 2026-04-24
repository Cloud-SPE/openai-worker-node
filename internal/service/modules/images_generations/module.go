package images_generations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	nethttp "net/http"
	"strconv"
	"strings"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

const (
	Capability = types.CapabilityID("openai:/v1/images/generations")
	HTTPPath   = "/v1/images/generations"
)

// Module is the images/generations capability adapter. Stateless; safe
// for concurrent use.
type Module struct {
	backend backendhttp.Client
}

// New wires the module against the shared backend HTTP provider. No
// tokenizer needed — metering is dimensional.
func New(backend backendhttp.Client) *Module {
	return &Module{backend: backend}
}

// Compile-time interface check.
var _ modules.Module = (*Module)(nil)

func (m *Module) Capability() types.CapabilityID { return Capability }
func (m *Module) HTTPMethod() string             { return nethttp.MethodPost }
func (m *Module) HTTPPath() string               { return HTTPPath }

func (m *Module) ExtractModel(body []byte) (types.ModelID, error) {
	var r request
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("images_generations: parse request: %w", err)
	}
	if r.Model == "" {
		return "", errors.New("images_generations: request is missing `model`")
	}
	return types.ModelID(r.Model), nil
}

// EstimateWorkUnits returns `steps × megapixels × n`, rounded up to
// int. Megapixels are computed from the `size` field ("WxH", e.g.
// "1024x1024" → 1.048576 MP). Fields missing default per OpenAI's
// documented values.
//
// The returned value is the CEILING of the real product so operators
// never under-charge; this matches the over-debit-accepted policy.
func (m *Module) EstimateWorkUnits(body []byte, _ types.ModelID) (int64, error) {
	var r request
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, fmt.Errorf("images_generations: parse request: %w", err)
	}
	n := defaultN
	if r.N != nil && *r.N > 0 {
		n = *r.N
	}
	steps := defaultSteps
	if r.Steps != nil && *r.Steps > 0 {
		steps = *r.Steps
	}
	w, h, err := parseSize(r.Size)
	if err != nil {
		return 0, fmt.Errorf("images_generations: %w", err)
	}
	// Pixels → megapixels (1_000_000, not 2^20, for user-facing math).
	// Round up so fractional results don't under-charge.
	pixels := int64(n) * int64(steps) * int64(w) * int64(h)
	units := (pixels + 999_999) / 1_000_000
	return units, nil
}

func (m *Module) Serve(
	ctx context.Context,
	w nethttp.ResponseWriter,
	_ *nethttp.Request,
	body []byte,
	_ types.ModelID,
	backendURL string,
) (int64, error) {
	targetURL := strings.TrimRight(backendURL, "/") + HTTPPath
	status, respBody, err := m.backend.DoJSON(ctx, targetURL, body)
	if err != nil {
		nethttp.Error(w, "backend error", nethttp.StatusBadGateway)
		return 0, fmt.Errorf("images_generations: backend DoJSON: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(respBody)

	// Reconciliation: image backends don't emit usage blocks; actual =
	// the estimate we already debited. Return 0 so the middleware
	// skips the reconcile debit.
	return 0, nil
}

// parseSize turns "WxH" into (w, h). Accepts "auto" as a pass-through
// alias for the default size. Returns a descriptive error on malformed
// input rather than silently returning zero, which would under-charge.
func parseSize(size string) (int, int, error) {
	s := size
	if s == "" || s == "auto" {
		s = defaultSize
	}
	parts := strings.Split(strings.ToLower(s), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("size %q: want WxH (e.g. 1024x1024)", size)
	}
	w, werr := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, herr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if werr != nil || herr != nil || w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("size %q: width and height must be positive integers", size)
	}
	return w, h, nil
}
