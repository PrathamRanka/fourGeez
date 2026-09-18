package payments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/api"
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

func TestPaidRouteControllerMapsInactiveCommerceToGone(t *testing.T) {
	t.Parallel()

	controller := &HTTPController{}
	request := httptest.NewRequest(http.MethodGet, "/pay/demo-seller/weather", nil)
	response := httptest.NewRecorder()
	controller.writeError(response, request, CheckoutResult{}, domain.ErrCommerceUnavailable)
	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
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
