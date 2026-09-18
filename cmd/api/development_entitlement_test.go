package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/operations"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/settlement"
)

func TestLocalOnboardingProvisionsStarterEntitlementBeforeSellerQuota(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	clock := domain.FixedClock{Value: now}
	catalogRepository := memory.NewCatalogRepository()
	sellerID := mustDevelopmentSellerID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	seller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID: sellerID, OwnerSubject: "local:new-seller", Slug: "new-seller",
		Name: "New Seller", UpstreamBaseURL: "https://seller.example", CreatedAt: domain.NewTimestamp(now),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogRepository.CreateSeller(context.Background(), seller); err != nil {
		t.Fatal(err)
	}

	storedEntitlements := memory.NewSellerEntitlementRepository()
	entitlements := configureOnboardingEntitlementRepository(
		"local",
		storedEntitlements,
		catalogRepository,
		clock,
	)
	billingService := billing.NewService(entitlements, nil, clock)
	quotaService := operations.NewService(memory.NewQuotaCounterRepository(), billingService, clock)

	if err := quotaService.ConsumeAPIRequest(context.Background(), sellerID); err != nil {
		t.Fatalf("ConsumeAPIRequest() error = %v, want local Starter entitlement", err)
	}
	entitlement, err := storedEntitlements.Get(context.Background(), sellerID)
	if err != nil {
		t.Fatal(err)
	}
	periodStart := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	if entitlement.PlanID() != billing.PlanStarter ||
		entitlement.Status() != billing.EntitlementStatusActive ||
		entitlement.Source() != billing.EntitlementSourceLocal ||
		entitlement.Provider() != billing.EntitlementProviderLocal ||
		!entitlement.BillingPeriodStart().Time().Equal(periodStart) ||
		!entitlement.BillingPeriodEnd().Time().Equal(periodEnd) ||
		!entitlement.AccessEndsAt().Time().Equal(periodEnd) {
		t.Fatalf("local entitlement = %#v", entitlement.Snapshot())
	}
}

func TestNonLocalOnboardingDoesNotCreateMissingEntitlement(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	clock := domain.FixedClock{Value: now}
	catalogRepository := memory.NewCatalogRepository()
	sellerID := mustDevelopmentSellerID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H8")
	seller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID: sellerID, OwnerSubject: "cognito:new-seller", Slug: "paid-seller",
		Name: "Paid Seller", UpstreamBaseURL: "https://seller.example", CreatedAt: domain.NewTimestamp(now),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogRepository.CreateSeller(context.Background(), seller); err != nil {
		t.Fatal(err)
	}

	storedEntitlements := memory.NewSellerEntitlementRepository()
	entitlements := configureOnboardingEntitlementRepository(
		"production",
		storedEntitlements,
		catalogRepository,
		clock,
	)
	if _, err := entitlements.Get(context.Background(), sellerID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("Get() error = %v, want missing production entitlement", err)
	}
}

func TestNewLocalSellerCanCreatePaymentDestinationThroughQuotaMiddleware(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	clock := domain.FixedClock{Value: now}
	catalogRepository := memory.NewCatalogRepository()
	sellerID := mustDevelopmentSellerID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H9")
	seller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID: sellerID, OwnerSubject: "local-seller", Slug: "checkout-seller",
		Name: "Checkout Seller", UpstreamBaseURL: "https://seller.example", CreatedAt: domain.NewTimestamp(now),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogRepository.CreateSeller(context.Background(), seller); err != nil {
		t.Fatal(err)
	}
	catalogService := catalog.NewService(
		catalogRepository,
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("a", 128))),
		clock,
		audit.NoopRecorder{},
	)
	storedEntitlements := memory.NewSellerEntitlementRepository()
	billingService := billing.NewService(
		configureOnboardingEntitlementRepository("local", storedEntitlements, catalogRepository, clock),
		catalogService,
		clock,
	)
	quotaService := operations.NewService(memory.NewQuotaCounterRepository(), billingService, clock)
	settlementService := settlement.NewService(
		memory.NewPaymentDestinationRepository(),
		catalogService,
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("b", 128))),
		nil,
		nil,
		clock,
		audit.NoopRecorder{},
	)
	mux := http.NewServeMux()
	settlement.NewHTTPController(settlementService, memory.NewIdempotencyStore()).RegisterRoutes(mux)
	handler := api.Middleware(api.Config{
		Authenticator:        api.NewStaticAuthenticator("seller-secret", "agent-secret"),
		SellerRequestLimiter: quotaService,
		SellerAuthorizer:     catalogService,
	}, mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/sellers/"+sellerID.String()+"/payment-destinations",
		bytes.NewBufferString(`{"asset":"USDC","network":"eip155:84532","address":"0x1111111111111111111111111111111111111111"}`),
	)
	request.Header.Set("Authorization", "Bearer seller-secret")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "new-local-seller-destination")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("payment-destination status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestLocalOnboardingDoesNotReactivateExistingCancelledEntitlement(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	clock := domain.FixedClock{Value: now}
	catalogRepository := memory.NewCatalogRepository()
	sellerID := mustDevelopmentSellerID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9HA")
	seller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID: sellerID, OwnerSubject: "local:cancelled-seller", Slug: "cancelled-seller",
		Name: "Cancelled Seller", UpstreamBaseURL: "https://seller.example", CreatedAt: domain.NewTimestamp(now),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogRepository.CreateSeller(context.Background(), seller); err != nil {
		t.Fatal(err)
	}
	storedEntitlements := memory.NewSellerEntitlementRepository()
	cancelled, reconciliation, err := billing.ReconcileSellerEntitlement(
		nil,
		billing.EntitlementCandidate{
			SellerID: sellerID, PlanID: billing.PlanStarter, PlanVersion: 1,
			Status:             billing.EntitlementStatusCancelled,
			BillingPeriodStart: domain.NewTimestamp(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)),
			BillingPeriodEnd:   domain.NewTimestamp(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)),
			AccessEndsAt:       domain.NewTimestamp(now.Add(-time.Hour)),
			Source:             billing.EntitlementSourceLocal,
			StatusReason:       billing.EntitlementStatusReasonCancelled,
			Provider:           billing.EntitlementProviderLocal,
		},
		domain.NewTimestamp(now),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := storedEntitlements.Apply(context.Background(), cancelled, reconciliation, 0); err != nil {
		t.Fatal(err)
	}
	entitlements := configureOnboardingEntitlementRepository(
		"local",
		storedEntitlements,
		catalogRepository,
		clock,
	)

	loaded, err := entitlements.Get(context.Background(), sellerID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status() != billing.EntitlementStatusCancelled || loaded.Version() != cancelled.Version() {
		t.Fatalf("loaded entitlement = %#v, want unchanged cancellation", loaded.Snapshot())
	}
}

func mustDevelopmentSellerID(t *testing.T, value string) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(value, domain.SellerIDPrefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
