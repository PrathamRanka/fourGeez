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

// TestCatalogServiceAuditsDirectRoutePublicationAndPricing verifies seller changes.
func TestCatalogServiceAuditsDirectRoutePublicationAndPricing(t *testing.T) {
	t.Parallel()

	clock := &integrationCatalogClock{
		now: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	}
	recorder := &catalogAuditRecorder{}
	service := catalog.NewService(
		memory.NewCatalogRepository(),
		&integrationCatalogIDGenerator{},
		clock,
		recorder,
	)
	seller, err := service.CreateSeller(
		t.Context(),
		"owner-123",
		catalog.CreateSellerRequest{
			Name:            "Demo",
			Slug:            "audit-demo",
			UpstreamBaseURL: "https://seller.example",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	route, err := service.CreateRoute(
		t.Context(),
		"owner-123",
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
	clock.now = clock.now.Add(time.Minute)
	updatedRoute, err := service.UpdateRoutePrice(
		t.Context(),
		"owner-123",
		seller.SellerID,
		route.RouteID,
		catalog.UpdateRoutePriceRequest{
			Amount:          domain.MustParseAmount("200"),
			ExpectedVersion: route.Version,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(time.Minute)
	pausedRoute, err := service.PauseSellerRoute(
		t.Context(),
		"owner-123",
		seller.SellerID,
		route.RouteID,
		catalog.RouteVersionRequest{ExpectedVersion: updatedRoute.Version},
	)
	if err != nil {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(time.Minute)
	if _, err := service.ArchiveSellerRoute(
		t.Context(),
		"owner-123",
		seller.SellerID,
		route.RouteID,
		catalog.RouteVersionRequest{ExpectedVersion: pausedRoute.Version},
	); err != nil {
		t.Fatal(err)
	}

	if len(recorder.requests) != 4 ||
		recorder.requests[0].Action != audit.ActionRoutePublished ||
		recorder.requests[1].Action != audit.ActionRoutePriceChanged ||
		recorder.requests[2].Action != audit.ActionRoutePaused ||
		recorder.requests[3].Action != audit.ActionRouteArchived {
		t.Fatalf("audit requests = %#v", recorder.requests)
	}
}

type catalogAuditRecorder struct {
	requests []audit.RecordRequest
}

// Record captures one direct catalog audit request.
func (recorder *catalogAuditRecorder) Record(
	_ context.Context,
	request audit.RecordRequest,
) error {
	recorder.requests = append(recorder.requests, request)
	return nil
}
