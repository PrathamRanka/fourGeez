package payments

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

// TestPaidRouteControllerReturnsChallengeAndDelivery verifies the HTTP flow.
func TestPaidRouteControllerReturnsChallengeAndDelivery(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	transactionRepository := memory.NewTransactionRepository()
	evidenceRepository := memory.NewEvidenceRepository()
	evidenceSigner, err := evidence.NewLocalHMACSigner(
		"local-evidence-key",
		[]byte(strings.Repeat("e", 32)),
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder := evidence.NewRecorder(
		evidenceRepository,
		domain.NewULIDGenerator(
			fixture.clock,
			strings.NewReader(strings.Repeat("r", 256)),
		),
		evidenceSigner,
		fixture.clock,
	)
	service := NewCheckoutService(
		fixture.service,
		NewMockAdapter(),
		transactionRepository,
		recorder,
		&checkoutExecutor{},
		fixture.clock,
	)
	mux := http.NewServeMux()
	NewHTTPController(service).RegisterRoutes(mux)
	handler := api.Middleware(
		api.Config{Authenticator: checkoutAuthenticator{}},
		mux,
	)

	challengeRequest := httptest.NewRequest(
		http.MethodGet,
		"/pay/demo-seller/weather",
		nil,
	)
	challengeRequest.Header.Set(api.AgentKeyHeader, "agent-key")
	challengeRequest.Header.Set(
		intentIDHeader,
		fixture.purchaseIntent.IntentID().String(),
	)
	challengeResponse := httptest.NewRecorder()
	handler.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusPaymentRequired ||
		challengeResponse.Header().Get(paymentRequiredHeader) == "" {
		t.Fatalf(
			"challenge status/header = %d/%q",
			challengeResponse.Code,
			challengeResponse.Header().Get(paymentRequiredHeader),
		)
	}

	paidRequest := httptest.NewRequest(
		http.MethodGet,
		"/pay/demo-seller/weather",
		nil,
	)
	paidRequest.Header.Set(api.AgentKeyHeader, "agent-key")
	paidRequest.Header.Set(
		intentIDHeader,
		fixture.purchaseIntent.IntentID().String(),
	)
	paidRequest.Header.Set(paymentSignatureHeader, MockApprovedProof)
	paidResponse := httptest.NewRecorder()
	handler.ServeHTTP(paidResponse, paidRequest)
	if paidResponse.Code != http.StatusOK ||
		paidResponse.Header().Get(transactionIDHeader) == "" ||
		paidResponse.Header().Get(paymentResponseHeader) == "" {
		t.Fatalf(
			"paid response = %d, headers %#v",
			paidResponse.Code,
			paidResponse.Header(),
		)
	}
}

func TestPaidRouteControllerPublishesRuntimePaymentCapabilities(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	service := NewCheckoutService(
		fixture.service,
		NewX402Adapter(),
		memory.NewTransactionRepository(),
		newCheckoutEvidenceRecorder(t, fixture),
		&checkoutExecutor{},
		fixture.clock,
	)
	mux := http.NewServeMux()
	NewHTTPController(service).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/v1/payment-capabilities", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	var catalog PaymentCapabilityCatalog
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Capabilities) != 1 || catalog.Capabilities[0].Rail != PaymentRailX402 {
		t.Fatalf("catalog = %#v", catalog)
	}
}

func TestPaidRouteControllerMapsInactiveCommerceToGone(t *testing.T) {
	t.Parallel()

	controller := &HTTPController{}
	request := httptest.NewRequest(http.MethodGet, "/pay/demo-seller/weather", nil)
	response := httptest.NewRecorder()
	controller.writeError(response, request, CheckoutResult{}, domain.ErrCommerceUnavailable)
	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assertPaymentErrorCode(t, response, "seller_inactive")
}

func TestPaidRouteControllerMapsReplayToPaymentSpecificConflict(t *testing.T) {
	t.Parallel()

	controller := &HTTPController{}
	request := httptest.NewRequest(http.MethodPost, "/pay/demo-seller/weather", nil)
	response := httptest.NewRecorder()
	controller.writeError(response, request, CheckoutResult{}, ErrPaymentReplay)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	assertPaymentErrorCode(t, response, "payment_replayed")
}

