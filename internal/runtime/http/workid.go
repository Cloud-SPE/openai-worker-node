package http

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

// deriveWorkID builds a stable opaque WorkID from the raw payment
// bytes. In the worker's current per-request session model, the only
// properties we need are:
//
//   - deterministic for a given payment blob, so retries hit the same
//     payee-side session identifier
//   - distinct for different blobs, so unrelated requests don't share a
//     balance ledger accidentally
//
// SHA-256 hex satisfies both without coupling the worker to the
// payment proto's internal fields.
func deriveWorkID(paymentBytes []byte) types.WorkID {
	sum := sha256.Sum256(paymentBytes)
	return types.WorkID(hex.EncodeToString(sum[:]))
}
