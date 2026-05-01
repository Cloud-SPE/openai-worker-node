package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Cloud-SPE/openai-worker-node/internal/config"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/payeedaemon"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeModule is a minimal Module for middleware tests.
type fakeModule struct {
	capability     types.CapabilityID
	path           string
	unit           string
	extractModelFn func(body []byte) (types.ModelID, error)
	estimateFn     func(body []byte, model types.ModelID) (int64, error)
	serveFn        func(ctx context.Context, w nethttp.ResponseWriter, r *nethttp.Request, body []byte, model types.ModelID, backendURL string) (int64, error)

	// observed on Serve
	servedBackend atomic.Value // string
}

func (f *fakeModule) Capability() types.CapabilityID { return f.capability }
func (f *fakeModule) HTTPMethod() string             { return nethttp.MethodPost }
func (f *fakeModule) HTTPPath() string               { return f.path }
func (f *fakeModule) Unit() string {
	if f.unit == "" {
		return "token"
	}
	return f.unit
}
func (f *fakeModule) ExtractModel(body []byte) (types.ModelID, error) {
	return f.extractModelFn(body)
}
func (f *fakeModule) EstimateWorkUnits(body []byte, model types.ModelID) (int64, error) {
	return f.estimateFn(body, model)
}
func (f *fakeModule) Serve(
	ctx context.Context,
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	body []byte,
	model types.ModelID,
	backendURL string,
) (int64, error) {
	f.servedBackend.Store(backendURL)
	return f.serveFn(ctx, w, r, body, model, backendURL)
}

// testFixture builds a fully-wired Mux with a single fake module, plus
// a fake payee-daemon and a config with one capability / one model.
type testFixture struct {
	mux    *Mux
	payee  *payeedaemon.Fake
	module *fakeModule
	cfg    *config.Config
}

func buildFixture(t *testing.T) *testFixture {
	t.Helper()
	cfg := config.New(
		config.WorkerSection{
			HTTPListen:            "127.0.0.1:0",
			PaymentDaemonSocket:   "/tmp/fake.sock",
			MaxConcurrentRequests: 8,
		},
		[]config.CapabilityEntry{
			{
				Capability: "openai:/v1/chat/completions",
				WorkUnit:   "token",
				Extra: map[string]any{
					"supports_streaming": true,
				},
				Offerings: []config.OfferingEntry{
					{
						Model:               "test-model",
						PricePerWorkUnitWei: "100",
						BackendURL:          "http://backend.local:9000",
						Constraints: map[string]any{
							"max_context_tokens": 8192,
						},
					},
				},
			},
		},
	)
	cfg.WorkerEthAddress = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	payee := payeedaemon.NewFake()
	mod := &fakeModule{
		capability: "openai:/v1/chat/completions",
		path:       "/v1/chat/completions",
		extractModelFn: func(body []byte) (types.ModelID, error) {
			var req struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				return "", err
			}
			if req.Model == "" {
				return "", errors.New("missing model field")
			}
			return types.ModelID(req.Model), nil
		},
		estimateFn: func(body []byte, model types.ModelID) (int64, error) {
			return 10, nil
		},
		serveFn: func(ctx context.Context, w nethttp.ResponseWriter, r *nethttp.Request, body []byte, model types.ModelID, backendURL string) (int64, error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(nethttp.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"model": string(model), "backend": backendURL})
			return 10, nil
		},
	}
	mux := NewMux(cfg, payee, nil)
	mux.RegisterPaidRoute(mod)
	return &testFixture{mux: mux, payee: payee, module: mod, cfg: cfg}
}

