package intents_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

// TestPurchaseIntentRoutesCreateAndRetrieveImmutableIntent verifies API-004.
func TestPurchaseIntentRoutesCreateAndRetrieveImmutableIntent(t *testing.T) {
	t.Parallel()

	handler, route := newIntentHandler(t)
	requestBody := "{\"routeId\":\"" + route.RouteID.String() +
		"\",\"requestBodyHash\":\"" + strings.Repeat("a", 64) +
		"\",\"maximumAmount\":\"40000000\"}"
	createRequest := httptest.NewRequest(
		http.MethodPost,
		"/v1/intents",
		bytes.NewBufferString(requestBody),
	)
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set(api.AgentKeyHeader, "agent-secret")
	createRequest.Header.Set("Idempotency-Key", "intent-create-1")
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}

	var created intents.Snapshot
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("create response JSON error = %v", err)
	}
	if created.Amount != route.Amount || created.SellerID != route.SellerID {
		t.Fatal("intent did not freeze the authoritative seller quote")
	}

	getRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/intents/"+created.IntentID.String(),
		nil,
	)
	getRequest.Header.Set(api.AgentKeyHeader, "agent-secret")
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getResponse.Code, getResponse.Body.String())
	}
}

// TestPurchaseIntentRejectsMaximumBelowQuote verifies buyer limit enforcement.
func TestPurchaseIntentRejectsMaximumBelowQuote(t *testing.T) {
	t.Parallel()

	handler, route := newIntentHandler(t)
	requestBody := "{\"routeId\":\"" + route.RouteID.String() +
		"\",\"requestBodyHash\":\"" + strings.Repeat("a", 64) +
		"\",\"maximumAmount\":\"1\"}"
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/intents",
		bytes.NewBufferString(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(api.AgentKeyHeader, "agent-secret")
	request.Header.Set("Idempotency-Key", "intent-create-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

// newIntentHandler creates a seeded API-004 test server.
func newIntentHandler(t *testing.T) (http.Handler, catalog.PaidRoute) {
	t.Helper()

	catalogRepository := memory.NewCatalogRepository()
	intentRepository := memory.NewPurchaseIntentRepository()
	idempotencyStore := memory.NewIdempotencyStore()
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	seller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID:        mustIntentAPIID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		OwnerSubject:    "local-seller",
		Slug:            "demo-shop",
		Name:            "Demo",
		UpstreamBaseURL: "https://seller.example",
		CreatedAt:       domain.NewTimestamp(clock.Now()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogRepository.CreateSeller(t.Context(), seller); err != nil {
		t.Fatal(err)
	}
	threshold := domain.MustParseAmount("30000000")
	route, err := catalog.NewPaidRoute(catalog.PaidRouteParams{
		RouteID:                 mustIntentAPIID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		SellerID:                seller.SellerID,
		DisplayName:             "Research Report",
		ProductSlug:             "research-report",
		Method:                  catalog.RouteMethodPost,
		PathPattern:             "/research",
		Description:             "Research",
		MIMEType:                "application/json",
		Amount:                  domain.MustParseAmount("35000000"),
		Asset:                   "test-usdc",
		Network:                 "test-network",
		PayTo:                   "0x123",
		ApprovalThresholdAmount: &threshold,
		UpstreamTimeoutSeconds:  20,
		CreatedAt:               domain.NewTimestamp(clock.Now()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogRepository.CreateRoute(t.Context(), route); err != nil {
		t.Fatal(err)
	}

	service := intents.NewService(
		intentRepository,
		catalogRepository,
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("b", 256))),
		clock,
	)
	controller := intents.NewHTTPController(service, idempotencyStore)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	return api.Middleware(
		api.Config{Authenticator: api.NewStaticAuthenticator("seller-secret", "agent-secret")},
		mux,
	), route
}

// mustIntentAPIID parses a test identifier.
func mustIntentAPIID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
