package http

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"

	"github.com/Cloud-SPE/openai-worker-node/internal/config"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/payeedaemon"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

// maxPaidRequestBodyBytes is the absolute cap on a paid route body.
// Chat / embeddings rarely exceed 1 MiB; we pick a generous default
// and will let specific modules lower it via their own validation
// when called for.
const maxPaidRequestBodyBytes = 16 << 20 // 16 MiB

// paidRouteDeps is the dependency bundle the middleware closes over.
// Fields are set once at registration and read-only thereafter.
type paidRouteDeps struct {
	module modules.Module
	cfg    *config.Config
	payee  payeedaemon.Client
	logger *slog.Logger
}

// paymentMiddleware is the canonical paid-request pipeline. Every
// paid route MUST pass through this function; RegisterPaidRoute is
// the only public surface that builds a handler with it wired in.
//
// Flow:
//
//  1. Parse body (bounded).
//  2. Extract + base64-decode the `livepeer-payment` header.
//  3. Derive work_id from the payment bytes.
//  4. ProcessPayment → { sender, credited_ev, balance, winners }.
//  5. Module extracts model; lookup (capability, model) → backend URL.
//  6. EstimateWorkUnits(body, model) → int64.
//  7. DebitBalance(sender, work_id, estimate); reject if balance < 0.
//  8. Module.Serve(...) returns actual work units consumed.
//  9. Reconcile: if actual > estimate, second DebitBalance(delta).
//
// Errors along the way map to the contract documented in
// docs/product-specs/index.md: 402 / 404 / 502 / 503 / 400.
func paymentMiddleware(deps paidRouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// 1. Body.
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPaidRequestBodyBytes))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "failed to read request body: "+err.Error())
			return
		}

		// 2. Payment header.
		hdr := r.Header.Get(types.PaymentHeaderName)
		if hdr == "" {
			writeJSONError(w, http.StatusPaymentRequired, "missing_or_invalid_payment", "missing "+types.PaymentHeaderName+" header")
			return
		}
		paymentBytes, err := base64.StdEncoding.DecodeString(hdr)
		if err != nil {
			writeJSONError(w, http.StatusPaymentRequired, "missing_or_invalid_payment", "header is not valid base64")
			return
		}

		// 3. work_id.
		workID := deriveWorkID(paymentBytes)

		// 4. ProcessPayment.
		pp, err := deps.payee.ProcessPayment(ctx, paymentBytes, string(workID))
		if err != nil {
			deps.logger.Warn("ProcessPayment rejected",
				"capability", deps.module.Capability(),
				"work_id", workID,
				"err", err)
			writeJSONError(w, http.StatusPaymentRequired, "payment_rejected", err.Error())
			return
		}

		// 5. Extract model + resolve route.
		model, err := deps.module.ExtractModel(body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "could not extract model from request body: "+err.Error())
			return
		}
		route, ok := deps.cfg.Lookup(deps.module.Capability(), model)
		if !ok {
			writeJSONError(w, http.StatusNotFound, "capability_not_found", "no backend configured for capability="+string(deps.module.Capability())+" model="+string(model))
			return
		}

		// 6. Upfront estimate.
		estimate, err := deps.module.EstimateWorkUnits(body, model)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "could not estimate work units: "+err.Error())
			return
		}
		if estimate < 0 {
			estimate = 0
		}

		// 7. DebitBalance upfront.
		db, err := deps.payee.DebitBalance(ctx, pp.Sender, string(workID), estimate)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "backend_unavailable", "DebitBalance: "+err.Error())
			return
		}
		if db.BalanceWei.Sign() < 0 {
			deps.logger.Info("insufficient balance after estimate debit",
				"capability", deps.module.Capability(),
				"model", model,
				"estimate_work_units", estimate,
				"balance_wei", db.BalanceWei.String())
			writeJSONError(w, http.StatusPaymentRequired, "insufficient_balance", "balance after estimate debit is negative")
			return
		}

		// 8. Serve.
		actual, err := deps.module.Serve(ctx, w, r, body, model, route.BackendURL)
		if err != nil {
			// Module handlers may partially write the body before
			// erroring. Log and return — we can't replace headers that
			// are already on the wire.
			deps.logger.Warn("module.Serve error",
				"capability", deps.module.Capability(),
				"model", model,
				"err", err)
			return
		}

		// 9. Reconcile over-debit.
		if actual > estimate {
			delta := actual - estimate
			if _, err := deps.payee.DebitBalance(ctx, pp.Sender, string(workID), delta); err != nil {
				// Logged, not surfaced — the response is already sent.
				deps.logger.Warn("reconcile DebitBalance error",
					"capability", deps.module.Capability(),
					"model", model,
					"delta_work_units", delta,
					"err", err)
			}
		}
	}
}

// writeJSONError serialises the worker's error contract shape.
// Central helper so every error surface matches
// docs/product-specs/index.md.
func writeJSONError(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":  code,
		"detail": detail,
	})
}

// ensureBigIntImported keeps math/big in the import set for the balance
// comparison path; dropped by the linter if unused. Placeholder while
// middleware grows.
var _ = new(big.Int)
