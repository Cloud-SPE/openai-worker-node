package payeedaemon

import "context"

// Client is the small surface the worker-node needs from the
// livepeer-payment-daemon. Four methods cover the full lifecycle:
//
//   - ListCapabilities at startup for the worker/daemon catalog
//     cross-check.
//   - ProcessPayment + DebitBalance on every paid request.
//   - Close on shutdown.
//
// Implementations are expected to be safe for concurrent use —
// middleware calls ProcessPayment and DebitBalance from many goroutines
// against the same Client.
type Client interface {
	// ListCapabilities returns the daemon's full configured catalog.
	// Called once at worker startup; the worker fails closed if its
	// own shared-YAML parse doesn't byte-match this response.
	ListCapabilities(ctx context.Context) (ListCapabilitiesResult, error)

	// GetQuote returns the daemon's TicketParams + per-model prices
	// for a (sender, capability) pair. The worker's /quote and
	// /quotes HTTP handlers proxy this call through to the bridge so
	// the bridge can refresh its quote cache. NotFound is expected
	// when the operator hasn't configured `capability`.
	GetQuote(ctx context.Context, sender []byte, capability string) (GetQuoteResult, error)

	// ProcessPayment validates a payment blob and credits the sender's
	// balance. The workID identifies the session the credit posts to;
	// typically the worker derives it from the payment (e.g. the
	// RecipientRandHash hex) so a sender + capability pair collapses
	// to a single long-lived session.
	ProcessPayment(ctx context.Context, paymentBytes []byte, workID string) (ProcessPaymentResult, error)

	// DebitBalance subtracts workUnits from the (sender, workID)
	// balance. Returns the new balance; a negative balance means the
	// caller over-debited and must refuse to serve further work on
	// this session.
	DebitBalance(ctx context.Context, sender []byte, workID string, workUnits int64) (DebitBalanceResult, error)

	// Close releases the underlying transport. Calling any other
	// method after Close is undefined.
	Close() error
}
