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

	"github.com/fourgeez/agentpay/internal/analytics"
	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/identity"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/payments"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/sellerworkspace"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/storefront"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// TestOpenAPILaunchTargetOperationAndResponseCoverage locks the complete
// development-preview target contract.
func TestOpenAPILaunchTargetOperationAndResponseCoverage(t *testing.T) {
	t.Parallel()

	operations := readOpenAPIOperations(t)
	expected := map[string][]string{
		"acceptStripeBillingEvent":                {"204", "400", "401", "409", "429", "503"},
		"activateSellerService":                   {"200", "400", "401", "404", "409", "429", "503"},
		"archivePaidRoute":                        {"200", "400", "401", "403", "404", "409", "429", "503"},
		"cancelPurchaseIntent":                    {"200", "400", "401", "403", "404", "409", "410", "429", "503"},
		"createBrowserPurchaseRecoveryChallenge":  {"201", "400", "404", "409", "410", "422", "429", "503"},
		"createBrowserPurchaseSession":            {"201", "400", "404", "409", "410", "422", "429", "503"},
		"createCurrentSellerBillingPortalSession": {"201", "400", "401", "404", "429", "503"},
		"createDispute":                           {"201", "400", "401", "403", "404", "409", "410", "422", "429", "503"},
		"createIntegrationCredential":             {"201", "400", "401", "403", "404", "409", "429", "503"},
		"createManualRefundRecord":                {"201", "400", "401", "403", "404", "409", "422", "429", "503"},
		"createMcpConfirmationGrant":              {"201", "400", "401", "403", "404", "409", "422", "429", "503"},
		"createPaidRoute":                         {"201", "400", "401", "403", "404", "409", "429", "503"},
		"createPaymentDestination":                {"201", "400", "401", "403", "404", "409", "429", "503"},
		"createPaymentDestinationChallenge":       {"201", "400", "401", "403", "404", "409", "429", "503"},
		"createPurchaseIntent":                    {"201", "400", "401", "403", "404", "409", "410", "422", "429", "503"},
		"createSeller":                            {"201", "400", "401", "403", "409", "429", "503"},
		"createWebhookSubscription":               {"201", "400", "401", "403", "404", "409", "429", "503"},
		"downloadPurchaseReceipt":                 {"200", "401", "403", "404", "409", "429", "503"},
		"emergencyDisablePaidRoute":               {"200", "400", "401", "403", "404", "409", "429", "503"},
		"exchangeProjectKey":                      {"200", "400", "401", "403", "429", "503"},
		"getAgentPayManifest":                     {"200", "429", "503"},
		"getCapabilityJwks":                       {"200", "429", "503"},
		"getCurrentSeller":                        {"200", "401", "404", "429", "503"},
		"getCurrentSellerBillingSummary":          {"200", "401", "404", "429", "503"},
		"getCurrentSellerCredentialSummary":       {"200", "401", "404", "429", "503"},
		"getCurrentSellerDashboard":               {"200", "401", "404", "429", "503"},
		"getCurrentSellerEvidenceSummary":         {"200", "401", "404", "429", "503"},
		"getCurrentSellerOnboarding":              {"200", "401", "404", "429", "503"},
		"getCurrentSellerProductSummary":          {"200", "401", "404", "429", "503"},
		"getCurrentSellerSettings":                {"200", "401", "404", "429", "503"},
		"getCurrentSellerTransactionSummary":      {"200", "401", "404", "429", "503"},
		"getCurrentSellerWebhookSummary":          {"200", "401", "404", "429", "503"},
		"getDispute":                              {"200", "401", "403", "404", "429", "503"},
		"getHealth":                               {"200", "429", "503"},
		"getManualRefundRecord":                   {"200", "401", "403", "404", "429", "503"},
		"getPaidResource":                         {"200", "400", "401", "402", "403", "404", "409", "410", "422", "429", "503"},
		"getPaymentCapabilities":                  {"200", "429"},
		"detectExternalBuyerCompatibility":        {"200", "400", "429"},
		"getPaidRoute":                            {"200", "400", "401", "403", "404", "429", "503"},
		"getPaymentDestination":                   {"200", "400", "401", "403", "404", "429", "503"},
		"getPublicProduct":                        {"200", "404", "410", "429", "503"},
		"getPurchaseIntent":                       {"200", "401", "403", "404", "429", "503"},
		"getSellerDashboardSummary":               {"200", "400", "401", "403", "404", "422", "429", "503"},
		"getSellerInvoiceExport":                  {"200", "400", "401", "403", "404", "422", "429", "503"},
		"getSellerPlan":                           {"200", "400", "401", "403", "404", "429", "503"},
		"getStorefrontLlmsText":                   {"200", "404", "410", "429", "503"},
		"getStorefrontManifest":                   {"200", "404", "410", "429", "503"},
		"getTransaction":                          {"200", "401", "403", "404", "429", "503"},
		"listIntegrationCredentials":              {"200", "400", "401", "403", "404", "429", "503"},
		"listPublicProducts":                      {"200", "400", "429", "503"},
		"listPaidRoutes":                          {"200", "400", "401", "403", "404", "429", "503"},
		"listPaymentDestinations":                 {"200", "400", "401", "403", "404", "429", "503"},
		"listSellerAuditEvents":                   {"200", "400", "401", "403", "404", "429", "503"},
		"listSellerPlans":                         {"200", "429", "503"},
		"listSellerTransactions":                  {"200", "400", "401", "403", "404", "429", "503"},
		"listWebhookDeliveries":                   {"200", "400", "401", "403", "404", "429", "503"},
		"listWebhookSubscriptions":                {"200", "400", "401", "403", "404", "429", "503"},
		"pausePaidRoute":                          {"200", "400", "401", "403", "404", "409", "429", "503"},
		"postPaidResource":                        {"200", "400", "401", "402", "403", "404", "409", "410", "422", "429", "503"},
		"publishPaidRoute":                        {"200", "400", "401", "403", "404", "409", "422", "429", "503"},
		"recoverBrowserPurchase":                  {"204", "400", "401", "404", "409", "410", "422", "429", "503"},
		"redeliverWebhookDelivery":                {"200", "400", "401", "403", "404", "409", "429", "503"},
		"revokeCurrentSellerSession":              {"204", "401", "429", "503"},
		"revokeIntegrationCredential":             {"200", "400", "401", "403", "404", "409", "429", "503"},
		"rotateIntegrationCredential":             {"201", "400", "401", "403", "404", "409", "429", "503"},
		"updatePaidRouteDraft":                    {"200", "400", "401", "403", "404", "409", "429", "503"},
		"updateCurrentSellerSettings":             {"200", "400", "401", "404", "409", "429", "503"},
		"validatePaidRoute":                       {"200", "400", "401", "403", "404", "429", "503"},
		"verifyPaymentDestination":                {"200", "400", "401", "403", "404", "409", "410", "422", "429", "503"},
	}
	if !reflect.DeepEqual(operations, expected) {
		t.Fatalf("launch target OpenAPI operation coverage = %#v, want %#v", operations, expected)
	}
}

