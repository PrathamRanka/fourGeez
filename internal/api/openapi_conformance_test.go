package api_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/realtime"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// TestOpenAPIM3OperationAndResponseCoverage locks every M3 contract response.
func TestOpenAPIM3OperationAndResponseCoverage(t *testing.T) {
	t.Parallel()

	operations := readOpenAPIOperations(t)
	expected := map[string][]string{
		"getHealth":                   {"200", "429", "503"},
		"createSeller":                {"201", "400", "409"},
		"createPaidRoute":             {"201", "400", "404", "409"},
		"updatePaidRoutePrice":        {"200", "400", "404", "409"},
		"listPaymentDestinations":     {"200", "400", "404"},
		"createPaymentDestination":    {"201", "400", "404", "409"},
		"getPaymentDestination":       {"200", "400", "404"},
		"listSellerTransactions":      {"200", "404"},
		"listIntegrationCredentials":  {"200", "400", "404"},
		"createIntegrationCredential": {"201", "400", "404", "409"},
		"revokeIntegrationCredential": {"200", "400", "404", "409"},
		"createPurchaseIntent":        {"201", "400", "409"},
		"getPurchaseIntent":           {"200", "404"},
		"createApprovalSession":       {"201", "409", "422"},
		"getApprovalSession":          {"200", "404"},
		"decideApproval":              {"200", "409", "410"},
		"getTransaction":              {"200", "404"},
		"createDispute":               {"201", "404", "409"},
		"getDispute":                  {"200", "404"},
		"getStorefrontManifest":       {"200", "404"},
		"getStorefrontLlmsText":       {"200", "404"},
	}
	delete(operations, "getPaidResource")
	delete(operations, "postPaidResource")
	if !reflect.DeepEqual(operations, expected) {
		t.Fatalf("M3 OpenAPI operation coverage = %#v, want %#v", operations, expected)
	}
}

