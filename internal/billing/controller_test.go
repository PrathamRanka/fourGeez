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
