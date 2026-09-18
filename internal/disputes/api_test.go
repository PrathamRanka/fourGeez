package disputes_test

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
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// TestDisputeRoutesCreateClassifyRetrieveAndReplay verifies API-008.
func TestDisputeRoutesCreateClassifyRetrieveAndReplay(t *testing.T) {
	t.Parallel()

	handler, transaction := newDisputeAPIHandler(t, true)
	requestBody := `{"transactionId":"` + transaction.TransactionID().String() +
		`","reason":"not_delivered","statement":"No response was delivered."}`
	createResponse := performDisputeRequest(
		handler,
		http.MethodPost,
		"/v1/disputes",
		requestBody,
		map[string]string{
			"Content-Type":     "application/json",
			api.AgentKeyHeader: "agent-secret",
			"Idempotency-Key":  "dispute-create-1",
		},
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}
	if createResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("dispute response did not disable caching")
	}
	var created disputes.Dispute
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != disputes.StatusRefundRecommended ||
		created.ClassificationCode != disputes.CodeDeliveryNotConfirmed {
		t.Fatalf("created dispute = %#v", created)
	}

	replayResponse := performDisputeRequest(
		handler,
		http.MethodPost,
		"/v1/disputes",
		requestBody,
		map[string]string{
			"Content-Type":     "application/json",
			api.AgentKeyHeader: "agent-secret",
			"Idempotency-Key":  "dispute-create-1",
		},
	)
	if replayResponse.Code != http.StatusCreated ||
		replayResponse.Body.String() != createResponse.Body.String() {
		t.Fatal("dispute idempotency did not replay the original result")
	}

	getResponse := performDisputeRequest(
		handler,
		http.MethodGet,
		"/v1/disputes/"+created.DisputeID.String(),
		"",
		map[string]string{api.AgentKeyHeader: "agent-secret"},
	)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getResponse.Code, getResponse.Body.String())
	}
}

// TestDisputeRoutesRejectInvalidRequests verifies mutation failure paths.
func TestDisputeRoutesRejectInvalidRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		build      func(http.Handler, transactions.Transaction) *httptest.ResponseRecorder
		seedFailed bool
		wantStatus int
	}{
		{
			name: "unknown field",
			build: func(handler http.Handler, transaction transactions.Transaction) *httptest.ResponseRecorder {
				body := `{"transactionId":"` + transaction.TransactionID().String() + `","reason":"not_delivered","extra":true}`
				return performDisputeRequest(handler, http.MethodPost, "/v1/disputes", body, map[string]string{"Content-Type": "application/json", api.AgentKeyHeader: "agent-secret", "Idempotency-Key": "dispute-create-2"})
			},
			seedFailed: true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing transaction",
			build: func(handler http.Handler, _ transactions.Transaction) *httptest.ResponseRecorder {
				body := `{"transactionId":"txn_01K5D09YJ0C0M7RJM4FWQ0K9H8","reason":"not_delivered"}`
				return performDisputeRequest(handler, http.MethodPost, "/v1/disputes", body, map[string]string{"Content-Type": "application/json", api.AgentKeyHeader: "agent-secret", "Idempotency-Key": "dispute-create-3"})
			},
			seedFailed: true,
			wantStatus: http.StatusNotFound,
		},
		{
			name: "transaction not disputable",
			build: func(handler http.Handler, transaction transactions.Transaction) *httptest.ResponseRecorder {
				body := `{"transactionId":"` + transaction.TransactionID().String() + `","reason":"quality_or_output"}`
				return performDisputeRequest(handler, http.MethodPost, "/v1/disputes", body, map[string]string{"Content-Type": "application/json", api.AgentKeyHeader: "agent-secret", "Idempotency-Key": "dispute-create-4"})
			},
			seedFailed: false,
			wantStatus: http.StatusConflict,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, transaction := newDisputeAPIHandler(t, test.seedFailed)
			response := test.build(handler, transaction)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

// newDisputeAPIHandler creates a seeded API-008 handler.
func newDisputeAPIHandler(
	t *testing.T,
	seedFailed bool,
) (http.Handler, transactions.Transaction) {
	t.Helper()
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	catalogRepository := memory.NewCatalogRepository()
	transactionRepository := memory.NewTransactionRepository()
	disputeRepository := memory.NewDisputeRepository()
	seller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID:        mustDisputeAPIID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		OwnerSubject:    "local-seller",
		Slug:            "demo-shop",
		Name:            "Demo Shop",
		UpstreamBaseURL: "https://seller.example",
		CreatedAt:       domain.NewTimestamp(clock.Now()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogRepository.CreateSeller(t.Context(), seller); err != nil {
		t.Fatal(err)
	}
	transaction, err := transactions.NewTransaction(transactions.TransactionParams{
		TransactionID: mustDisputeAPIID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		IntentID:      mustDisputeAPIID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:      seller.SellerID,
		RouteID:       mustDisputeAPIID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:       "local-agent",
		Amount:        domain.MustParseAmount("35000000"),
		Asset:         "test-usdc",
		Network:       "test-network",
		CreatedAt:     domain.NewTimestamp(clock.Now()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if seedFailed {
		if err := transaction.RequirePayment(domain.NewTimestamp(clock.Now().Add(time.Second))); err != nil {
			t.Fatal(err)
		}
		if err := transaction.VerifyPayment(
			"payment-dispute-test",
			mustDisputeAPIDigest(t, strings.Repeat("a", 64)),
			domain.NewTimestamp(clock.Now().Add(2*time.Second)),
		); err != nil {
			t.Fatal(err)
		}
		if err := transaction.FinalizePayment(
			"payment-dispute-test",
			"0xtestnettransaction",
			domain.NewTimestamp(clock.Now().Add(3*time.Second)),
		); err != nil {
			t.Fatal(err)
		}
		if err := transaction.MarkForwarded(domain.NewTimestamp(clock.Now().Add(4 * time.Second))); err != nil {
			t.Fatal(err)
		}
		if err := transaction.MarkFailed(
			"seller_timeout",
			nil,
			nil,
			domain.NewTimestamp(clock.Now().Add(5*time.Second)),
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := transactionRepository.Create(t.Context(), transaction); err != nil {
		t.Fatal(err)
	}
	clock.Value = clock.Value.Add(time.Minute)
	service := disputes.NewService(
		disputeRepository,
		transactionRepository,
		catalogRepository,
		domain.NewULIDGenerator(
			clock,
			strings.NewReader(strings.Repeat("d", 256)),
		),
		clock,
	)
	controller := disputes.NewHTTPController(
		service,
		memory.NewIdempotencyStore(),
	)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	return api.Middleware(
		api.Config{
			Authenticator: api.NewStaticAuthenticator(
				"seller-secret",
				"agent-secret",
			),
		},
		mux,
	), transaction
}

// performDisputeRequest sends one request through the complete middleware.
func performDisputeRequest(
	handler http.Handler,
	method string,
	path string,
	body string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// mustDisputeAPIID parses a test domain identifier.
func mustDisputeAPIID(
	t *testing.T,
	raw string,
	prefix domain.IDPrefix,
) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}

// mustDisputeAPIDigest parses a test digest.
func mustDisputeAPIDigest(
	t *testing.T,
	raw string,
) intents.SHA256Digest {
	t.Helper()
	digest, err := intents.ParseSHA256Digest(raw)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