func doPaidRequest(t *testing.T, f *testFixture, body, paymentHeader string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(nethttp.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	if paymentHeader != "" {
		req.Header.Set(types.PaymentHeaderName, paymentHeader)
	}
	rr := httptest.NewRecorder()
	f.mux.Handler().ServeHTTP(rr, req)
	return rr
}

func validHeader() string {
	return base64.StdEncoding.EncodeToString([]byte("fake-payment-bytes"))
}

func TestPaymentMiddleware_HappyPath(t *testing.T) {
	f := buildFixture(t)
	rr := doPaidRequest(t, f, `{"model":"test-model"}`, validHeader())
	if rr.Code != nethttp.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	if f.payee.ProcessPaymentCalls != 1 {
		t.Errorf("ProcessPayment calls: got %d, want 1", f.payee.ProcessPaymentCalls)
	}
	if f.payee.DebitBalanceCalls != 1 {
		t.Errorf("DebitBalance calls: got %d, want 1 (no reconcile when actual==estimate)", f.payee.DebitBalanceCalls)
	}
	if f.payee.LastDebitBalanceWorkUnits != 10 {
		t.Errorf("debit work_units: got %d, want 10", f.payee.LastDebitBalanceWorkUnits)
	}
	if got, _ := f.module.servedBackend.Load().(string); got != "http://backend.local:9000" {
		t.Errorf("served backend: got %q, want http://backend.local:9000", got)
	}
}

func TestPaymentMiddleware_MissingHeader(t *testing.T) {
	f := buildFixture(t)
	rr := doPaidRequest(t, f, `{"model":"test-model"}`, "")
	if rr.Code != nethttp.StatusPaymentRequired {
		t.Fatalf("status: got %d, want 402", rr.Code)
	}
	if f.payee.ProcessPaymentCalls != 0 {
		t.Errorf("ProcessPayment should not be called; got %d", f.payee.ProcessPaymentCalls)
	}
	assertErrorCode(t, rr.Body.Bytes(), "missing_or_invalid_payment")
}

func TestPaymentMiddleware_BadBase64(t *testing.T) {
	f := buildFixture(t)
	rr := doPaidRequest(t, f, `{"model":"test-model"}`, "not!base64")
	if rr.Code != nethttp.StatusPaymentRequired {
		t.Fatalf("status: got %d, want 402", rr.Code)
	}
	assertErrorCode(t, rr.Body.Bytes(), "missing_or_invalid_payment")
}

func TestPaymentMiddleware_ProcessPaymentRejected(t *testing.T) {
	f := buildFixture(t)
	f.payee.ProcessPaymentError = errors.New("unknown sender")
	rr := doPaidRequest(t, f, `{"model":"test-model"}`, validHeader())
	if rr.Code != nethttp.StatusPaymentRequired {
		t.Fatalf("status: got %d, want 402", rr.Code)
	}
	if f.payee.DebitBalanceCalls != 0 {
		t.Errorf("DebitBalance should not be called after ProcessPayment failure")
	}
	assertErrorCode(t, rr.Body.Bytes(), "payment_rejected")
}

func TestPaymentMiddleware_ModelExtractionFails(t *testing.T) {
	f := buildFixture(t)
	rr := doPaidRequest(t, f, `not-json`, validHeader())
	if rr.Code != nethttp.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rr.Code)
	}
	assertErrorCode(t, rr.Body.Bytes(), "invalid_request")
	if f.payee.DebitBalanceCalls != 0 {
		t.Errorf("DebitBalance should not be called before route is resolved")
	}
}

func TestPaymentMiddleware_UnknownModel(t *testing.T) {
	f := buildFixture(t)
	rr := doPaidRequest(t, f, `{"model":"not-a-configured-model"}`, validHeader())
	if rr.Code != nethttp.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rr.Code)
	}
	assertErrorCode(t, rr.Body.Bytes(), "capability_not_found")
	if f.payee.DebitBalanceCalls != 0 {
		t.Errorf("DebitBalance should not be called for unknown route")
	}
}

func TestPaymentMiddleware_EstimateError(t *testing.T) {
	f := buildFixture(t)
	f.module.estimateFn = func(body []byte, model types.ModelID) (int64, error) {
		return 0, errors.New("estimator broken")
	}
	rr := doPaidRequest(t, f, `{"model":"test-model"}`, validHeader())
	if rr.Code != nethttp.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rr.Code)
	}
	assertErrorCode(t, rr.Body.Bytes(), "invalid_request")
	if f.payee.DebitBalanceCalls != 0 {
		t.Errorf("DebitBalance must not run when estimate errors")
	}
}

