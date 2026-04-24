package http

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Cloud-SPE/openai-worker-node/internal/config"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/payeedaemon"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

// Mux is the worker's routing surface. Two entry points:
//
//   - Register(method, path, handler)     → unpaid; for /health,
//     /capabilities, /quote, /quotes.
//   - RegisterPaidRoute(module)           → wraps module in the
//     payment middleware and binds at (module.HTTPMethod,
//     module.HTTPPath).
//
// No direct access to the underlying http.ServeMux is exposed —
// forcing every paid request through paymentMiddleware is the only
// way core belief #3 stays enforceable.
type Mux struct {
	cfg    *config.Config
	payee  payeedaemon.Client
	logger *slog.Logger
	inner  *http.ServeMux

	// registered tracks every (method, path) already bound so we can
	// fail loudly on duplicates at startup rather than silently
	// shadowing.
	registered map[string]struct{}

	// paidCapabilities tracks capabilities that have a paid route
	// registered, so we can confirm all config-declared capabilities
	// have a module before Start.
	paidCapabilities map[types.CapabilityID]struct{}
}

// NewMux wires a Mux against a validated config and a connected
// payee-daemon client. The logger is threaded into paymentMiddleware
// for structured per-request event emission.
func NewMux(cfg *config.Config, payee payeedaemon.Client, logger *slog.Logger) *Mux {
	if logger == nil {
		logger = slog.Default()
	}
	return &Mux{
		cfg:              cfg,
		payee:            payee,
		logger:           logger,
		inner:            http.NewServeMux(),
		registered:       map[string]struct{}{},
		paidCapabilities: map[types.CapabilityID]struct{}{},
	}
}

// Register binds an unpaid handler. Panics on duplicate (method, path)
// — startup wiring mistakes should fail loudly.
func (m *Mux) Register(method, path string, h http.HandlerFunc) {
	key := method + " " + path
	if _, dup := m.registered[key]; dup {
		panic(fmt.Sprintf("Mux.Register: duplicate route %q", key))
	}
	m.registered[key] = struct{}{}
	m.inner.HandleFunc(key, h)
}

// RegisterPaidRoute wraps a Module in the payment middleware and binds
// its declared (HTTPMethod, HTTPPath). Panics on duplicate.
//
// This is the ONLY public API on Mux that mounts paid routes. Future
// custom lint (payment-middleware-check) verifies every capability
// module reaches the mux via this method.
func (m *Mux) RegisterPaidRoute(mod modules.Module) {
	key := mod.HTTPMethod() + " " + mod.HTTPPath()
	if _, dup := m.registered[key]; dup {
		panic(fmt.Sprintf("Mux.RegisterPaidRoute: duplicate route %q", key))
	}
	m.registered[key] = struct{}{}
	m.paidCapabilities[mod.Capability()] = struct{}{}

	handler := paymentMiddleware(paidRouteDeps{
		module: mod,
		cfg:    m.cfg,
		payee:  m.payee,
		logger: m.logger,
	})
	m.inner.HandleFunc(key, handler)
}

// HasPaidCapability reports whether a module for the given capability
// has been registered. Useful for the startup check that every
// config-declared capability has a module backing it.
func (m *Mux) HasPaidCapability(c types.CapabilityID) bool {
	_, ok := m.paidCapabilities[c]
	return ok
}

// Handler exposes the underlying http.Handler for use by http.Server.
// Not for registering new routes — use Register / RegisterPaidRoute.
func (m *Mux) Handler() http.Handler {
	return m.inner
}
