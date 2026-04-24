// Package chat_completions implements the openai:/v1/chat/completions
// capability. One module handles every chat model the operator
// configured; per-model backend routing is resolved by the middleware
// before Serve is called.
//
// The module forwards the bridge's request body verbatim to the
// backend inference server (which is already OpenAI-compatible). It
// never rewrites the prompt, messages, tool definitions, or any other
// field — doing so would make the worker a black-box transformer
// where bridge debugging becomes impossible.
//
// # Metering
//
// EstimateWorkUnits counts tokens in the flattened message stream
// plus the requested max_tokens, then applies a conservative fudge so
// the upfront debit doesn't under-shoot. Actual work units come from
// the backend's `usage.total_tokens` field — exposed in both the
// non-streaming response and the final streaming chunk. Modules
// running against backends that don't emit usage (unusual) fall back
// to the estimate.
//
// # Streaming
//
// The module auto-detects `stream: true` on the request and takes
// the SSE path. Chunks are piped to the client byte-for-byte; the
// final chunk containing `usage` is parsed to derive actual tokens
// for reconciliation.
package chat_completions
