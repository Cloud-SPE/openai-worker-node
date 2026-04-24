package backendhttp

import (
	"bytes"
	"context"
	"io"
	"sync"
)

// Fake is an in-memory Client for tests. Programmable response shapes
// for both DoJSON (buffered) and DoStream (SSE chunks). Records the
// last request body seen so tests can assert the worker passed the
// bridge's bytes through verbatim.
//
// Fake is concurrency-safe.
type Fake struct {
	mu sync.Mutex

	// For DoJSON:
	JSONStatus int
	JSONBody   []byte
	JSONErr    error

	// For DoStream: StreamChunks is written one-per-chunk (each chunk
	// already includes the `data: ...\n\n` framing, or the final
	// `data: [DONE]\n\n` marker). Kept raw so tests can insert exactly
	// what they want the module to see.
	StreamStatus int
	StreamChunks [][]byte
	StreamErr    error

	// Observations recorded on each call.
	LastJSONURL    string
	LastJSONBody   []byte
	LastStreamURL  string
	LastStreamBody []byte
	JSONCalls      int
	StreamCalls    int
}

func NewFake() *Fake {
	return &Fake{
		JSONStatus:   200,
		StreamStatus: 200,
	}
}

func (f *Fake) DoJSON(_ context.Context, url string, body []byte) (int, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.JSONCalls++
	f.LastJSONURL = url
	f.LastJSONBody = append([]byte(nil), body...)
	if f.JSONErr != nil {
		return 0, nil, f.JSONErr
	}
	return f.JSONStatus, append([]byte(nil), f.JSONBody...), nil
}

func (f *Fake) DoStream(_ context.Context, url string, body []byte) (int, io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.StreamCalls++
	f.LastStreamURL = url
	f.LastStreamBody = append([]byte(nil), body...)
	if f.StreamErr != nil {
		return 0, nil, f.StreamErr
	}
	buf := bytes.NewBuffer(nil)
	for _, c := range f.StreamChunks {
		buf.Write(c)
	}
	return f.StreamStatus, io.NopCloser(buf), nil
}
