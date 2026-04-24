// Package multipartutil is a small shared helper for multipart-bodied
// capability modules (images/edits, audio/transcriptions). It extracts
// the multipart boundary directly from the body — the first line of a
// well-formed multipart request is `--<boundary>\r\n` — so modules
// don't need the Content-Type header threaded through the Module
// interface.
//
// Scope is deliberately narrow: boundary extraction + form-field value
// lookup. Modules that need richer parsing (iterate files, read file
// metadata) build on top of stdlib `mime/multipart` directly.
package multipartutil