func TestPaidRouteControllerReturnsFieldValidationDetails(t *testing.T) {
	t.Parallel()

	controller := &HTTPController{}
	request := httptest.NewRequest(http.MethodPost, "/pay/demo-seller/weather", nil)
	response := httptest.NewRecorder()
	controller.writeError(response, request, CheckoutResult{}, RequestValidationError{
		Issues: []catalog.InputValidationIssue{{
			Field: "productName", Rule: "required", Message: "productName is required.",
		}},
	})

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body api.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != api.ErrorCodeValidationFailed {
		t.Fatalf("error code = %q", body.Error.Code)
	}
	validationErrors, ok := body.Error.Details["validationErrors"].([]any)
	if !ok || len(validationErrors) != 1 {
		t.Fatalf("validation errors = %#v", body.Error.Details["validationErrors"])
	}
}

func TestPaidRouteControllerLabelsRejectedProofWithoutLosingChallenge(t *testing.T) {
	t.Parallel()

	controller := &HTTPController{}
	request := httptest.NewRequest(http.MethodPost, "/pay/demo-seller/weather", nil)
	response := httptest.NewRecorder()
	controller.writeError(response, request, CheckoutResult{Challenge: &Challenge{Header: "encoded-challenge"}}, ErrPaymentRejected)

	if response.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get(paymentRequiredHeader) != "encoded-challenge" {
		t.Fatalf("PAYMENT-REQUIRED = %q", response.Header().Get(paymentRequiredHeader))
	}
	assertPaymentErrorCode(t, response, "payment_rejected")
	assertPaymentRecoveryAction(t, response, RecoveryActionSignFreshAuthorization)
}

func TestPaidRouteControllerMapsPaymentRecoveryFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		result     CheckoutResult
		status     int
		code       string
		recovery   RecoveryAction
		retryAfter string
	}{
		{name: "expired", err: ErrIntentExpired, status: http.StatusGone, code: "payment_expired", recovery: RecoveryActionStartNewCheckout},
		{name: "unsupported", err: ErrPaymentCapabilityUnsupported, status: http.StatusUnprocessableEntity, code: "payment_capability_unsupported", recovery: RecoveryActionSignFreshAuthorization},
		{name: "verification unavailable", err: ErrPaymentUnavailable, result: CheckoutResult{RecoveryAction: RecoveryActionRetrySameRequest}, status: http.StatusServiceUnavailable, code: "payment_unavailable", recovery: RecoveryActionRetrySameRequest, retryAfter: "2"},
		{name: "settlement unknown", err: ErrPaymentUnavailable, result: CheckoutResult{RecoveryAction: RecoveryActionRetrySamePayment}, status: http.StatusServiceUnavailable, code: "payment_outcome_unknown", recovery: RecoveryActionRetrySamePayment, retryAfter: "2"},
		{name: "settlement rejected", err: ErrPaymentRejected, result: CheckoutResult{RecoveryAction: RecoveryActionStartNewCheckout}, status: http.StatusPaymentRequired, code: "payment_rejected", recovery: RecoveryActionStartNewCheckout},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := &HTTPController{}
			request := httptest.NewRequest(http.MethodPost, "/pay/demo-seller/weather", nil)
			response := httptest.NewRecorder()
			controller.writeError(response, request, test.result, test.err)

			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			assertPaymentErrorCode(t, response, test.code)
			assertPaymentRecoveryAction(t, response, test.recovery)
			if response.Header().Get("Retry-After") != test.retryAfter {
				t.Fatalf("Retry-After = %q", response.Header().Get("Retry-After"))
			}
		})
	}
}

func assertPaymentErrorCode(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()
	var body api.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != expected {
		t.Fatalf("error code = %q, want %q", body.Error.Code, expected)
	}
}

func assertPaymentRecoveryAction(t *testing.T, response *httptest.ResponseRecorder, expected RecoveryAction) {
	t.Helper()
	var body api.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Details["recoveryAction"] != string(expected) {
		t.Fatalf("recoveryAction = %#v, want %q", body.Error.Details["recoveryAction"], expected)
	}
}

type checkoutAuthenticator struct{}

// AuthenticateSeller rejects seller credentials in this agent-only test.
func (checkoutAuthenticator) AuthenticateSeller(
	context.Context,
	string,
) (api.Principal, bool) {
	return api.Principal{}, false
}

// AuthenticateAgent accepts the test key as the fixture buyer.
func (checkoutAuthenticator) AuthenticateAgent(
	_ context.Context,
	key string,
) (api.Principal, bool) {
	return api.Principal{
		Kind:    api.PrincipalAgent,
		Subject: "agent-123",
	}, key == "agent-key"
}
