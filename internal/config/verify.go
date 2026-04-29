package config

import (
	"fmt"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/payeedaemon"
)

// VerifyDaemonCatalog compares the worker's parsed capability set
// against what the payee daemon returned from ListCapabilities.
// Byte-equal on everything both sides parse; the worker's BackendURL
// field is excluded because the daemon doesn't see it.
//
// Mismatch is a fail-closed startup condition. Returns an error with a
// human-readable explanation of the first drift found. The error
// message is safe to log and exit on.
//
// Run unconditionally in production; worker.VerifyDaemonConsistencyOnStart
// being false is the operator's escape hatch for dev environments that
// knowingly run out of lockstep.
func VerifyDaemonCatalog(cfg *Config, daemon payeedaemon.ListCapabilitiesResult) error {
	if cfg == nil {
		return fmt.Errorf("verify: nil config")
	}
	if cfg.ProtocolVersion != daemon.ProtocolVersion {
		return fmt.Errorf("verify: protocol_version mismatch: config=%d daemon=%d (upgrade one side to match)", cfg.ProtocolVersion, daemon.ProtocolVersion)
	}
	if got, want := len(daemon.Capabilities), len(cfg.Capabilities.Ordered); got != want {
		return fmt.Errorf("verify: capability count mismatch: config has %d, daemon has %d", want, got)
	}
	for i, cfgCap := range cfg.Capabilities.Ordered {
		daemonCap := daemon.Capabilities[i]
		if string(cfgCap.Capability) != daemonCap.Capability {
			return fmt.Errorf("verify: capability[%d] mismatch: config=%q daemon=%q", i, cfgCap.Capability, daemonCap.Capability)
		}
		if string(cfgCap.WorkUnit) != daemonCap.WorkUnit {
			return fmt.Errorf("verify: capability[%d] (%q) work_unit mismatch: config=%q daemon=%q", i, cfgCap.Capability, cfgCap.WorkUnit, daemonCap.WorkUnit)
		}
		// payment-daemon proto still uses `Models` (its own contract,
		// not renamed in v3.0.0 — only the registry contract renamed).
		// Compare config offerings against daemon models 1:1 by index.
		if got, want := len(daemonCap.Models), len(cfgCap.Offerings); got != want {
			return fmt.Errorf("verify: capability[%d] (%q) offering count mismatch: config=%d daemon=%d", i, cfgCap.Capability, want, got)
		}
		for j, cfgOffering := range cfgCap.Offerings {
			daemonModel := daemonCap.Models[j]
			if string(cfgOffering.Model) != daemonModel.Model {
				return fmt.Errorf("verify: capability[%d] (%q) offering[%d] id mismatch: config=%q daemon=%q", i, cfgCap.Capability, j, cfgOffering.Model, daemonModel.Model)
			}
			if cfgOffering.PricePerWorkUnitWei != daemonModel.PricePerWorkUnitWei {
				return fmt.Errorf("verify: capability[%d] (%q) offering[%d] (%q) price mismatch: config=%q daemon=%q", i, cfgCap.Capability, j, cfgOffering.Model, cfgOffering.PricePerWorkUnitWei, daemonModel.PricePerWorkUnitWei)
			}
		}
	}
	return nil
}
