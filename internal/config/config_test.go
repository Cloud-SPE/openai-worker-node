package config

import (
	"strings"
	"testing"

	"github.com/Cloud-SPE/openai-worker-node/internal/providers/payeedaemon"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

func goodConfig() *Config {
	return New(
		WorkerSection{
			HTTPListen:            "0.0.0.0:8080",
			PaymentDaemonSocket:   "/tmp/lpd.sock",
			MaxConcurrentRequests: 32,
		},
		[]CapabilityEntry{
			{
				Capability: "openai:/v1/chat/completions",
				WorkUnit:   "token",
				Offerings: []OfferingEntry{
					{Model: "llama-3.3-70b", PricePerWorkUnitWei: "2000000000", BackendURL: "http://localhost:8000"},
					{Model: "mistral-7b-instruct", PricePerWorkUnitWei: "500000000", BackendURL: "http://localhost:8001"},
				},
			},
			{
				Capability: "openai:/v1/embeddings",
				WorkUnit:   "token",
				Offerings: []OfferingEntry{
					{Model: "text-embedding-3-small", PricePerWorkUnitWei: "100000000", BackendURL: "http://localhost:8002"},
				},
			},
		},
	)
}

func sharedYAML(extra string) string {
	base := `
protocol_version: 1
payment_daemon:
  recipient_eth_address: "0x1111111111111111111111111111111111111111"
worker:
  http_listen: "0.0.0.0:8080"
  payment_daemon_socket: "/var/run/livepeer/payment-daemon.sock"
  max_concurrent_requests: 16
  verify_daemon_consistency_on_start: true
capabilities:
  - capability: "openai:/v1/chat/completions"
    work_unit: token
    offerings:
      - id: "test-model"
        price_per_work_unit_wei: "1250000"
        backend_url: "http://backend:8000"
`
	return strings.TrimSpace(base + "\n" + extra)
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
		Capabilities: []payeedaemon.Capability{
			{
				Capability: "openai:/v1/chat/completions",
				WorkUnit:   "token",
				Offerings: []payeedaemon.OfferingPrice{
					{ID: "llama-3.3-70b", PricePerWorkUnitWei: "2000000000"},
					{ID: "mistral-7b-instruct", PricePerWorkUnitWei: "500000000"},
				},
			},
			{
				Capability: "openai:/v1/embeddings",
				WorkUnit:   "token",
				Offerings: []payeedaemon.OfferingPrice{
					{ID: "text-embedding-3-small", PricePerWorkUnitWei: "100000000"},
				},
			},
		},
	}
	if err := VerifyDaemonCatalog(cfg, daemon); err != nil {
		t.Errorf("happy path: %v", err)
	}
}

func TestVerifyDaemonCatalog_PriceMismatch(t *testing.T) {
	cfg := goodConfig()
	daemon := payeedaemon.ListCapabilitiesResult{
		Capabilities: []payeedaemon.Capability{
			{
				Capability: "openai:/v1/chat/completions",
				WorkUnit:   "token",
				Offerings: []payeedaemon.OfferingPrice{
					{ID: "llama-3.3-70b", PricePerWorkUnitWei: "1"},
					{ID: "mistral-7b-instruct", PricePerWorkUnitWei: "500000000"},
				},
			},
			{
				Capability: "openai:/v1/embeddings",
				WorkUnit:   "token",
				Offerings: []payeedaemon.OfferingPrice{
					{ID: "text-embedding-3-small", PricePerWorkUnitWei: "100000000"},
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
		Capabilities: nil,
	}
	err := VerifyDaemonCatalog(cfg, daemon)
	if err == nil || !strings.Contains(err.Error(), "capability count mismatch") {
		t.Errorf("got %v, want count-mismatch error", err)
	}
}

func TestParseReader_SharedWorkerYAML(t *testing.T) {
	cfg, err := parseReader(strings.NewReader(sharedYAML(`
worker_eth_address: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
auth_token: "secret-token"
`)))
	if err != nil {
		t.Fatalf("parseReader: %v", err)
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("validate: %v", err)
	}
	projected := projectFromYAML(cfg)
	if projected.ProtocolVersion != CurrentProtocolVersion {
		t.Fatalf("protocol_version: got %d", projected.ProtocolVersion)
	}
	if projected.APIVersion != CurrentAPIVersion {
		t.Fatalf("api_version: got %d", projected.APIVersion)
	}
	if projected.WorkerEthAddress != "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("worker_eth_address: got %q", projected.WorkerEthAddress)
	}
	if projected.AuthToken != "secret-token" {
		t.Fatalf("auth_token: got %q", projected.AuthToken)
	}
}

func TestValidate_RejectsServiceRegistryPublisher(t *testing.T) {
	cfg, err := parseReader(strings.NewReader(sharedYAML(`
service_registry_publisher:
  enabled: true
`)))
	if err != nil {
		t.Fatalf("parseReader: %v", err)
	}
	err = validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "service_registry_publisher") {
		t.Fatalf("got %v, want service_registry_publisher rejection", err)
	}
}

func TestValidate_RejectsUnsupportedProtocolVersion(t *testing.T) {
	cfg, err := parseReader(strings.NewReader(strings.Replace(sharedYAML(""), "protocol_version: 1", "protocol_version: 9", 1)))
	if err != nil {
		t.Fatalf("parseReader: %v", err)
	}
	err = validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "protocol_version=9") {
		t.Fatalf("got %v, want protocol_version rejection", err)
	}
}

