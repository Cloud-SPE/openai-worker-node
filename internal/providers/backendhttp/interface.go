package backendhttp

import (
	"context"
	"io"
	"net/http"
)

// Client is the module-facing surface for an HTTP inference backend.
// The worker's runtime/http layer never calls these directly — only
// capability modules do, inside Serve.
type Client interface {
	// DoJSON posts body to url with Content-Type: application/json
	// and returns the buffered response body along with its status
	// code. Used by non-streaming JSON capabilities (chat when
	// stream=false, embeddings, image generation).
	DoJSON(ctx context.Context, url string, body []byte) (status int, respBody []byte, err error)

	// DoRaw posts body with an operator-supplied Content-Type and
	// returns the buffered response. Used by capabilities whose
	// request shape isn't JSON — notably multipart/form-data for
	// /images/edits and /audio/transcriptions, where the Content-Type
	// must preserve the caller's multipart boundary so the backend
	// can parse the body.
	DoRaw(ctx context.Context, url, contentType string, body []byte) (status int, respBody []byte, err error)

	// DoStream posts body with Content-Type: application/json and
	// Accept: text/event-stream, returning the response status, the
	// backend's response headers, and a reader over the raw response
	// body. Callers are responsible for closing the reader.
	//
	// Response headers let the caller relay the backend's actual
	// Content-Type (audio/mpeg vs audio/wav vs text/event-stream)
	// rather than hardcoding. `headers` is non-nil on success; may
	// be nil on error.
	DoStream(ctx context.Context, url string, body []byte) (status int, headers http.Header, stream io.ReadCloser, err error)
}
