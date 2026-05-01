package http

import (
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"

	"github.com/Cloud-SPE/openai-worker-node/internal/config"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/payeedaemon"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxTicketParamsBodyBytes = 8 << 10 // 8 KiB

// RegisterUnpaidHandlers binds the standard unpaid routes on mux.
// Called from server wiring; split out so tests can bind a mux
// without the full Server.
func RegisterUnpaidHandlers(m *Mux, cfg *config.Config) {
	m.Register(http.MethodGet, "/health", healthHandler(cfg, m))
	m.Register(http.MethodGet, "/registry/offerings", registryOfferingsHandler(cfg))
	m.Register(http.MethodPost, "/v1/payment/ticket-params", ticketParamsHandler(cfg, m.payee))
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
		if !requireBearerAuth(w, r, cfg.AuthToken) {
			return
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

type ticketParamsRequestJSON struct {
	SenderETHAddress    string `json:"sender_eth_address"`
	RecipientETHAddress string `json:"recipient_eth_address"`
	FaceValueWei        string `json:"face_value_wei"`
	Capability          string `json:"capability"`
	Offering            string `json:"offering"`
}

type ticketParamsResponseJSON struct {
	TicketParams ticketParamsJSON `json:"ticket_params"`
}

type ticketParamsJSON struct {
	Recipient         string                     `json:"recipient"`
	FaceValue         string                     `json:"face_value"`
	WinProb           string                     `json:"win_prob"`
	RecipientRandHash string                     `json:"recipient_rand_hash"`
	Seed              string                     `json:"seed"`
	ExpirationBlock   string                     `json:"expiration_block"`
	ExpirationParams  ticketExpirationParamsJSON `json:"expiration_params"`
}

type ticketExpirationParamsJSON struct {
	CreationRound          int64  `json:"creation_round"`
	CreationRoundBlockHash string `json:"creation_round_block_hash"`
}

func ticketParamsHandler(cfg *config.Config, payee payeedaemon.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireBearerAuth(w, r, cfg.AuthToken) {
			return
		}
		defer func() { _ = r.Body.Close() }()

		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxTicketParamsBodyBytes))
		dec.DisallowUnknownFields()

		var req ticketParamsRequestJSON
		if err := dec.Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body: "+err.Error())
			return
		}
		if err := ensureSingleJSONDocument(dec); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}

		daemonReq, err := parseTicketParamsRequest(req)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}

		params, err := payee.GetTicketParams(r.Context(), daemonReq)
		if err != nil {
			writeTicketParamsProxyError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ticketParamsResponseJSON{
			TicketParams: renderTicketParamsJSON(params),
		})
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

func requireBearerAuth(w http.ResponseWriter, r *http.Request, authToken string) bool {
	if authToken == "" {
		return true
	}
	gotAuth := r.Header.Get("Authorization")
	want := "Bearer " + authToken
	if subtle.ConstantTimeCompare([]byte(gotAuth), []byte(want)) != 1 {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid bearer token")
		return false
	}
	return true
}

func ensureSingleJSONDocument(dec *json.Decoder) error {
	var tail struct{}
	if err := dec.Decode(&tail); err == nil {
		return fmt.Errorf("request body must contain exactly one JSON object")
	} else if err == io.EOF {
		return nil
	} else {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
}

func parseTicketParamsRequest(in ticketParamsRequestJSON) (payeedaemon.GetTicketParamsRequest, error) {
	sender, err := parseHexAddress("sender_eth_address", in.SenderETHAddress)
	if err != nil {
		return payeedaemon.GetTicketParamsRequest{}, err
	}
	recipient, err := parseHexAddress("recipient_eth_address", in.RecipientETHAddress)
	if err != nil {
		return payeedaemon.GetTicketParamsRequest{}, err
	}
	faceValue, ok := new(big.Int).SetString(strings.TrimSpace(in.FaceValueWei), 10)
	if !ok {
		return payeedaemon.GetTicketParamsRequest{}, fmt.Errorf("face_value_wei must be a decimal integer")
	}
	if faceValue.Sign() <= 0 {
		return payeedaemon.GetTicketParamsRequest{}, fmt.Errorf("face_value_wei must be > 0")
	}
	if strings.TrimSpace(in.Capability) == "" {
		return payeedaemon.GetTicketParamsRequest{}, fmt.Errorf("capability is required")
	}
	if strings.TrimSpace(in.Offering) == "" {
		return payeedaemon.GetTicketParamsRequest{}, fmt.Errorf("offering is required")
	}
	return payeedaemon.GetTicketParamsRequest{
		Sender:     sender,
		Recipient:  recipient,
		FaceValue:  faceValue,
		Capability: strings.TrimSpace(in.Capability),
		Offering:   strings.TrimSpace(in.Offering),
	}, nil
}

func parseHexAddress(field, value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "0x") && !strings.HasPrefix(trimmed, "0X") {
		return nil, fmt.Errorf("%s must be a 0x-prefixed hex address", field)
	}
	raw := trimmed[2:]
	if len(raw) != 40 {
		return nil, fmt.Errorf("%s must be exactly 20 bytes (40 hex chars)", field)
	}
	out, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid hex address", field)
	}
	return out, nil
}

func writeTicketParamsProxyError(w http.ResponseWriter, err error) {
	switch status.Code(err) {
	case codes.InvalidArgument:
		writeJSONError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case codes.Unavailable, codes.DeadlineExceeded:
		writeJSONError(w, http.StatusServiceUnavailable, "payment_daemon_unavailable", err.Error())
	default:
		writeJSONError(w, http.StatusInternalServerError, "ticket_params_unavailable", err.Error())
	}
}

func renderTicketParamsJSON(tp payeedaemon.TicketParams) ticketParamsJSON {
	return ticketParamsJSON{
		Recipient:         bytesToHexString(tp.Recipient),
		FaceValue:         bytesToDecimalString(tp.FaceValueWei),
		WinProb:           bytesToHexString(tp.WinProb),
		RecipientRandHash: bytesToHexString(tp.RecipientRandHash),
		Seed:              bytesToHexString(tp.Seed),
		ExpirationBlock:   bytesToDecimalString(tp.ExpirationBlock),
		ExpirationParams: ticketExpirationParamsJSON{
			CreationRound:          tp.ExpirationParams.CreationRound,
			CreationRoundBlockHash: bytesToHexString(tp.ExpirationParams.CreationRoundBlockHash),
		},
	}
}

func bytesToHexString(b []byte) string {
	if len(b) == 0 {
		return "0x"
	}
	return "0x" + hex.EncodeToString(b)
}

func bytesToDecimalString(b []byte) string {
	if len(b) == 0 {
		return "0"
	}
	return new(big.Int).SetBytes(b).String()
}
