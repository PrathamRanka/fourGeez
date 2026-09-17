package billing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestPlanRoutesExposeCatalogAndSellerAssignment verifies BIL-001 HTTP reads.
func TestPlanRoutesExposeCatalogAndSellerAssignment(t *testing.T) {
	t.Parallel()

	sellerID := mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	service := NewService(
		newBillingRepository(),
		billingControllerAuthorizer{sellerID: sellerID},
		domain.FixedClock{
			Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
		},
	)
	mux := http.NewServeMux()
	NewHTTPController(service).RegisterRoutes(mux)
	handler := api.Middleware(api.Config{
		Authenticator: api.NewStaticAuthenticator("seller-token", "agent-key"),
	}, mux)

	planRequest := httptest.NewRequest(http.MethodGet, "/v1/plans", nil)
	planResponse := httptest.NewRecorder()
	handler.ServeHTTP(planResponse, planRequest)
	if planResponse.Code != http.StatusOK ||
		!strings.Contains(planResponse.Body.String(), `"planId":"starter"`) {
		t.Fatalf("plan response = %d %s", planResponse.Code, planResponse.Body.String())
	}

	assignmentRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/sellers/"+sellerID.String()+"/plan",
		nil,
	)
	assignmentRequest.Header.Set("Authorization", "Bearer seller-token")
	assignmentResponse := httptest.NewRecorder()
	handler.ServeHTTP(assignmentResponse, assignmentRequest)
	if assignmentResponse.Code != http.StatusOK ||
		!strings.Contains(assignmentResponse.Body.String(), `"planId":"starter"`) {
		t.Fatalf("assignment response = %d %s", assignmentResponse.Code, assignmentResponse.Body.String())
	}
}

// TestInvoiceExportRouteReturnsBoundedUsage verifies the BIL-002 HTTP contract.
func TestInvoiceExportRouteReturnsBoundedUsage(t *testing.T) {
	t.Parallel()
	occurredAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	transaction := fulfilledBillingTransaction(t, occurredAt)
	planRepository := newBillingRepository()
	planService := NewService(planRepository, billingControllerAuthorizer{sellerID: transaction.SellerID()}, domain.FixedClock{Value: occurredAt.Time()})
	if _, err := planService.GetSellerPlan(context.Background(), "local-seller", transaction.SellerID()); err != nil {
		t.Fatal(err)
	}
	usageRepository := newUsageRepository()
	usageService := NewUsageService(
		usageRepository,
		planService,
		billingTransactionReader{transaction: transaction},
		billingControllerAuthorizer{sellerID: transaction.SellerID()},
		domain.NewULIDGenerator(domain.FixedClock{Value: occurredAt.Time()}, strings.NewReader(strings.Repeat("u", 128))),
		domain.FixedClock{Value: occurredAt.Time()},
	)
	if _, err := usageService.RecordSuccessfulTransaction(context.Background(), transaction.TransactionID()); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewUsageHTTPController(usageService).RegisterRoutes(mux)
	handler := api.Middleware(api.Config{Authenticator: api.NewStaticAuthenticator("seller-token", "agent-key")}, mux)
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/sellers/"+transaction.SellerID().String()+"/invoice-export?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z",
		nil,
	)
	request.Header.Set("Authorization", "Bearer seller-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"quantity":1`) {
		t.Fatalf("invoice response = %d %s", response.Code, response.Body.String())
	}
}

type billingControllerAuthorizer struct {
	sellerID domain.ID
}

// AuthorizeSeller verifies the local seller principal used by middleware.
func (authorizer billingControllerAuthorizer) AuthorizeSeller(
	_ context.Context,
	ownerSubject string,
	sellerID domain.ID,
) error {
	if ownerSubject != "local-seller" || sellerID != authorizer.sellerID {
		return ErrSellerPlanNotFound
	}
	return nil
}