func TestPaymentMiddleware_DebitError(t *testing.T) {
	f := buildFixture(t)
	f.payee.DebitBalanceError = errors.New("daemon hung up")
	rr := doPaidRequest(t, f, `{"model":"test-model"}`, validHeader())
	if rr.Code != nethttp.StatusBadGateway {
		t.Fatalf("status: got %d, want 502", rr.Code)
	}
	assertErrorCode(t, rr.Body.Bytes(), "backend_unavailable")
}

func TestPaymentMiddleware_InsufficientBalance(t *testing.T) {
	f := buildFixture(t)
	// Credit is 0, so estimate-debit 10 → balance -10 → fail.
	f.payee.CreditPerCall.SetInt64(0)
	rr := doPaidRequest(t, f, `{"model":"test-model"}`, validHeader())
	if rr.Code != nethttp.StatusPaymentRequired {
		t.Fatalf("status: got %d, want 402", rr.Code)
	}
	assertErrorCode(t, rr.Body.Bytes(), "insufficient_balance")
}

func TestPaymentMiddleware_ReconcilesWhenActualExceedsEstimate(t *testing.T) {
	f := buildFixture(t)
	f.module.serveFn = func(ctx context.Context, w nethttp.ResponseWriter, r *nethttp.Request, body []byte, model types.ModelID, backendURL string) (int64, error) {
		w.WriteHeader(nethttp.StatusOK)
		return 25, nil // actual > estimate (10)
	}
	rr := doPaidRequest(t, f, `{"model":"test-model"}`, validHeader())
	if rr.Code != nethttp.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	if f.payee.DebitBalanceCalls != 2 {
		t.Errorf("DebitBalance calls: got %d, want 2 (estimate + reconcile)", f.payee.DebitBalanceCalls)
	}
	// Final debit should be the delta = 25 - 10 = 15.
	if f.payee.LastDebitBalanceWorkUnits != 15 {
		t.Errorf("reconcile debit: got %d work units, want 15", f.payee.LastDebitBalanceWorkUnits)
	}
}

func TestPaymentMiddleware_NoReconcileWhenActualLessThanEstimate(t *testing.T) {
	f := buildFixture(t)
	f.module.serveFn = func(ctx context.Context, w nethttp.ResponseWriter, r *nethttp.Request, body []byte, model types.ModelID, backendURL string) (int64, error) {
		w.WriteHeader(nethttp.StatusOK)
		return 3, nil // actual < estimate (10) → over-debit accepted
	}
	rr := doPaidRequest(t, f, `{"model":"test-model"}`, validHeader())
	if rr.Code != nethttp.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	if f.payee.DebitBalanceCalls != 1 {
		t.Errorf("DebitBalance calls: got %d, want 1 (no credit-back in v1)", f.payee.DebitBalanceCalls)
	}
}

func TestPaymentMiddleware_WorkIDStableAcrossIdenticalPayments(t *testing.T) {
	// Same payment blob → same work_id on both ProcessPayment calls.
	f := buildFixture(t)
	_ = doPaidRequest(t, f, `{"model":"test-model"}`, validHeader())
	first := f.payee.LastProcessPaymentWorkID
	_ = doPaidRequest(t, f, `{"model":"test-model"}`, validHeader())
	second := f.payee.LastProcessPaymentWorkID
	if first == "" || first != second {
		t.Errorf("work_id should be stable for identical payment bytes; got %q then %q", first, second)
	}
}

func TestPaymentMiddleware_WorkIDDifferentForDifferentPayments(t *testing.T) {
	f := buildFixture(t)
	_ = doPaidRequest(t, f, `{"model":"test-model"}`, base64.StdEncoding.EncodeToString([]byte("payment-A")))
	first := f.payee.LastProcessPaymentWorkID
	_ = doPaidRequest(t, f, `{"model":"test-model"}`, base64.StdEncoding.EncodeToString([]byte("payment-B")))
	second := f.payee.LastProcessPaymentWorkID
	if first == "" || first == second {
		t.Errorf("work_id should differ for different payment bytes; got %q and %q", first, second)
	}
}

