package http

import (
	"encoding/json"
	"net/http"

	"github.com/Cloud-SPE/openai-worker-node/internal/config"
)

// RegisterUnpaidHandlers binds the standard unpaid routes (/health,
// /capabilities) on mux. Called from server wiring; split out so tests
// can bind a mux without the full Server.
//
// /quote and /quotes are deferred to a follow-up plan (proxy through
// PayeeDaemon.GetQuote).
func RegisterUnpaidHandlers(m *Mux, cfg *config.Config) {
	m.Register(http.MethodGet, "/health", healthHandler(cfg))
	m.Register(http.MethodGet, "/capabilities", capabilitiesHandler(cfg))
}

// healthHandler reports liveness + protocol version + configured
// capacity. inflight + queue depth are backlog (need the Server's
// live counter).
func healthHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":           "ok",
			"protocol_version": cfg.ProtocolVersion,
			"max_concurrent":   cfg.Worker.MaxConcurrentRequests,
		})
	}
}

// capabilitiesHandler emits the worker's capability catalog in the
// ordered form the config carries. backend_url is deliberately
// omitted — the bridge shouldn't see where inference is hosted.
func capabilitiesHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		out := struct {
			ProtocolVersion int32            `json:"protocol_version"`
			Capabilities    []capabilityJSON `json:"capabilities"`
		}{
			ProtocolVersion: cfg.ProtocolVersion,
			Capabilities:    make([]capabilityJSON, 0, len(cfg.Capabilities.Ordered)),
		}
		for _, c := range cfg.Capabilities.Ordered {
			models := make([]modelJSON, 0, len(c.Models))
			for _, m := range c.Models {
				models = append(models, modelJSON{
					Model:               string(m.Model),
					PricePerWorkUnitWei: m.PricePerWorkUnitWei,
				})
			}
			out.Capabilities = append(out.Capabilities, capabilityJSON{
				Capability: string(c.Capability),
				WorkUnit:   string(c.WorkUnit),
				Models:     models,
			})
		}
		_ = json.NewEncoder(w).Encode(out)
	}
}

type capabilityJSON struct {
	Capability string      `json:"capability"`
	WorkUnit   string      `json:"work_unit"`
	Models     []modelJSON `json:"models"`
}

type modelJSON struct {
	Model               string `json:"model"`
	PricePerWorkUnitWei string `json:"price_per_work_unit_wei"`
}