func TestValidate_RejectsMissingPaymentDaemon(t *testing.T) {
	cfg, err := parseReader(strings.NewReader(`
protocol_version: 1
worker:
  http_listen: "0.0.0.0:8080"
  payment_daemon_socket: "/var/run/livepeer/payment-daemon.sock"
  max_concurrent_requests: 16
  verify_daemon_consistency_on_start: true
capabilities:
  - capability: "openai:/v1/chat/completions"
    work_unit: token
    offerings:
      - id: "test-model"
        price_per_work_unit_wei: "1250000"
        backend_url: "http://backend:8000"
`))
	if err != nil {
		t.Fatalf("parseReader: %v", err)
	}
	err = validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "missing 'payment_daemon'") {
		t.Fatalf("got %v, want payment_daemon rejection", err)
	}
}

func TestValidate_RejectsBadWorkerEthAddress(t *testing.T) {
	cfg, err := parseReader(strings.NewReader(sharedYAML(`
worker_eth_address: "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
`)))
	if err != nil {
		t.Fatalf("parseReader: %v", err)
	}
	err = validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "worker_eth_address") {
		t.Fatalf("got %v, want worker_eth_address rejection", err)
	}
}

func TestValidate_RejectsNonObjectExtraAndConstraints(t *testing.T) {
	cfg, err := parseReader(strings.NewReader(`
protocol_version: 1
payment_daemon:
  recipient_eth_address: "0x1111111111111111111111111111111111111111"
worker:
  http_listen: "0.0.0.0:8080"
  payment_daemon_socket: "/var/run/livepeer/payment-daemon.sock"
  max_concurrent_requests: 16
  verify_daemon_consistency_on_start: true
capabilities:
  - capability: "openai:/v1/chat/completions"
    work_unit: token
    extra: "bad"
    offerings:
      - id: "test-model"
        price_per_work_unit_wei: "1250000"
        backend_url: "http://backend:8000"
        constraints: [1, 2]
`))
	if err != nil {
		if !strings.Contains(err.Error(), "JSON object") {
			t.Fatalf("parseReader: %v", err)
		}
		return
	}
	err = validate(cfg)
	if err == nil || !strings.Contains(err.Error(), ".extra") {
		t.Fatalf("got %v, want extra rejection", err)
	}
}

func TestProjectFromYAML_OptionalObjectsPassThrough(t *testing.T) {
	cfg, err := parseReader(strings.NewReader(`
protocol_version: 1
payment_daemon:
  recipient_eth_address: "0x1111111111111111111111111111111111111111"
worker:
  http_listen: "0.0.0.0:8080"
  payment_daemon_socket: "/var/run/livepeer/payment-daemon.sock"
  max_concurrent_requests: 16
  verify_daemon_consistency_on_start: true
capabilities:
  - capability: "openai:/v1/chat/completions"
    work_unit: token
    extra:
      supports_streaming: true
    offerings:
      - id: "test-model"
        price_per_work_unit_wei: "1250000"
        backend_url: "http://backend:8000"
        constraints:
          max_context_tokens: 8192
`))
	if err != nil {
		t.Fatalf("parseReader: %v", err)
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("validate: %v", err)
	}
	projected := projectFromYAML(cfg)
	if got := projected.Capabilities.Ordered[0].Extra["supports_streaming"]; got != true {
		t.Fatalf("extra: got %#v", projected.Capabilities.Ordered[0].Extra)
	}
	if got := projected.Capabilities.Ordered[0].Offerings[0].Constraints["max_context_tokens"]; got != 8192 {
		t.Fatalf("constraints: got %#v", projected.Capabilities.Ordered[0].Offerings[0].Constraints)
	}
}