func TestMux_Register_DuplicateRoutePanics(t *testing.T) {
	f := buildFixture(t)
	defer func() {
		if recover() == nil {
			t.Error("duplicate Register should panic")
		}
	}()
	f.mux.Register(nethttp.MethodGet, "/health", func(_ nethttp.ResponseWriter, _ *nethttp.Request) {})
	f.mux.Register(nethttp.MethodGet, "/health", func(_ nethttp.ResponseWriter, _ *nethttp.Request) {})
}

func TestMux_HasPaidCapability(t *testing.T) {
	f := buildFixture(t)
	if !f.mux.HasPaidCapability("openai:/v1/chat/completions") {
		t.Error("HasPaidCapability should report the registered capability")
	}
	if f.mux.HasPaidCapability("openai:/unregistered") {
		t.Error("HasPaidCapability should NOT report unregistered capability")
	}
}

func TestHealthHandler(t *testing.T) {
	f := buildFixture(t)
	RegisterUnpaidHandlers(f.mux, f.cfg)
	req := httptest.NewRequest(nethttp.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	f.mux.Handler().ServeHTTP(rr, req)
	if rr.Code != nethttp.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field: got %v, want ok", body["status"])
	}
	if fmt.Sprintf("%v", body["api_version"]) != fmt.Sprintf("%d", config.CurrentAPIVersion) {
		t.Errorf("api_version: got %v", body["api_version"])
	}
	if fmt.Sprintf("%v", body["protocol_version"]) != fmt.Sprintf("%d", config.CurrentProtocolVersion) {
		t.Errorf("protocol_version: got %v", body["protocol_version"])
	}
}

func TestRegistryOfferingsHandler(t *testing.T) {
	f := buildFixture(t)
	RegisterUnpaidHandlers(f.mux, f.cfg)
	req := httptest.NewRequest(nethttp.MethodGet, "/registry/offerings", nil)
	rr := httptest.NewRecorder()
	f.mux.Handler().ServeHTTP(rr, req)
	if rr.Code != nethttp.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	raw, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(raw), `"worker_eth_address":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`) {
		t.Errorf("registry output missing worker_eth_address: %s", raw)
	}
	if !strings.Contains(string(raw), `"name":"openai:/v1/chat/completions"`) {
		t.Errorf("registry output missing capability name: %s", raw)
	}
	if !strings.Contains(string(raw), `"id":"test-model"`) {
		t.Errorf("registry output missing offering id: %s", raw)
	}
	if !strings.Contains(string(raw), `"offerings":[`) {
		t.Errorf("registry output missing offerings list: %s", raw)
	}
	if !strings.Contains(string(raw), `"extra":{"supports_streaming":true}`) {
		t.Errorf("registry output missing extra object: %s", raw)
	}
	if !strings.Contains(string(raw), `"constraints":{"max_context_tokens":8192}`) {
		t.Errorf("registry output missing constraints object: %s", raw)
	}
	if strings.Contains(string(raw), "backend_url") {
		t.Errorf("registry output MUST NOT include backend_url: %s", raw)
	}
}

