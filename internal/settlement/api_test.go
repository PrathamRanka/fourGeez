package settlement

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestPaymentDestinationRoutesCreateListAndRead verifies the WAL-001 HTTP contract.
func TestPaymentDestinationRoutesCreateListAndRead(t *testing.T) {
	t.Parallel()

	sellerID := mustSettlementID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	handler := newSettlementHandler(t, sellerID)
	createResponse := performSettlementRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+sellerID.String()+"/payment-destinations",
		"destination-create-1",
		`{"asset":"USDC","network":"eip155:84532","address":"0x1111111111111111111111111111111111111111"}`,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf(
			"create status = %d, body = %s",
			createResponse.Code,
			createResponse.Body.String(),
		)
	}

	var created PaymentDestination
	decodeSettlementResponse(t, createResponse, &created)
	listResponse := performSettlementRequest(
		t,
		handler,
		http.MethodGet,
		"/v1/sellers/"+sellerID.String()+"/payment-destinations",
		"",
		"",
	)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d", listResponse.Code)
	}
	var listed PaymentDestinationListResponse
	decodeSettlementResponse(t, listResponse, &listed)
	if len(listed.Items) != 1 || listed.Items[0] != created {
		t.Fatalf("listed destinations = %#v", listed.Items)
	}

	getResponse := performSettlementRequest(
		t,
		handler,
		http.MethodGet,
		"/v1/sellers/"+sellerID.String()+"/payment-destinations/"+
			created.DestinationID.String(),
		"",
		"",
	)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d", getResponse.Code)
	}
	var loaded PaymentDestination
	decodeSettlementResponse(t, getResponse, &loaded)
	if loaded != created {
		t.Fatalf("loaded destination = %#v, want %#v", loaded, created)
	}
}

// TestPaymentDestinationCreateRejectsUnknownFields verifies strict JSON input.
func TestPaymentDestinationCreateRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	sellerID := mustSettlementID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	handler := newSettlementHandler(t, sellerID)
	response := performSettlementRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+sellerID.String()+"/payment-destinations",
		"destination-create-1",
		`{"asset":"USDC","network":"eip155:84532","address":"0x111","privateKey":"forbidden"}`,
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

// newSettlementHandler creates the WAL-001 API test server.
func newSettlementHandler(t *testing.T, sellerID domain.ID) http.Handler {
	t.Helper()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	service := NewService(
		newTestPaymentDestinationRepository(),
		testSellerAuthorizer{sellerID: sellerID, ownerSubject: "local-seller"},
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("c", 128))),
		clock,
	)
	controller := NewHTTPController(service, newTestIdempotencyStore())
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	return api.Middleware(
		api.Config{
			Authenticator: api.NewStaticAuthenticator("seller-secret", "agent-secret"),
		},
		mux,
	)
}

// performSettlementRequest sends one authenticated HTTP request.
func performSettlementRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	idempotencyKey string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer seller-secret")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// decodeSettlementResponse decodes one JSON response body.
func decodeSettlementResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
	destination any,
) {
	t.Helper()

	if err := json.Unmarshal(response.Body.Bytes(), destination); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
}

type testIdempotencyStore struct {
	records map[string]domain.IdempotencyRecord
}

// newTestIdempotencyStore creates an empty replay-protection fixture.
func newTestIdempotencyStore() *testIdempotencyStore {
	return &testIdempotencyStore{
		records: make(map[string]domain.IdempotencyRecord),
	}
}

// Load returns a prior response for the exact scope and key.
func (store *testIdempotencyStore) Load(
	_ context.Context,
	scope string,
	key domain.IdempotencyKey,
) (domain.IdempotencyRecord, bool, error) {
	record, found := store.records[scope+":"+string(key)]
	return record, found, nil
}

// SaveIfAbsent stores only the first response for a scope and key.
func (store *testIdempotencyStore) SaveIfAbsent(
	_ context.Context,
	record domain.IdempotencyRecord,
) (bool, error) {
	key := record.Scope + ":" + string(record.Key)
	if _, exists := store.records[key]; exists {
		return false, nil
	}
	store.records[key] = record
	return true, nil
}