// TestImplementedOpenAPIRoutesAreRegistered verifies that active contract
// routes are present in the composed development-preview HTTP handler.
func TestImplementedOpenAPIRoutesAreRegistered(t *testing.T) {
	t.Parallel()

	handler := newConformanceHandler(t)
	tests := []struct {
		operationID string
		method      string
		path        string
		wantStatus  int
	}{
		{operationID: "getHealth", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK},
		{operationID: "getPaymentCapabilities", method: http.MethodGet, path: "/v1/payment-capabilities", wantStatus: http.StatusOK},
		{operationID: "detectExternalBuyerCompatibility", method: http.MethodPost, path: "/v1/payment-capabilities/compatibility", wantStatus: http.StatusBadRequest},
		{operationID: "createSeller", method: http.MethodPost, path: "/v1/sellers", wantStatus: http.StatusUnauthorized},
		{operationID: "activateSellerService", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/service-activation", wantStatus: http.StatusUnauthorized},
		{operationID: "getCurrentSeller", method: http.MethodGet, path: "/v1/me/seller", wantStatus: http.StatusUnauthorized},
		{operationID: "revokeCurrentSellerSession", method: http.MethodDelete, path: "/v1/me/session", wantStatus: http.StatusUnauthorized},
		{operationID: "listSellerPlans", method: http.MethodGet, path: "/v1/plans", wantStatus: http.StatusOK},
		{operationID: "getSellerPlan", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/plan", wantStatus: http.StatusUnauthorized},
		{operationID: "getSellerInvoiceExport", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/invoice-export?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z", wantStatus: http.StatusUnauthorized},
		{operationID: "createPaidRoute", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes", wantStatus: http.StatusUnauthorized},
		{operationID: "listPaidRoutes", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes", wantStatus: http.StatusUnauthorized},
		{operationID: "getPaidRoute", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes/rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", wantStatus: http.StatusUnauthorized},
		{operationID: "updatePaidRouteDraft", method: http.MethodPatch, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes/rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", wantStatus: http.StatusUnauthorized},
		{operationID: "validatePaidRoute", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes/rte_01K5D09YJ0C0M7RJM4FWQ0K9H7/validation", wantStatus: http.StatusUnauthorized},
		{operationID: "publishPaidRoute", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes/rte_01K5D09YJ0C0M7RJM4FWQ0K9H7/publish", wantStatus: http.StatusUnauthorized},
		{operationID: "pausePaidRoute", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes/rte_01K5D09YJ0C0M7RJM4FWQ0K9H7/pause", wantStatus: http.StatusUnauthorized},
		{operationID: "archivePaidRoute", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes/rte_01K5D09YJ0C0M7RJM4FWQ0K9H7/archive", wantStatus: http.StatusUnauthorized},
		{operationID: "emergencyDisablePaidRoute", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/routes/rte_01K5D09YJ0C0M7RJM4FWQ0K9H7/emergency-disable", wantStatus: http.StatusUnauthorized},
		{operationID: "listPaymentDestinations", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/payment-destinations", wantStatus: http.StatusUnauthorized},
		{operationID: "createPaymentDestination", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/payment-destinations", wantStatus: http.StatusUnauthorized},
		{operationID: "getPaymentDestination", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/payment-destinations/dst_01K5D09YJ0C0M7RJM4FWQ0K9H8", wantStatus: http.StatusUnauthorized},
		{operationID: "createPaymentDestinationChallenge", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/payment-destinations/dst_01K5D09YJ0C0M7RJM4FWQ0K9H8/ownership-challenges", wantStatus: http.StatusUnauthorized},
		{operationID: "verifyPaymentDestination", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/payment-destinations/dst_01K5D09YJ0C0M7RJM4FWQ0K9H8/verify", wantStatus: http.StatusUnauthorized},
		{operationID: "listWebhookSubscriptions", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/webhook-subscriptions", wantStatus: http.StatusUnauthorized},
		{operationID: "createWebhookSubscription", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/webhook-subscriptions", wantStatus: http.StatusUnauthorized},
		{operationID: "listWebhookDeliveries", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/webhook-deliveries", wantStatus: http.StatusUnauthorized},
		{operationID: "redeliverWebhookDelivery", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/webhook-deliveries/whd_01K5D09YJ0C0M7RJM4FWQ0K9H7/redeliver", wantStatus: http.StatusUnauthorized},
		{operationID: "listSellerTransactions", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/transactions", wantStatus: http.StatusUnauthorized},
		{operationID: "getSellerDashboardSummary", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/dashboard-summary?from=2026-09-01T00:00:00Z&to=2026-09-17T23:59:59Z", wantStatus: http.StatusUnauthorized},
		{operationID: "listIntegrationCredentials", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/integration-credentials", wantStatus: http.StatusUnauthorized},
		{operationID: "createIntegrationCredential", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/integration-credentials", wantStatus: http.StatusUnauthorized},
		{operationID: "revokeIntegrationCredential", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/integration-credentials/key_01K5D09YJ0C0M7RJM4FWQ0K9H8/revoke", wantStatus: http.StatusUnauthorized},
		{operationID: "createPurchaseIntent", method: http.MethodPost, path: "/v1/intents", wantStatus: http.StatusUnauthorized},
		{operationID: "cancelPurchaseIntent", method: http.MethodPost, path: "/v1/intents/int_01K5D09YJ0C0M7RJM4FWQ0K9H7/cancel", wantStatus: http.StatusUnauthorized},
		{operationID: "getPurchaseIntent", method: http.MethodGet, path: "/v1/intents/int_01K5D09YJ0C0M7RJM4FWQ0K9H7", wantStatus: http.StatusUnauthorized},
		{operationID: "getTransaction", method: http.MethodGet, path: "/v1/transactions/txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", wantStatus: http.StatusUnauthorized},
		{operationID: "downloadPurchaseReceipt", method: http.MethodGet, path: "/v1/transactions/txn_01K5D09YJ0C0M7RJM4FWQ0K9H7/receipt", wantStatus: http.StatusUnauthorized},
		{operationID: "createDispute", method: http.MethodPost, path: "/v1/disputes", wantStatus: http.StatusUnauthorized},
		{operationID: "getDispute", method: http.MethodGet, path: "/v1/disputes/dsp_01K5D09YJ0C0M7RJM4FWQ0K9H7", wantStatus: http.StatusUnauthorized},
		{operationID: "createManualRefundRecord", method: http.MethodPost, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/disputes/dsp_01K5D09YJ0C0M7RJM4FWQ0K9H7/refund-records", wantStatus: http.StatusUnauthorized},
		{operationID: "getManualRefundRecord", method: http.MethodGet, path: "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/disputes/dsp_01K5D09YJ0C0M7RJM4FWQ0K9H7/refund-records/current", wantStatus: http.StatusUnauthorized},
		{operationID: "getAgentPayManifest", method: http.MethodGet, path: "/.well-known/agentpay", wantStatus: http.StatusOK},
		{operationID: "listPublicProducts", method: http.MethodGet, path: "/v1/discovery/products", wantStatus: http.StatusOK},
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

// newConformanceHandler composes the active Lean V1 HTTP routes.
func newConformanceHandler(t *testing.T) http.Handler {
	t.Helper()
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	catalogRepository := memory.NewCatalogRepository()
	intentRepository := memory.NewPurchaseIntentRepository()
	transactionRepository := memory.NewTransactionRepository()
	evidenceRepository := memory.NewEvidenceRepository()
	disputeRepository := memory.NewDisputeRepository()
	manualRefundRecordRepository := memory.NewManualRefundRecordRepository()
	integrationCredentialRepository := memory.NewIntegrationCredentialRepository()
	paymentDestinationRepository := memory.NewPaymentDestinationRepository()
	sellerWorkspaceRepository := memory.NewSellerWorkspaceRepository()
	webhookSubscriptionRepository := memory.NewWebhookSubscriptionRepository()
	webhookDeliveryRepository := memory.NewWebhookDeliveryRepository()
	sellerPlanRepository := memory.NewSellerPlanRepository()
	usageMeterEventRepository := memory.NewUsageMeterEventRepository()
	auditEventRepository := memory.NewAuditEventRepository()
	webhookSecretStore := memory.NewWebhookSecretStore()
	idempotencyStore := memory.NewIdempotencyStore()
	idGenerator := domain.NewULIDGenerator(
		clock,
		strings.NewReader(strings.Repeat("c", 2048)),
	)
	auditAppender := audit.NewAppender(
		auditEventRepository,
		idGenerator,
		clock,
	)
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
	payments.NewCapabilityHTTPController(payments.NewMockAdapter()).RegisterRoutes(mux)
	catalogService := catalog.NewService(
		catalogRepository,
		idGenerator,
		clock,
		auditAppender,
	)
	catalog.NewHTTPController(catalogService, idempotencyStore).RegisterControlRoutes(mux)
	localIdentity := identity.NewLocalAdapter(clock)
	if err := localIdentity.Register("seller-secret", identity.Claims{
		Subject: "local-seller", TokenID: "conformance-token", SessionID: "conformance-session",
		ExpiresAt: clock.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	identity.NewHTTPController(
		identity.NewService(localIdentity, memory.NewSellerSessionRevocationRepository(), clock),
		catalogService,
	).RegisterRoutes(mux)
	audit.NewHTTPController(
		audit.NewService(
			auditEventRepository,
			catalogService,
			idGenerator,
			clock,
		),
	).RegisterRoutes(mux)
	billingService := billing.NewService(
		sellerPlanRepository,
		catalogService,
		clock,
	)
	billing.NewHTTPController(billingService).RegisterRoutes(mux)
	billing.NewUsageHTTPController(
		billing.NewUsageService(
			usageMeterEventRepository,
			billingService,
			transactionRepository,
			catalogService,
			idGenerator,
			clock,
		),
	).RegisterRoutes(mux)
	settlementService := settlement.NewService(
		paymentDestinationRepository,
		catalogService,
		idGenerator,
		settlement.NewSecureOwnershipNonceGenerator(
			strings.NewReader(strings.Repeat("n", 512)),
		),
		settlement.NewEVMPersonalSignOwnershipVerifier(),
		clock,
		auditAppender,
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
		auditAppender,
	)
	integrations.NewHTTPController(
		integrationService,
		idempotencyStore,
	).RegisterRoutes(mux)
	notifications.NewHTTPController(
		notifications.NewService(
			webhookSubscriptionRepository,
			catalogService,
			idGenerator,
			notifications.NewSecureSecretGenerator(
				strings.NewReader(strings.Repeat("w", 512)),
			),
			webhookSecretStore,
			clock,
			auditAppender,
		),
		idempotencyStore,
	).RegisterRoutes(mux)
	notifications.NewDeliveryHTTPController(
		notifications.NewDeliveryService(
			webhookDeliveryRepository,
			webhookSubscriptionRepository,
			catalogService,
			idGenerator,
			notifications.NewHMACEventSigner(webhookSecretStore, clock),
			notifications.NewWebhookSender(nil),
			clock,
		),
		idempotencyStore,
	).RegisterRoutes(mux)
	workspaceService := sellerworkspace.NewService(sellerworkspace.Dependencies{
		Workspaces:           sellerWorkspaceRepository,
		Sellers:              catalogRepository,
		Products:             catalogRepository,
		PaymentDestinations:  sellerworkspace.NewPaymentDestinationRepositoryReader(paymentDestinationRepository),
		Credentials:          sellerworkspace.NewCredentialRepositoryReader(integrationCredentialRepository),
		Transactions:         sellerworkspace.NewTransactionRepositoryReader(transactionRepository),
		Evidence:             evidenceRepository,
		WebhookSubscriptions: sellerworkspace.NewWebhookSubscriptionRepositoryReader(webhookSubscriptionRepository),
		WebhookDeliveries:    sellerworkspace.NewWebhookDeliveryRepositoryReader(webhookDeliveryRepository),
		Billing:              billingService,
		BillingPortal:        sellerworkspace.UnavailableBillingPortal{},
		AccountVerification:  sellerworkspace.AuthenticatedAccountVerification{},
		Clock:                clock,
	})
	sellerworkspace.NewHTTPController(
		workspaceService,
		sellerworkspace.NewContextPrincipalSource(
			catalogService,
			sellerworkspace.AuthenticatedAccountVerification{},
		),
	).RegisterRoutes(mux)
	storefront.NewHTTPController(storefront.NewService(storefront.Dependencies{
		Catalog: catalogRepository, Directory: catalogRepository,
		CanonicalOrigin: "https://agentpay.example", APIOrigin: "https://api.agentpay.example",
	})).RegisterRoutes(mux)
	intentService := intents.NewService(
		intentRepository,
		catalogRepository,
		idGenerator,
		clock,
	)
	intents.NewHTTPController(intentService, idempotencyStore).RegisterRoutes(mux)
	transactionService := transactions.NewService(
		transactionRepository,
		evidenceRepository,
		evidenceSigner,
		catalogRepository,
	)
	transactions.NewHTTPController(transactionService).RegisterRoutes(mux)
	analytics.NewHTTPController(
		analytics.NewDashboardService(transactionService, analytics.NewService()),
	).RegisterRoutes(mux)
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
	manualRemediationService := disputes.NewManualRemediationService(
		disputeRepository,
		transactionRepository,
		manualRefundRecordRepository,
		clock,
	)
	manualRemediationService.SetAuditRecorder(auditAppender)
	disputes.NewManualRemediationHTTPController(
		manualRemediationService,
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
