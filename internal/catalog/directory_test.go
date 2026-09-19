package catalog

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestNormalizePublicDirectoryQueryUsesBoundedExactTerms(t *testing.T) {
	t.Parallel()

	terms, err := NormalizePublicDirectoryQuery("  Research, MARKET research  ")
	if err != nil {
		t.Fatalf("NormalizePublicDirectoryQuery() error = %v", err)
	}
	want := []string{"research", "market"}
	if !reflect.DeepEqual(terms, want) {
		t.Fatalf("terms = %#v, want %#v", terms, want)
	}

	if _, err := NormalizePublicDirectoryQuery("a"); err == nil {
		t.Fatal("one-character search term must be rejected")
	}
	if _, err := NormalizePublicDirectoryQuery("one two three four five"); err == nil {
		t.Fatal("more than four search terms must be rejected")
	}
	if _, err := NormalizePublicDirectoryQuery("same same same same same"); err == nil {
		t.Fatal("more than four repeated search terms must be rejected")
	}
	if _, err := NormalizePublicDirectoryQuery("research " + strings.Repeat("x", MaximumPublicDirectoryTermLength+1)); err == nil {
		t.Fatal("an overlong term must not be silently discarded")
	}
}

func testPublishedDirectoryRoute(t *testing.T) PaidRoute {
	t.Helper()
	now := domain.NewTimestamp(time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC))
	route, err := NewPaidRoute(PaidRouteParams{
		RouteID:     mustCatalogID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.RouteIDPrefix),
		SellerID:    mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		DisplayName: "Research Report", ProductSlug: "research-report", Method: RouteMethodPost,
		PathPattern: "/research", Description: "Source-backed market brief", MIMEType: "application/json",
		Amount: domain.MustParseAmount("100000"), Asset: "USDC", Network: "eip155:84532",
		PayTo: "0x1111111111111111111111111111111111111111", UpstreamTimeoutSeconds: 10, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return route
}

func TestNewPublicDirectoryProjectionIndexesPublishedProductCopy(t *testing.T) {
	t.Parallel()
	route := testPublishedDirectoryRoute(t)

	projection, ok := NewPublicDirectoryProjection(route)
	if !ok {
		t.Fatal("published route should create a directory projection")
	}
	if projection.SellerID != route.SellerID || projection.RouteID != route.RouteID || projection.RouteVersion != route.Version {
		t.Fatalf("projection identity = %#v", projection)
	}
	wantTerms := []string{"research", "report", "source", "backed", "market", "brief"}
	if !reflect.DeepEqual(projection.SearchTerms, wantTerms) {
		t.Fatalf("search terms = %#v, want %#v", projection.SearchTerms, wantTerms)
	}
	if projection.DisplaySort != "research-report" {
		t.Fatalf("display sort = %q", projection.DisplaySort)
	}

	route.Enabled = false
	route.LifecycleStatus = RouteLifecyclePaused
	if _, ok := NewPublicDirectoryProjection(route); ok {
		t.Fatal("paused route must not create a directory projection")
	}
}
