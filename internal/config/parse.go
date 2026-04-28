package config

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

// yamlConfig is the on-disk worker.yaml shape from the worker's
// perspective. The daemon section is captured as a yaml.Node so the
// worker accepts a co-tenant-shared file without needing to know the
// daemon's schema. KnownFields(true) still applies at the top level —
// unknown sibling keys at root are rejected as typos.
type yamlConfig struct {
	ProtocolVersion int              `yaml:"protocol_version"`
	PaymentDaemon   yaml.Node        `yaml:"payment_daemon"`
	Worker          yamlWorker       `yaml:"worker"`
	Capabilities    []yamlCapability `yaml:"capabilities"`
}

type yamlWorker struct {
	HTTPListen                     string `yaml:"http_listen"`
	PaymentDaemonSocket            string `yaml:"payment_daemon_socket"`
	MaxConcurrentRequests          int    `yaml:"max_concurrent_requests"`
	VerifyDaemonConsistencyOnStart bool   `yaml:"verify_daemon_consistency_on_start"`

	// Optional sections accepted but not consumed by the current worker.
	// Listed explicitly so KnownFields(true) doesn't reject a shared
	// worker.yaml that includes them for the daemon or future modules.
	ServiceRegistryPublisher *yaml.Node `yaml:"service_registry_publisher,omitempty"`
}

type yamlCapability struct {
	Capability string      `yaml:"capability"`
	WorkUnit   string      `yaml:"work_unit"`
	Models     []yamlModel `yaml:"models"`

	// Streaming-only knobs. Optional; not used by the current worker
	// modules (none stream yet) but accepted so a shared worker.yaml
	// can declare them without tripping KnownFields(true).
	DebitCadenceSeconds        int `yaml:"debit_cadence_seconds,omitempty"`
	SufficientMinRunwaySeconds int `yaml:"sufficient_min_runway_seconds,omitempty"`
	SufficientGraceSeconds     int `yaml:"sufficient_grace_seconds,omitempty"`
}

type yamlModel struct {
	Model               string `yaml:"model"`
	PricePerWorkUnitWei string `yaml:"price_per_work_unit_wei"`
	BackendURL          string `yaml:"backend_url"`
}

var capabilityRE = regexp.MustCompile(`^[a-z][a-z0-9]*:.+$`)

// knownWorkUnits is the closed set of accepted work_unit identifiers
// for CurrentProtocolVersion. Adding a unit requires bumping the
// protocol version on both worker and daemon in lockstep.
var knownWorkUnits = map[string]struct{}{
	"token":                 {},
	"character":             {},
	"audio_second":          {},
	"image_step_megapixel":  {},
	"video_frame_megapixel": {},
}

func parseFile(path string) (*yamlConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: open %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return parseReader(f)
}

func parseReader(r io.Reader) (*yamlConfig, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)

	var cfg yamlConfig
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: decode: %w", err)
	}
	var tail yamlConfig
	if err := dec.Decode(&tail); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("config: unexpected second YAML document; only one document per file is supported")
		}
		return nil, fmt.Errorf("config: trailing data after first document: %w", err)
	}
	return &cfg, nil
}

// validate enforces the worker-relevant invariants. payment_daemon.* is
// intentionally not validated — the daemon validates its own section
// and refuses to start on errors, so the worker fails fast at gRPC
// dial time anyway.
func validate(cfg *yamlConfig) error {
	if cfg == nil {
		return errors.New("config.validate: nil config")
	}
	if cfg.ProtocolVersion != CurrentProtocolVersion {
		return fmt.Errorf("protocol_version: got %d, expected %d", cfg.ProtocolVersion, CurrentProtocolVersion)
	}
	if err := validateWorker(&cfg.Worker); err != nil {
		return err
	}
	return validateCapabilities(cfg.Capabilities)
}

func validateWorker(w *yamlWorker) error {
	if w.HTTPListen == "" {
		return errors.New("worker.http_listen: required")
	}
	if w.PaymentDaemonSocket == "" {
		return errors.New("worker.payment_daemon_socket: required")
	}
	if w.MaxConcurrentRequests <= 0 {
		return fmt.Errorf("worker.max_concurrent_requests: must be > 0 (got %d)", w.MaxConcurrentRequests)
	}
	return nil
}