func TestRegistryOfferingsHandler_BearerAuth(t *testing.T) {
	f := buildFixture(t)
	f.cfg.AuthToken = "secret-token"
	RegisterUnpaidHandlers(f.mux, f.cfg)

	req := httptest.NewRequest(nethttp.MethodGet, "/registry/offerings", nil)
	rr := httptest.NewRecorder()
	f.mux.Handler().ServeHTTP(rr, req)
	if rr.Code != nethttp.StatusUnauthorized {
		t.Fatalf("unauthenticated status: got %d, want 401", rr.Code)
	}

	req = httptest.NewRequest(nethttp.MethodGet, "/registry/offerings", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rr = httptest.NewRecorder()
	f.mux.Handler().ServeHTTP(rr, req)
	if rr.Code != nethttp.StatusOK {
		t.Fatalf("authenticated status: got %d, want 200", rr.Code)
	}
}

func TestTicketParamsHandler(t *testing.T) {
	f := buildFixture(t)
	f.payee.GetTicketParamsResponse = payeedaemon.TicketParams{
		Recipient:         mustHexBytes(t, "0xd00354656922168815fcd1e51cbddb9e359e3c7f"),
		FaceValueWei:      big.NewInt(1_250_000).Bytes(),
		WinProb:           []byte{0x12, 0x34, 0x56},
		RecipientRandHash: []byte{0xaa, 0xbb, 0xcc},
		Seed:              []byte{0xde, 0xad, 0xbe, 0xef},
		ExpirationBlock:   big.NewInt(9_876_543).Bytes(),
		ExpirationParams: payeedaemon.TicketExpirationParams{
			CreationRound:          4523,
			CreationRoundBlockHash: []byte{0x01, 0x02, 0x03, 0x04},
		},
	}
	RegisterUnpaidHandlers(f.mux, f.cfg)

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/payment/ticket-params", strings.NewReader(`{
		"sender_eth_address":"0x1111111111111111111111111111111111111111",
		"recipient_eth_address":"0xd00354656922168815fcd1e51cbddb9e359e3c7f",
		"face_value_wei":"1250000",
		"capability":"openai:/v1/chat/completions",
		"offering":"test-model"
	}`))
	rr := httptest.NewRecorder()
	f.mux.Handler().ServeHTTP(rr, req)
	if rr.Code != nethttp.StatusOK {
		t.Fatalf("status: got %d, want 200 body=%s", rr.Code, rr.Body.String())
	}
	if f.payee.GetTicketParamsCalls != 1 {
		t.Fatalf("GetTicketParamsCalls: got %d, want 1", f.payee.GetTicketParamsCalls)
	}
	if got := f.payee.LastGetTicketParams.FaceValue.String(); got != "1250000" {
		t.Fatalf("face value: got %s, want 1250000", got)
	}
	if got := f.payee.LastGetTicketParams.Capability; got != "openai:/v1/chat/completions" {
		t.Fatalf("capability: got %q", got)
	}
	if got := f.payee.LastGetTicketParams.Offering; got != "test-model" {
		t.Fatalf("offering: got %q", got)
	}

	var body struct {
		TicketParams struct {
			Recipient         string `json:"recipient"`
			FaceValue         string `json:"face_value"`
			WinProb           string `json:"win_prob"`
			RecipientRandHash string `json:"recipient_rand_hash"`
			Seed              string `json:"seed"`
			ExpirationBlock   string `json:"expiration_block"`
			ExpirationParams  struct {
				CreationRound          int64  `json:"creation_round"`
				CreationRoundBlockHash string `json:"creation_round_block_hash"`
			} `json:"expiration_params"`
		} `json:"ticket_params"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body.TicketParams.Recipient != "0xd00354656922168815fcd1e51cbddb9e359e3c7f" {
		t.Fatalf("recipient: got %q", body.TicketParams.Recipient)
	}
	if body.TicketParams.FaceValue != "1250000" {
		t.Fatalf("face_value: got %q", body.TicketParams.FaceValue)
	}
	if body.TicketParams.WinProb != "0x123456" {
		t.Fatalf("win_prob: got %q", body.TicketParams.WinProb)
	}
	if body.TicketParams.RecipientRandHash != "0xaabbcc" {
		t.Fatalf("recipient_rand_hash: got %q", body.TicketParams.RecipientRandHash)
	}
	if body.TicketParams.Seed != "0xdeadbeef" {
		t.Fatalf("seed: got %q", body.TicketParams.Seed)
	}
	if body.TicketParams.ExpirationBlock != "9876543" {
		t.Fatalf("expiration_block: got %q", body.TicketParams.ExpirationBlock)
	}
	if body.TicketParams.ExpirationParams.CreationRound != 4523 {
		t.Fatalf("creation_round: got %d", body.TicketParams.ExpirationParams.CreationRound)
	}
	if body.TicketParams.ExpirationParams.CreationRoundBlockHash != "0x01020304" {
		t.Fatalf("creation_round_block_hash: got %q", body.TicketParams.ExpirationParams.CreationRoundBlockHash)
	}
}

