// Package images_generations implements the openai:/v1/images/generations
// capability. Request is OpenAI-compatible JSON: model + prompt + n + size
// + response_format. Response is JSON (either URL list or base64 blobs).
//
// Work unit = `image_step_megapixel` — steps × megapixels × n, rounded up.
// Cost is fully deterministic from the request; actual == estimate except
// when the backend clamps n downward (rare; still over-debit-safe).
//
// The sibling /v1/images/edits route is multipart/form-data and requires
// a different body-parser; it lives in its own plan (tracked in
// tech-debt-tracker as `multipart-capability-handling`).
package images_generations