func validateCapabilities(caps []yamlCapability) error {
	if len(caps) == 0 {
		return errors.New("capabilities: at least one capability required")
	}
	seen := make(map[string]struct{}, len(caps))
	for i, c := range caps {
		if err := validateCapability(i, &c); err != nil {
			return err
		}
		if _, dup := seen[c.Capability]; dup {
			return fmt.Errorf("capabilities[%d].capability: duplicate %q", i, c.Capability)
		}
		seen[c.Capability] = struct{}{}
	}
	return nil
}

func validateCapability(i int, c *yamlCapability) error {
	prefix := fmt.Sprintf("capabilities[%d]", i)
	if !capabilityRE.MatchString(c.Capability) {
		return fmt.Errorf(`%s.capability: must match ^[a-z][a-z0-9]*:.+$ (got %q)`, prefix, c.Capability)
	}
	if _, ok := knownWorkUnits[c.WorkUnit]; !ok {
		return fmt.Errorf("%s.work_unit: must be one of %s (got %q)", prefix, strings.Join(sortedKeys(knownWorkUnits), "|"), c.WorkUnit)
	}
	if len(c.Models) == 0 {
		return fmt.Errorf("%s.models: at least one model required", prefix)
	}
	seen := make(map[string]struct{}, len(c.Models))
	for j, m := range c.Models {
		if err := validateModel(prefix, j, &m); err != nil {
			return err
		}
		if _, dup := seen[m.Model]; dup {
			return fmt.Errorf("%s.models[%d].model: duplicate %q within capability", prefix, j, m.Model)
		}
		seen[m.Model] = struct{}{}
	}
	return nil
}

func validateModel(capPrefix string, j int, m *yamlModel) error {
	prefix := fmt.Sprintf("%s.models[%d]", capPrefix, j)
	if m.Model == "" {
		return fmt.Errorf("%s.model: required", prefix)
	}
	if m.PricePerWorkUnitWei == "" {
		return fmt.Errorf("%s.price_per_work_unit_wei: required", prefix)
	}
	price, ok := new(big.Int).SetString(m.PricePerWorkUnitWei, 10)
	if !ok {
		return fmt.Errorf("%s.price_per_work_unit_wei: %q is not a decimal integer", prefix, m.PricePerWorkUnitWei)
	}
	if price.Sign() <= 0 {
		return fmt.Errorf("%s.price_per_work_unit_wei: must be > 0 (got %q)", prefix, m.PricePerWorkUnitWei)
	}
	if m.BackendURL == "" {
		return fmt.Errorf("%s.backend_url: required", prefix)
	}
	if _, err := url.Parse(m.BackendURL); err != nil {
		return fmt.Errorf("%s.backend_url: %w", prefix, err)
	}
	return nil
}

func projectFromYAML(y *yamlConfig) *Config {
	ordered := make([]CapabilityEntry, 0, len(y.Capabilities))
	for _, c := range y.Capabilities {
		entry := CapabilityEntry{
			Capability: types.CapabilityID(c.Capability),
			WorkUnit:   types.WorkUnit(c.WorkUnit),
			Models:     make([]ModelEntry, 0, len(c.Models)),
		}
		for _, m := range c.Models {
			entry.Models = append(entry.Models, ModelEntry{
				Model:               types.ModelID(m.Model),
				PricePerWorkUnitWei: m.PricePerWorkUnitWei,
				BackendURL:          m.BackendURL,
			})
		}
		ordered = append(ordered, entry)
	}
	return New(int32(y.ProtocolVersion), WorkerSection{
		HTTPListen:                     y.Worker.HTTPListen,
		PaymentDaemonSocket:            y.Worker.PaymentDaemonSocket,
		MaxConcurrentRequests:          y.Worker.MaxConcurrentRequests,
		VerifyDaemonConsistencyOnStart: y.Worker.VerifyDaemonConsistencyOnStart,
	}, ordered)
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
