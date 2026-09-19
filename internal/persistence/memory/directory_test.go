package memory

import (
	"context"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
)

func TestCatalogRepositoryMaintainsPublicDirectoryProjection(t *testing.T) {
	t.Parallel()
	repository := NewCatalogRepository()
	now := domain.NewTimestamp(time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC))
	sellerID := mustMemoryID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	routeID := mustMemoryID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.RouteIDPrefix)
	seller, err := catalog.NewSeller(catalog.SellerParams{SellerID: sellerID, OwnerSubject: "owner", Slug: "northstar", Name: "Northstar", UpstreamBaseURL: "https://seller.example", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateSeller(context.Background(), seller); err != nil {
		t.Fatal(err)
	}
	route, err := catalog.NewPaidRoute(catalog.PaidRouteParams{RouteID: routeID, SellerID: sellerID, DisplayName: "Research Report", ProductSlug: "research-report", Method: catalog.RouteMethodPost, PathPattern: "/research", Description: "Source backed market brief", MIMEType: "application/json", Amount: domain.MustParseAmount("100000"), Asset: "USDC", Network: "eip155:84532", PayTo: "0x1111111111111111111111111111111111111111", UpstreamTimeoutSeconds: 10, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateRoute(context.Background(), route); err != nil {
		t.Fatal(err)
	}

	page, err := repository.ListPublicDirectoryCandidates(context.Background(), catalog.PublicDirectoryQuery{Limit: 24})
	if err != nil || len(page.Items) != 1 || page.Items[0].Projection.RouteID != routeID {
		t.Fatalf("directory page = %#v, error = %v", page, err)
	}
	searchPage, err := repository.ListPublicDirectoryCandidates(context.Background(), catalog.PublicDirectoryQuery{SearchTerm: "market", Limit: 24})
	if err != nil || len(searchPage.Items) != 1 {
		t.Fatalf("search page = %#v, error = %v", searchPage, err)
	}

	expectedVersion := route.Version
	if err := route.Pause(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateRoute(context.Background(), route, expectedVersion); err != nil {
		t.Fatal(err)
	}
	page, err = repository.ListPublicDirectoryCandidates(context.Background(), catalog.PublicDirectoryQuery{Limit: 24})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("paused directory page = %#v, error = %v", page, err)
	}
}
