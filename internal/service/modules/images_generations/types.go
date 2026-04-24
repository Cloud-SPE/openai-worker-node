package images_generations

// request captures the fields the worker needs to route + meter. Size
// is a string like "1024x1024" or "auto"; parsed lazily in the
// estimator. Steps is non-standard OpenAI but supported by SDXL-style
// backends — we read it opportunistically and fall back to a default.
type request struct {
	Model  string `json:"model"`
	N      *int   `json:"n,omitempty"`
	Size   string `json:"size,omitempty"`
	Steps  *int   `json:"steps,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

// defaults applied when the request omits the field. OpenAI's own
// defaults for `dall-e-3` are n=1, size=1024x1024. Steps is backend-
// specific; 30 is a reasonable SDXL default.
const (
	defaultN     = 1
	defaultSteps = 30
	defaultSize  = "1024x1024"
)
