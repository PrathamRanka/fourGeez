package sellerworkspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/transactions"
)

func TestHTTPControllerUsesInjectedPrincipalAndIgnoresSellerQueryData(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	fixture.routes = []catalog.PaidRoute{fixture.route}
	fixture.destinations = []settlement.PaymentDestination{fixture.destination}
	fixture.credentials = []integrations.CredentialView{fixture.credential}
	fixture.entitlement = fixture.activeEntitlement
	handler := workspaceHandler(fixture.service, principalSource{principal: fixture.principal})

	request := httptest.NewRequest(http.MethodGet, "/v1/me/onboarding?sellerId=sel_01K5D09YJ0C0M7RJM4FWQ0K9ZZ", nil)
	request.Header.Set("Authorization", "Bearer seller-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body OnboardingView
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SellerID == nil || *body.SellerID != fixture.seller.SellerID {
		t.Fatalf("seller = %#v", body.SellerID)
	}
}

func TestHTTPControllerExposesAllCurrentSellerSummaries(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	fixture.entitlement = fixture.activeEntitlement
	handler := workspaceHandler(fixture.service, principalSource{principal: fixture.principal})
	paths := []string{
		"/v1/me/dashboard", "/v1/me/dashboard/products", "/v1/me/dashboard/transactions",
		"/v1/me/dashboard/evidence", "/v1/me/dashboard/webhooks", "/v1/me/dashboard/billing",
		"/v1/me/dashboard/credentials", "/v1/me/settings",
	}
	for _, path := range paths {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer seller-token")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, body = %s", path, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("GET %s missing no-store", path)
		}
	}
}

func TestHTTPControllerRejectsMissingPrincipal(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	handler := workspaceHandler(fixture.service, principalSource{err: ErrAuthenticationRequired})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/me/dashboard", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
	var body api.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != api.ErrorCodeUnauthorized {
		t.Fatalf("code = %s", body.Error.Code)
	}
}

func TestHTTPControllerUpdatesSettingsWithStrictJSON(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	handler := workspaceHandler(fixture.service, principalSource{principal: fixture.principal})
	request := httptest.NewRequest(http.MethodPatch, "/v1/me/settings", strings.NewReader(`{"supportEmail":"support@example.com","securityNotificationEmail":"security@example.com","webhookFailureNotifications":true,"expectedVersion":1}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer seller-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	unknown := httptest.NewRequest(http.MethodPatch, "/v1/me/settings", strings.NewReader(`{"expectedVersion":2,"unknown":true}`))
	unknown.Header.Set("Content-Type", "application/json")
	unknown.Header.Set("Authorization", "Bearer seller-token")
	unknownResponse := httptest.NewRecorder()
	handler.ServeHTTP(unknownResponse, unknown)
	if unknownResponse.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d", unknownResponse.Code)
	}
}

func TestHTTPControllerCreatesBillingPortalSession(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	handler := workspaceHandler(fixture.service, principalSource{principal: fixture.principal})
	request := httptest.NewRequest(http.MethodPost, "/v1/me/billing/portal-sessions", strings.NewReader(`{"returnUrl":"https://app.agentpay.example/dashboard/billing"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer seller-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("portal response must not be cached")
	}
}

func TestHTTPControllerRecordsAuthoritativeSandboxPurchase(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	fixture.routes = []catalog.PaidRoute{fixture.route}
	fixture.destinations = []settlement.PaymentDestination{fixture.destination}
	fixture.credentials = []integrations.CredentialView{fixture.credential}
	fixture.transactions = []transactions.Transaction{fixture.transaction}
	fixture.entitlement = fixture.activeEntitlement
	handler := workspaceHandler(fixture.service, principalSource{principal: fixture.principal})

	request := httptest.NewRequest(http.MethodPost, "/v1/me/onboarding/sandbox-purchases", strings.NewReader(`{"transactionId":"`+fixture.transaction.TransactionID().String()+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer seller-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("sandbox purchase response must not be cached")
	}
	var body OnboardingView
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	assertStep(t, body, StepSandboxPurchase, StepComplete)
}

func TestHTTPControllerRejectsInvalidSandboxPurchase(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	handler := workspaceHandler(fixture.service, principalSource{principal: fixture.principal})

	request := httptest.NewRequest(http.MethodPost, "/v1/me/onboarding/sandbox-purchases", strings.NewReader(`{"transactionId":"`+fixture.transaction.TransactionID().String()+`","unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer seller-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/v1/me/onboarding/sandbox-purchases", strings.NewReader(`{"transactionId":"`+fixture.transaction.TransactionID().String()+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer seller-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("non-authoritative transaction status = %d, body = %s", response.Code, response.Body.String())
	}
}

type principalSource struct {
	principal Principal
	err       error
}

func (source principalSource) CurrentPrincipal(context.Context) (Principal, error) {
	return source.principal, source.err
}

func workspaceHandler(service *Service, principal AuthenticatedPrincipal) http.Handler {
	mux := http.NewServeMux()
	NewHTTPController(service, principal).RegisterRoutes(mux)
	return api.Middleware(api.Config{Authenticator: api.NewStaticAuthenticator("seller-token", "")}, mux)
}
