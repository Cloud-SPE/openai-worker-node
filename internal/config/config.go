package config

import (
	"fmt"

	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

// CurrentProtocolVersion is the shared worker.yaml schema version this
// worker build accepts.
const CurrentProtocolVersion = 1

// CurrentAPIVersion is the worker HTTP surface version advertised on
// /health. It evolves independently from CurrentProtocolVersion.
const CurrentAPIVersion = 1

// Config is the worker's projection of worker.yaml.
//
// In v3.0.1 worker.yaml is shared between the worker and receiver-mode
// payment daemon. The worker owns the top-level worker-facing fields,
// captures payment_daemon opaquely, and flattens capabilities into a
// (CapabilityID, ModelID) -> ModelRoute map for O(1) routing in the
// middleware.
type Config struct {
	// ProtocolVersion is the shared worker.yaml schema version accepted
	// by this build. The parser validates the on-disk value equals
	// CurrentProtocolVersion before projection.
	ProtocolVersion int32
	// APIVersion is the worker HTTP contract version advertised on
	// /health.
	APIVersion int32
	// WorkerEthAddress is optional orch-internal metadata surfaced only
	// on /registry/offerings when configured.
	WorkerEthAddress string
	// AuthToken is the optional orch-issued bearer token protecting
	// /registry/offerings.
	AuthToken string

	// Worker holds the worker-only fields (http_listen,
	// payment_daemon_socket, etc.).
	Worker WorkerSection

	// Capabilities exposes the parsed capability catalog in two views:
	//   - Ordered: iteration order matches the YAML, for deterministic
	//     /registry/offerings output and catalog comparison.
	//   - Route:   (capability, model) → routing target, for
	//     middleware and module dispatch.
	Capabilities CapabilityCatalog
}

// WorkerSection holds the parsed worker-only fields.
type WorkerSection struct {
	HTTPListen                     string
	PaymentDaemonSocket            string
	MaxConcurrentRequests          int
	VerifyDaemonConsistencyOnStart bool
}

// CapabilityCatalog is the flattened routing table.
type CapabilityCatalog struct {
	// Ordered is the full set as it appears in the YAML. Iterate this
	// for /registry/offerings output.
	Ordered []CapabilityEntry
	// Route is the flat lookup used on every request.
	Route map[RouteKey]ModelRoute
}

// CapabilityEntry is one row of the ordered view.
type CapabilityEntry struct {
	Capability types.CapabilityID
	WorkUnit   types.WorkUnit
	Extra      map[string]any
	Offerings  []OfferingEntry
}

// OfferingEntry is one row of a capability's offering list.
type OfferingEntry struct {
	Model               types.ModelID
	PricePerWorkUnitWei string
	BackendURL          string
	Constraints         map[string]any
}

// RouteKey is the composite lookup key.
type RouteKey struct {
	Capability types.CapabilityID
	Model      types.ModelID
}

// ModelRoute is the per-(capability, model) routing target, materialized
// once at startup.
type ModelRoute struct {
	Capability          types.CapabilityID
	Model               types.ModelID
	WorkUnit            types.WorkUnit
	BackendURL          string
	PricePerWorkUnitWei string
}

// New constructs a *Config from its parts, building the flat Route map
// from the ordered capability list. Used by Load (after parsing
// worker.yaml) and by tests that build fixtures in memory.
func New(w WorkerSection, ordered []CapabilityEntry) *Config {
	cfg := &Config{
		ProtocolVersion: CurrentProtocolVersion,
		APIVersion:      CurrentAPIVersion,
		Worker:          w,
		Capabilities: CapabilityCatalog{
			Ordered: append([]CapabilityEntry(nil), ordered...),
			Route:   make(map[RouteKey]ModelRoute, len(ordered)*2),
		},
	}
	for _, entry := range ordered {
		for _, m := range entry.Offerings {
			cfg.Capabilities.Route[RouteKey{Capability: entry.Capability, Model: m.Model}] = ModelRoute{
				Capability:          entry.Capability,
				Model:               m.Model,
				WorkUnit:            entry.WorkUnit,
				BackendURL:          m.BackendURL,
				PricePerWorkUnitWei: m.PricePerWorkUnitWei,
			}
		}
	}
	return cfg
}

// Load reads, validates, and projects worker.yaml.
func Load(path string) (*Config, error) {
	parsed, err := parseFile(path)
	if err != nil {
		return nil, err
	}
	if err := validate(parsed); err != nil {
		return nil, fmt.Errorf("config: validate: %w", err)
	}
	return projectFromYAML(parsed), nil
}

// Lookup returns the routing target for a (capability, model) pair, or
// false if unknown. Used by the middleware to resolve a request to a
// backend URL before it hits the module's Serve method.
func (c *Config) Lookup(cap types.CapabilityID, model types.ModelID) (ModelRoute, bool) {
	r, ok := c.Capabilities.Route[RouteKey{Capability: cap, Model: model}]
	return r, ok
}
