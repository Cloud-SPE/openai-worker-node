// Package images_edits implements the openai:/v1/images/edits
// capability. Request is multipart/form-data: `image` (required),
// optional `mask`, `prompt`, `model`, `n`, `size`. Response is JSON.
//
// Work unit = `image_step_megapixel`. Unlike images_generations the
// request doesn't carry a dimension field reliably — the output size
// typically matches the input image's dimensions, which would require
// image-header decoding to observe. For v1 we meter at the default
// megapixel count (1024×1024 → ~1 MP) per image, multiplied by n and
// a steps default. The over-debit policy absorbs undercounts when
// users upload larger base images.
//
// Body parsing uses multipartutil.FormField for the small text
// fields; the raw body is forwarded to the backend verbatim so image
// bytes aren't re-serialised.
package images_edits
