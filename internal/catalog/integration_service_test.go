package catalog_test

import (
	"context"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

// TestIntegrationCatalogLifecycle verifies configure, draft, validate, and publish operations.
func TestIntegrationCatalogLifecycle(t *testing.T) {
	t.Parallel()

	clock := &integrationCatalogClock{
		now: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	repository := memory.NewCatalogRepository()
	service := catalog.NewService(
		repository,
		&integrationCatalogIDGenerator{},
		clock,
		audit.NoopRecorder{},
	)
	quota := &integrationQuotaEnforcer{}
	service.SetQuotaEnforcer(quota)
	seller, err := service.CreateSeller(
		t.Context(),
		"owner-123",
		catalog.CreateSellerRequest{
			Name:            "Demo",
			Slug:            "demo-shop",
			UpstreamBaseURL: "https://seller.example",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	clock.now = clock.now.Add(time.Minute)
	configured, err := service.ConfigureStorefrontForIntegration(
		t.Context(),
		seller.SellerID,
		catalog.ConfigureStorefrontRequest{
			Name:            "Demo Intelligence",
			UpstreamBaseURL: "https://api.seller.example/v1",
			ExpectedVersion: seller.Version,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if configured.Name != "Demo Intelligence" {
		t.Fatalf("configured seller = %#v", configured)
	}

	clock.now = clock.now.Add(time.Minute)
	route, err := service.CreateDraftRouteForIntegration(
		t.Context(),
		seller.SellerID,
		catalog.CreateRouteRequest{
			Method:                 catalog.RouteMethodPost,
			PathPattern:            "/research",
			Description:            "Research",
			MIMEType:               "application/json",
			Amount:                 domain.MustParseAmount("100"),
			Asset:                  "USDC",
			Network:                "eip155:84532",
			PayTo:                  "0x123",
			UpstreamTimeoutSeconds: 20,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if route.Enabled {
		t.Fatal("integration route was published during configuration")
	}

	validation, err := service.ValidateRouteForIntegration(
		t.Context(),
		seller.SellerID,
		route.RouteID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid {
		t.Fatal("validation passed before seller activation")
	}

	storedSeller, err := repository.GetSeller(t.Context(), seller.SellerID)
	if err != nil {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(time.Minute)
	if err := storedSeller.Activate("secret/seller/demo", domain.NewTimestamp(clock.now)); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateSeller(t.Context(), storedSeller, configured.Version); err != nil {
		t.Fatal(err)
	}

	validation, err = service.ValidateRouteForIntegration(
		t.Context(),
		seller.SellerID,
		route.RouteID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid {
		t.Fatalf("validation = %#v", validation)
	}

	clock.now = clock.now.Add(time.Minute)
	published, err := service.PublishRouteForIntegration(
		t.Context(),
		seller.SellerID,
		route.RouteID,
		route.Version,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !published.Enabled || published.Version != route.Version+1 {
		t.Fatalf("published route = %#v", published)
	}
	if quota.publishedRouteCount != 0 {
		t.Fatalf("published route count = %d, want 0", quota.publishedRouteCount)
	}
}

type integrationQuotaEnforcer struct {
	publishedRouteCount uint64
}

// ConsumeAPIRequest permits catalog integration requests.
func (enforcer *integrationQuotaEnforcer) ConsumeAPIRequest(context.Context, domain.ID) error {
	return nil
}

// ConsumeMCPOperation permits catalog integration operations.
func (enforcer *integrationQuotaEnforcer) ConsumeMCPOperation(context.Context, domain.ID) error {
	return nil
}

// ConsumeWebhookDelivery permits catalog integration deliveries.
func (enforcer *integrationQuotaEnforcer) ConsumeWebhookDelivery(context.Context, domain.ID, string) error {
	return nil
}

// AllowPublishedRoute records the current published-route count.
func (enforcer *integrationQuotaEnforcer) AllowPublishedRoute(
	_ context.Context,
	_ domain.ID,
	count uint64,
) error {
	enforcer.publishedRouteCount = count
	return nil
}

// AllowWebhookSubscription permits catalog integration subscriptions.
func (enforcer *integrationQuotaEnforcer) AllowWebhookSubscription(context.Context, domain.ID, uint64) error {
	return nil
}

type integrationCatalogClock struct {
	now time.Time
}

// Now returns the mutable integration-test time.
func (clock *integrationCatalogClock) Now() time.Time {
	return clock.now
}

type integrationCatalogIDGenerator struct{}

// New returns stable seller and route identifiers for the integration lifecycle.
func (*integrationCatalogIDGenerator) New(
	prefix domain.IDPrefix,
) (domain.ID, error) {
	if prefix == domain.SellerIDPrefix {
		return domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"), nil
	}
	return domain.ID("rte_01K5D09YJ0C0M7RJM4FWQ0K9H8"), nil
}
