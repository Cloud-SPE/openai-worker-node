package http

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/Cloud-SPE/openai-worker-node/internal/config"
)

// RegisterUnpaidHandlers binds the standard unpaid routes on mux.
// Called from server wiring; split out so tests can bind a mux
// without the full Server.
func RegisterUnpaidHandlers(m *Mux, cfg *config.Config) {
	m.Register(http.MethodGet, "/health", healthHandler(cfg, m))
	m.Register(http.MethodGet, "/registry/offerings", registryOfferingsHandler(cfg))
}

// healthHandler reports liveness + worker.yaml schema version + HTTP
// API version + configured capacity + current paid-route inflight
// count.
func healthHandler(cfg *config.Config, mux *Mux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":           "ok",
			"api_version":      cfg.APIVersion,
			"protocol_version": cfg.ProtocolVersion,
			"max_concurrent":   mux.MaxConcurrentPaid(),
			"inflight":         mux.InflightPaid(),
		})
	}
}

type offeringJSON struct {
	ID                  string `json:"id"`
	PricePerWorkUnitWei string `json:"price_per_work_unit_wei"`
	Constraints         any    `json:"constraints,omitempty"`
}

// registryOfferingsHandler emits the modules-canonical capability fragment
// the orch-coordinator scrapes to pre-fill the operator's roster (per
// service-registry-daemon/docs/design-docs/worker-offerings-endpoint.md).
//
// Body shape: `{"capabilities": [{"name", "work_unit", "offerings": [...]}]}`
// — same outer envelope the modules manifest uses at
// `nodes[].capabilities[]`. Worker doesn't include node identity
// (id/url/region/lat/lon); operator types those into the coordinator's
// roster row alongside the worker URL.
//
// Auth: optional bearer via shared worker.yaml auth_token. If unset,
// plain HTTP.
func registryOfferingsHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.AuthToken != "" {
			gotAuth := r.Header.Get("Authorization")
			want := "Bearer " + cfg.AuthToken
			if subtle.ConstantTimeCompare([]byte(gotAuth), []byte(want)) != 1 {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid bearer token")
				return
			}
		}
		out := registryOfferingsJSON{
			Capabilities: make([]registryCapabilityJSON, 0, len(cfg.Capabilities.Ordered)),
		}
		if cfg.WorkerEthAddress != "" {
			out.WorkerEthAddress = cfg.WorkerEthAddress
		}
		for _, c := range cfg.Capabilities.Ordered {
			offerings := make([]offeringJSON, 0, len(c.Offerings))
			for _, o := range c.Offerings {
				offerings = append(offerings, offeringJSON{
					ID:                  string(o.Model),
					PricePerWorkUnitWei: o.PricePerWorkUnitWei,
					Constraints:         omitEmptyMap(o.Constraints),
				})
			}
			out.Capabilities = append(out.Capabilities, registryCapabilityJSON{
				Name:      string(c.Capability),
				WorkUnit:  string(c.WorkUnit),
				Extra:     omitEmptyMap(c.Extra),
				Offerings: offerings,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

type registryOfferingsJSON struct {
	WorkerEthAddress string                   `json:"worker_eth_address,omitempty"`
	Capabilities     []registryCapabilityJSON `json:"capabilities"`
}

// registryCapabilityJSON is the modules-canonical capability fragment
// shape (`name` not `capability`, matching nodes[].capabilities[].name
// in the signed manifest pipeline).
type registryCapabilityJSON struct {
	Name      string         `json:"name"`
	WorkUnit  string         `json:"work_unit"`
	Extra     any            `json:"extra,omitempty"`
	Offerings []offeringJSON `json:"offerings"`
}

func omitEmptyMap(m map[string]any) any {
	if len(m) == 0 {
		return nil
	}
	return m
}
