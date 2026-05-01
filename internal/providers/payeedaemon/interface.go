package payeedaemon

import (
	"context"
	"time"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/metrics"
)

// Client is the small surface the worker-node needs from the
// livepeer-payment-daemon.
//
//   - ListCapabilities at startup for the worker/daemon catalog
//     cross-check.
//   - GetQuote is retained for compatibility with the daemon surface
//     even though the v3.0.1 worker no longer exposes quote endpoints.
//   - ProcessPayment + DebitBalance on every paid request.
//   - Close on shutdown.
//
// Implementations are expected to be safe for concurrent use —
// middleware calls ProcessPayment and DebitBalance from many goroutines
// against the same Client.
type Client interface {
	// ListCapabilities returns the daemon's full configured catalog.
	// Called once at worker startup; the worker fails closed if its
	// own worker.yaml parse doesn't byte-match this response.
	ListCapabilities(ctx context.Context) (ListCapabilitiesResult, error)

	// GetQuote returns the daemon's TicketParams + per-model prices
	// for a (sender, capability) pair. The v3.0.1 worker no longer
	// exposes quote HTTP endpoints, but the provider surface keeps this
	// method so it remains wire-compatible with the daemon contract.
	GetQuote(ctx context.Context, sender []byte, capability string) (GetQuoteResult, error)

	// GetTicketParams returns canonical payee-issued ticket params for
	// an exact sender / recipient / face-value tuple. The worker uses
	// this only as a thin HTTP proxy for the gateway-side payment flow.
	GetTicketParams(ctx context.Context, req GetTicketParamsRequest) (TicketParams, error)

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

// WithMetrics wraps a Client so every RPC also emits the corresponding
// daemon-RPC metrics. The wrapper is thin and allocation-free per-call —
// the recorder methods are inlined by the compiler when the recorder is
// the Noop type. ObserveDaemonRPC writes to both the coarse and fast
// histograms internally, so a single Observe call covers both.
func WithMetrics(c Client, rec metrics.Recorder) Client {
	if rec == nil {
		return c
	}
	return &meteredClient{inner: c, rec: rec}
}

type meteredClient struct {
	inner Client
	rec   metrics.Recorder
}

func (m *meteredClient) ListCapabilities(ctx context.Context) (ListCapabilitiesResult, error) {
	start := time.Now()
	res, err := m.inner.ListCapabilities(ctx)
	outcome := metrics.OutcomeOK
	if err != nil {
		outcome = metrics.OutcomeError
	}
	m.rec.IncDaemonRPC(metrics.MethodListCapabilities, outcome)
	m.rec.ObserveDaemonRPC(metrics.MethodListCapabilities, outcome, time.Since(start))
	return res, err
}

func (m *meteredClient) GetQuote(ctx context.Context, sender []byte, capability string) (GetQuoteResult, error) {
	start := time.Now()
	res, err := m.inner.GetQuote(ctx, sender, capability)
	outcome := metrics.OutcomeOK
	if err != nil {
		outcome = metrics.OutcomeError
	}
	m.rec.IncDaemonRPC(metrics.MethodGetQuote, outcome)
	m.rec.ObserveDaemonRPC(metrics.MethodGetQuote, outcome, time.Since(start))
	return res, err
}

func (m *meteredClient) GetTicketParams(ctx context.Context, req GetTicketParamsRequest) (TicketParams, error) {
	start := time.Now()
	res, err := m.inner.GetTicketParams(ctx, req)
	outcome := metrics.OutcomeOK
	if err != nil {
		outcome = metrics.OutcomeError
	}
	m.rec.IncDaemonRPC(metrics.MethodGetTicketParams, outcome)
	m.rec.ObserveDaemonRPC(metrics.MethodGetTicketParams, outcome, time.Since(start))
	return res, err
}

func (m *meteredClient) ProcessPayment(ctx context.Context, paymentBytes []byte, workID string) (ProcessPaymentResult, error) {
	start := time.Now()
	res, err := m.inner.ProcessPayment(ctx, paymentBytes, workID)
	outcome := metrics.OutcomeOK
	if err != nil {
		outcome = metrics.OutcomeError
	}
	m.rec.IncDaemonRPC(metrics.MethodProcessPayment, outcome)
	m.rec.ObserveDaemonRPC(metrics.MethodProcessPayment, outcome, time.Since(start))
	return res, err
}

func (m *meteredClient) DebitBalance(ctx context.Context, sender []byte, workID string, workUnits int64) (DebitBalanceResult, error) {
	start := time.Now()
	res, err := m.inner.DebitBalance(ctx, sender, workID, workUnits)
	outcome := metrics.OutcomeOK
	if err != nil {
		outcome = metrics.OutcomeError
	}
	m.rec.IncDaemonRPC(metrics.MethodDebitBalance, outcome)
	m.rec.ObserveDaemonRPC(metrics.MethodDebitBalance, outcome, time.Since(start))
	return res, err
}

// Close passes through; lifecycle calls are not metered.
func (m *meteredClient) Close() error {
	return m.inner.Close()
}
