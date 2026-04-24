package multipartutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
)

// BoundaryFromBody reads the first line of a multipart/form-data body
// and returns the boundary string. Well-formed multipart bodies start
// with `--<boundary>\r\n` (or `\n`); we strip the leading `--` and the
// trailing line terminator.
//
// Returns an error if the body is too short, missing the `--` prefix,
// or lacks a line terminator in the first 1 KiB (larger than any real
// boundary, which RFC 2046 caps at 70 characters).
func BoundaryFromBody(body []byte) (string, error) {
	if len(body) < 3 {
		return "", errors.New("multipartutil: body too short to be multipart")
	}
	// Look at the first 1 KiB; real boundaries fit in <80 bytes.
	window := body
	if len(window) > 1024 {
		window = window[:1024]
	}
	idx := bytes.IndexByte(window, '\n')
	if idx < 0 {
		return "", errors.New("multipartutil: first line has no terminator in first 1 KiB")
	}
	first := window[:idx]
	first = bytes.TrimRight(first, "\r")
	if !bytes.HasPrefix(first, []byte("--")) {
		return "", fmt.Errorf("multipartutil: first line does not begin with `--`: %q", string(first))
	}
	b := string(first[2:])
	if b == "" {
		return "", errors.New("multipartutil: empty boundary")
	}
	return b, nil
}

// FormField returns the value of a named form field from a multipart
// body. Intended for small text fields (model names, language codes);
// values larger than 64 KiB are truncated to that cap to avoid
// unbounded memory use on a malformed request.
//
// Returns (value, true, nil) when found, ("", false, nil) when not
// present, or ("", false, err) if the body isn't parseable at all.
func FormField(body []byte, name string) (string, bool, error) {
	boundary, err := BoundaryFromBody(body)
	if err != nil {
		return "", false, err
	}
	r := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := r.NextPart()
		if errors.Is(err, io.EOF) {
			return "", false, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("multipartutil: read part: %w", err)
		}
		if part.FormName() != name {
			part.Close()
			continue
		}
		limited := io.LimitReader(part, 64<<10)
		buf, rerr := io.ReadAll(limited)
		part.Close()
		if rerr != nil {
			return "", false, fmt.Errorf("multipartutil: read field %q: %w", name, rerr)
		}
		return strings.TrimSpace(string(buf)), true, nil
	}
}