// TestM3OpenAPIRoutesAreRegistered verifies every documented M3 route exists.
func TestM3OpenAPIRoutesAreRegistered(t *testing.T) {
	t.Parallel()

	handler := newConformanceHandler(t)
	tests := []struct {
		operationID string
		method      string
		path        string
		wantStatus  int
	}{
		{operationID: "getHealth", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK},
		{operationID: "createSeller", method: http.MethodPost, path: "/v1/sellers", wantStatus: http.StatusUnauthorized},
		{operationID: "createPaidRoute", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes", wantStatus: http.StatusUnauthorized},
		{operationID: "updatePaidRoutePrice", method: http.MethodPatch, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes/rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", wantStatus: http.StatusUnauthorized},
		{operationID: "listPaymentDestinations", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/payment-destinations", wantStatus: http.StatusUnauthorized},
		{operationID: "createPaymentDestination", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/payment-destinations", wantStatus: http.StatusUnauthorized},
		{operationID: "getPaymentDestination", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/payment-destinations/dst_01K5D09YJ0C0M7RJM4FWQ0K9H8", wantStatus: http.StatusUnauthorized},
		{operationID: "listSellerTransactions", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/transactions", wantStatus: http.StatusUnauthorized},
		{operationID: "listIntegrationCredentials", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/integration-credentials", wantStatus: http.StatusUnauthorized},
		{operationID: "createIntegrationCredential", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/integration-credentials", wantStatus: http.StatusUnauthorized},
		{operationID: "revokeIntegrationCredential", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/integration-credentials/key_01K5D09YJ0C0M7RJM4FWQ0K9H8/revoke", wantStatus: http.StatusUnauthorized},
		{operationID: "createPurchaseIntent", method: http.MethodPost, path: "/v1/intents", wantStatus: http.StatusUnauthorized},
		{operationID: "getPurchaseIntent", method: http.MethodGet, path: "/v1/intents/int_01K5D09YJ0C0M7RJM4FWQ0K9H7", wantStatus: http.StatusUnauthorized},
		{operationID: "createApprovalSession", method: http.MethodPost, path: "/v1/intents/int_01K5D09YJ0C0M7RJM4FWQ0K9H7/approval-sessions", wantStatus: http.StatusUnauthorized},
		{operationID: "getApprovalSession", method: http.MethodGet, path: "/v1/approval-sessions/aps_01K5D09YJ0C0M7RJM4FWQ0K9H7", wantStatus: http.StatusNotFound},
		{operationID: "decideApproval", method: http.MethodPost, path: "/v1/approval-sessions/aps_01K5D09YJ0C0M7RJM4FWQ0K9H7/decisions", wantStatus: http.StatusUnauthorized},
		{operationID: "getTransaction", method: http.MethodGet, path: "/v1/transactions/txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", wantStatus: http.StatusUnauthorized},
		{operationID: "createDispute", method: http.MethodPost, path: "/v1/disputes", wantStatus: http.StatusUnauthorized},
		{operationID: "getDispute", method: http.MethodGet, path: "/v1/disputes/dsp_01K5D09YJ0C0M7RJM4FWQ0K9H7", wantStatus: http.StatusUnauthorized},
		{operationID: "getStorefrontManifest", method: http.MethodGet, path: "/store/missing/manifest.json", wantStatus: http.StatusNotFound},
		{operationID: "getStorefrontLlmsText", method: http.MethodGet, path: "/store/missing/llms.txt", wantStatus: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.operationID, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantStatus >= 400 && !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("error content type = %q", response.Header().Get("Content-Type"))
			}
		})
	}
}

// TestDocumentedErrorEnvelopeConforms verifies every M3 error status shape.
func TestDocumentedErrorEnvelopeConforms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		code   string
	}{
		{status: http.StatusBadRequest, code: api.ErrorCodeBadRequest},
		{status: http.StatusUnauthorized, code: api.ErrorCodeUnauthorized},
		{status: http.StatusNotFound, code: api.ErrorCodeNotFound},
		{status: http.StatusConflict, code: api.ErrorCodeConflict},
		{status: http.StatusGone, code: api.ErrorCodeGone},
		{status: http.StatusUnprocessableEntity, code: api.ErrorCodeUnprocessable},
		{status: http.StatusTooManyRequests, code: "rate_limited"},
		{status: http.StatusServiceUnavailable, code: "dependency_unavailable"},
	}

	for _, test := range tests {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/health", nil)
		handler := api.Middleware(
			api.Config{},
			http.HandlerFunc(func(
				response http.ResponseWriter,
				request *http.Request,
			) {
				api.WriteError(
					response,
					request,
					test.status,
					test.code,
					"contract test",
					nil,
				)
			}),
		)
		handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("status = %d, want %d", response.Code, test.status)
		}
		var envelope api.ErrorResponse
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Error.Code != test.code ||
			envelope.Error.Message == "" ||
			envelope.Error.RequestID == "" {
			t.Fatalf("error envelope = %#v", envelope)
		}
	}
}

// newConformanceHandler composes all M3 HTTP and WebSocket routes.
func newConformanceHandler(t *testing.T) http.Handler {
	t.Helper()
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	catalogRepository := memory.NewCatalogRepository()
	intentRepository := memory.NewPurchaseIntentRepository()
	approvalRepository := memory.NewApprovalRepository()
	transactionRepository := memory.NewTransactionRepository()
	evidenceRepository := memory.NewEvidenceRepository()
	disputeRepository := memory.NewDisputeRepository()
	integrationCredentialRepository := memory.NewIntegrationCredentialRepository()
	idempotencyStore := memory.NewIdempotencyStore()
	idGenerator := domain.NewULIDGenerator(
		clock,
		strings.NewReader(strings.Repeat("c", 2048)),
	)
	approvalSigner, err := approvals.NewApprovalTokenSigner(
		[]byte(strings.Repeat("a", 32)),
	)
	if err != nil {
		t.Fatal(err)
	}
	evidenceSigner, err := evidence.NewLocalHMACSigner(
		"conformance-key-v1",
		[]byte(strings.Repeat("e", 32)),
	)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(response http.ResponseWriter, _ *http.Request) {
		_ = api.WriteJSON(response, http.StatusOK, map[string]string{"status": "ok"})
	})
	catalogService := catalog.NewService(catalogRepository, idGenerator, clock)
	catalog.NewHTTPController(catalogService, idempotencyStore).RegisterRoutes(mux)
	settlementService := settlement.NewService(
		memory.NewPaymentDestinationRepository(),
		catalogService,
		idGenerator,
		clock,
	)
	settlement.NewHTTPController(
		settlementService,
		idempotencyStore,
	).RegisterRoutes(mux)
	integrationService := integrations.NewService(
		integrationCredentialRepository,
		catalogService,
		idGenerator,
		integrations.NewSecureTokenGenerator(
			strings.NewReader(strings.Repeat("k", 512)),
		),
		clock,
	)
	integrations.NewHTTPController(
		integrationService,
		idempotencyStore,
	).RegisterRoutes(mux)
	intentService := intents.NewService(
		intentRepository,
		catalogRepository,
		idGenerator,
		clock,
	)
	intents.NewHTTPController(intentService, idempotencyStore).RegisterRoutes(mux)
	approvalService := approvals.NewService(
		approvalRepository,
		intentRepository,
		idGenerator,
		approvals.NewSecureTokenGenerator(
			strings.NewReader(strings.Repeat("t", 512)),
		),
		approvalSigner,
		clock,
		"http://localhost:3000",
	)
	hub := realtime.NewLocalHub()
	realtimeService := realtime.NewService(
		hub,
		approvalService,
		hub,
		idGenerator,
		clock,
	)
	approvalService.SetEventPublisher(realtimeService)
	approvals.NewHTTPController(
		approvalService,
		idempotencyStore,
	).RegisterRoutes(mux)
	realtime.NewHTTPController(
		realtime.NewController(realtimeService),
		hub,
	).RegisterRoutes(mux)
	transactionService := transactions.NewService(
		transactionRepository,
		evidenceRepository,
		evidenceSigner,
		catalogRepository,
	)
	transactions.NewHTTPController(transactionService).RegisterRoutes(mux)
	disputeService := disputes.NewService(
		disputeRepository,
		transactionRepository,
		catalogRepository,
		idGenerator,
		clock,
	)
	disputes.NewHTTPController(
		disputeService,
		idempotencyStore,
	).RegisterRoutes(mux)
	return api.Middleware(
		api.Config{
			Authenticator: api.NewStaticAuthenticator(
				"seller-secret",
				"agent-secret",
			),
		},
		mux,
	)
}

// readOpenAPIOperations extracts operation identifiers and response statuses.
func readOpenAPIOperations(t *testing.T) map[string][]string {
	t.Helper()
	contractPath := filepath.Join("..", "..", "docs", "api", "openapi.yaml")
	contract, err := os.Open(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	defer contract.Close()

	operationPattern := regexp.MustCompile(`^      operationId: ([A-Za-z0-9]+)$`)
	statusPattern := regexp.MustCompile(`^        '([0-9]{3})':`)
	operations := make(map[string][]string)
	currentOperation := ""
	scanner := bufio.NewScanner(contract)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := operationPattern.FindStringSubmatch(line); matches != nil {
			currentOperation = matches[1]
			operations[currentOperation] = []string{}
			continue
		}
		if currentOperation == "" {
			continue
		}
		if matches := statusPattern.FindStringSubmatch(line); matches != nil {
			operations[currentOperation] = append(
				operations[currentOperation],
				matches[1],
			)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for operationID := range operations {
		sort.Strings(operations[operationID])
	}
	return operations
}
