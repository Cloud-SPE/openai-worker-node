package config

import (
	"strings"
	"testing"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/payeedaemon"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

func goodConfig() *Config {
	return New(
		CurrentProtocolVersion,
		WorkerSection{
			HTTPListen:            "0.0.0.0:8080",
			PaymentDaemonSocket:   "/tmp/lpd.sock",
			MaxConcurrentRequests: 32,
		},
		[]CapabilityEntry{
			{
				Capability: "openai:/v1/chat/completions",
				WorkUnit:   "token",
				Models: []ModelEntry{
					{Model: "llama-3.3-70b", PricePerWorkUnitWei: "2000000000", BackendURL: "http://localhost:8000"},
					{Model: "mistral-7b-instruct", PricePerWorkUnitWei: "500000000", BackendURL: "http://localhost:8001"},
				},
			},
			{
				Capability: "openai:/v1/embeddings",
				WorkUnit:   "token",
				Models: []ModelEntry{
					{Model: "text-embedding-3-small", PricePerWorkUnitWei: "100000000", BackendURL: "http://localhost:8002"},
				},
			},
		},
	)
}

func TestNew_FlatRouteMap(t *testing.T) {
	cfg := goodConfig()
	if got := len(cfg.Capabilities.Route); got != 3 {
		t.Errorf("route count: got %d, want 3 (one per model across all capabilities)", got)
	}
	route, ok := cfg.Lookup("openai:/v1/chat/completions", "llama-3.3-70b")
	if !ok {
		t.Fatal("Lookup(chat, llama): not found")
	}
	if route.BackendURL != "http://localhost:8000" {
		t.Errorf("backend: got %q", route.BackendURL)
	}
	if route.WorkUnit != types.WorkUnitToken {
		t.Errorf("work_unit: got %q, want token", route.WorkUnit)
	}
}

func TestLookup_UnknownModel(t *testing.T) {
	cfg := goodConfig()
	if _, ok := cfg.Lookup("openai:/v1/chat/completions", "unknown-model"); ok {
		t.Error("expected Lookup miss")
	}
}

func TestVerifyDaemonCatalog_HappyPath(t *testing.T) {
	cfg := goodConfig()
	daemon := payeedaemon.ListCapabilitiesResult{
		ProtocolVersion: cfg.ProtocolVersion,
		Capabilities: []payeedaemon.Capability{
			{
				Capability: "openai:/v1/chat/completions",
				WorkUnit:   "token",
				Models: []payeedaemon.ModelPrice{
					{Model: "llama-3.3-70b", PricePerWorkUnitWei: "2000000000"},
					{Model: "mistral-7b-instruct", PricePerWorkUnitWei: "500000000"},
				},
			},
			{
				Capability: "openai:/v1/embeddings",
				WorkUnit:   "token",
				Models: []payeedaemon.ModelPrice{
					{Model: "text-embedding-3-small", PricePerWorkUnitWei: "100000000"},
				},
			},
		},
	}
	if err := VerifyDaemonCatalog(cfg, daemon); err != nil {
		t.Errorf("happy path: %v", err)
	}
}

func TestVerifyDaemonCatalog_ProtocolVersionMismatch(t *testing.T) {
	cfg := goodConfig()
	daemon := payeedaemon.ListCapabilitiesResult{
		ProtocolVersion: cfg.ProtocolVersion + 1,
	}
	err := VerifyDaemonCatalog(cfg, daemon)
	if err == nil || !strings.Contains(err.Error(), "protocol_version") {
		t.Errorf("got %v, want error mentioning protocol_version", err)
	}
}

func TestVerifyDaemonCatalog_PriceMismatch(t *testing.T) {
	cfg := goodConfig()
	daemon := payeedaemon.ListCapabilitiesResult{
		ProtocolVersion: cfg.ProtocolVersion,
		Capabilities: []payeedaemon.Capability{
			{
				Capability: "openai:/v1/chat/completions",
				WorkUnit:   "token",
				Models: []payeedaemon.ModelPrice{
					{Model: "llama-3.3-70b", PricePerWorkUnitWei: "1"},
					{Model: "mistral-7b-instruct", PricePerWorkUnitWei: "500000000"},
				},
			},
			{
				Capability: "openai:/v1/embeddings",
				WorkUnit:   "token",
				Models: []payeedaemon.ModelPrice{
					{Model: "text-embedding-3-small", PricePerWorkUnitWei: "100000000"},
				},
			},
		},
	}
	err := VerifyDaemonCatalog(cfg, daemon)
	if err == nil || !strings.Contains(err.Error(), "price mismatch") {
		t.Errorf("got %v, want price-mismatch error", err)
	}
}

func TestVerifyDaemonCatalog_CountMismatch(t *testing.T) {
	cfg := goodConfig()
	daemon := payeedaemon.ListCapabilitiesResult{
		ProtocolVersion: cfg.ProtocolVersion,
		Capabilities:    nil,
	}
	err := VerifyDaemonCatalog(cfg, daemon)
	if err == nil || !strings.Contains(err.Error(), "capability count mismatch") {
		t.Errorf("got %v, want count-mismatch error", err)
	}
}
