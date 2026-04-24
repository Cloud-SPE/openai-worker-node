package backendhttp

import (
	"context"
	"io"
)

// Client is the module-facing surface for an HTTP inference backend.
// The worker's runtime/http layer never calls these directly — only
// capability modules do, inside Serve.
type Client interface {
	// DoJSON posts body to url and returns the buffered response body
	// along with its status code. Used by non-streaming capabilities
	// (chat when stream=false, embeddings, image generation, TTS when
	// non-streaming, ASR).
	DoJSON(ctx context.Context, url string, body []byte) (status int, respBody []byte, err error)

	// DoStream posts body to url with Accept: text/event-stream and
	// returns a reader over the raw response body. Callers are
	// responsible for closing the reader. status is populated before
	// any body reads happen. Used for streaming chat completions and
	// TTS audio.
	DoStream(ctx context.Context, url string, body []byte) (status int, stream io.ReadCloser, err error)
}