func TestTicketParamsHandler_BearerAuth(t *testing.T) {
	f := buildFixture(t)
	f.cfg.AuthToken = "secret-token"
	RegisterUnpaidHandlers(f.mux, f.cfg)

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/payment/ticket-params", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	f.mux.Handler().ServeHTTP(rr, req)
	if rr.Code != nethttp.StatusUnauthorized {
		t.Fatalf("unauthenticated status: got %d, want 401", rr.Code)
	}

	req = httptest.NewRequest(nethttp.MethodPost, "/v1/payment/ticket-params", strings.NewReader(`{
		"sender_eth_address":"0x1111111111111111111111111111111111111111",
		"recipient_eth_address":"0xd00354656922168815fcd1e51cbddb9e359e3c7f",
		"face_value_wei":"1250000",
		"capability":"openai:/v1/chat/completions",
		"offering":"test-model"
	}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	rr = httptest.NewRecorder()
	f.mux.Handler().ServeHTTP(rr, req)
	if rr.Code != nethttp.StatusOK {
		t.Fatalf("authenticated status: got %d, want 200 body=%s", rr.Code, rr.Body.String())
	}
}

func TestTicketParamsHandler_BadRequest(t *testing.T) {
	f := buildFixture(t)
	RegisterUnpaidHandlers(f.mux, f.cfg)

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/payment/ticket-params", strings.NewReader(`{
		"sender_eth_address":"0x1111111111111111111111111111111111111111",
		"recipient_eth_address":"0xd00354656922168815fcd1e51cbddb9e359e3c7f",
		"face_value_wei":"abc",
		"capability":"openai:/v1/chat/completions",
		"offering":"test-model"
	}`))
	rr := httptest.NewRecorder()
	f.mux.Handler().ServeHTTP(rr, req)
	if rr.Code != nethttp.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rr.Code)
	}
	assertErrorCode(t, rr.Body.Bytes(), "invalid_request")
	if f.payee.GetTicketParamsCalls != 0 {
		t.Fatalf("GetTicketParamsCalls: got %d, want 0", f.payee.GetTicketParamsCalls)
	}
}

func TestTicketParamsHandler_DaemonUnavailable(t *testing.T) {
	f := buildFixture(t)
	f.payee.GetTicketParamsError = status.Error(codes.Unavailable, "receiver daemon unavailable")
	RegisterUnpaidHandlers(f.mux, f.cfg)

	req := httptest.NewRequest(nethttp.MethodPost, "/v1/payment/ticket-params", strings.NewReader(`{
		"sender_eth_address":"0x1111111111111111111111111111111111111111",
		"recipient_eth_address":"0xd00354656922168815fcd1e51cbddb9e359e3c7f",
		"face_value_wei":"1250000",
		"capability":"openai:/v1/chat/completions",
		"offering":"test-model"
	}`))
	rr := httptest.NewRecorder()
	f.mux.Handler().ServeHTTP(rr, req)
	if rr.Code != nethttp.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", rr.Code)
	}
	assertErrorCode(t, rr.Body.Bytes(), "payment_daemon_unavailable")
}

// assertErrorCode reads the JSON error envelope and asserts the code
// field. Shared helper so each test's failure message points at the
// contract violation cleanly.
func assertErrorCode(t *testing.T, body []byte, wantCode string) {
	t.Helper()
	var env struct {
		Error  string `json:"error"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("error body not JSON: %v (raw=%s)", err, body)
	}
	if env.Error != wantCode {
		t.Errorf("error code: got %q, want %q (detail=%q)", env.Error, wantCode, env.Detail)
	}
}

func mustHexBytes(t *testing.T, s string) []byte {
	t.Helper()
	req, err := parseHexAddress("test", s)
	if err != nil {
		t.Fatalf("parseHexAddress(%q): %v", s, err)
	}
	return req
}
