package catalog

import (
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestNewPaidRouteNormalizesSellerApprovedProductIdentity(t *testing.T) {
	t.Parallel()

	routeID := mustCatalogID(
		t,
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.RouteIDPrefix,
	)
	paidRoute, err := NewPaidRoute(PaidRouteParams{
		RouteID:                routeID,
		SellerID:               mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		DisplayName:            "  Board-ready\t Market   Report  ",
		ProductSlug:            " Board_Ready--Market Report ",
		Method:                 RouteMethodPost,
		PathPattern:            "/research/board",
		Description:            "Generate a board-ready market report",
		MIMEType:               "application/json",
		Amount:                 domain.MustParseAmount("35000000"),
		Asset:                  "test-usdc",
		Network:                "test-network",
		PayTo:                  "0x1234567890abcdef",
		UpstreamTimeoutSeconds: 20,
		CreatedAt:              domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("NewPaidRoute() error = %v", err)
	}

	if paidRoute.DisplayName != "Board-ready Market Report" {
		t.Fatalf("DisplayName = %q", paidRoute.DisplayName)
	}
	if paidRoute.ProductSlug != "board-ready-market-report" {
		t.Fatalf("ProductSlug = %q", paidRoute.ProductSlug)
	}
	if paidRoute.RouteID != routeID || paidRoute.PathPattern != "/research/board" {
		t.Fatalf("technical identity changed: %#v", paidRoute)
	}
}

func TestNewPaidRouteRejectsInvalidProductIdentity(t *testing.T) {
	t.Parallel()

	valid := PaidRouteParams{
		RouteID:                mustCatalogID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		SellerID:               mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		DisplayName:            "Market Report",
		ProductSlug:            "market-report",
		Method:                 RouteMethodPost,
		PathPattern:            "/research",
		Description:            "Generate market research",
		MIMEType:               "application/json",
		Amount:                 domain.MustParseAmount("10000"),
		Asset:                  "test-usdc",
		Network:                "test-network",
		PayTo:                  "0x1234567890abcdef",
		UpstreamTimeoutSeconds: 20,
		CreatedAt:              domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)),
	}

	tests := []struct {
		name      string
		mutate    func(*PaidRouteParams)
		wantField string
	}{
		{name: "display name required", mutate: func(params *PaidRouteParams) { params.DisplayName = "  " }, wantField: "displayName"},
		{name: "display name too long", mutate: func(params *PaidRouteParams) { params.DisplayName = strings.Repeat("n", 121) }, wantField: "displayName"},
		{name: "display name control character", mutate: func(params *PaidRouteParams) { params.DisplayName = "Report\u0000Name" }, wantField: "displayName"},
		{name: "slug too short", mutate: func(params *PaidRouteParams) { params.ProductSlug = "a" }, wantField: "productSlug"},
		{name: "slug too long", mutate: func(params *PaidRouteParams) { params.ProductSlug = strings.Repeat("a", 81) }, wantField: "productSlug"},
		{name: "slug unsupported character", mutate: func(params *PaidRouteParams) { params.ProductSlug = "market/report" }, wantField: "productSlug"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			params := valid
			test.mutate(&params)
			_, err := NewPaidRoute(params)
			assertCatalogValidationField(t, err, test.wantField)
		})
	}
}

func TestPaidRouteNormalizesLegacyProductIdentity(t *testing.T) {
	t.Parallel()

	routeID := mustCatalogID(
		t,
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.RouteIDPrefix,
	)
	legacyRoute := PaidRoute{
		RouteID:     routeID,
		SellerID:    mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		Method:      RouteMethodPost,
		PathPattern: "/research/board",
		Description: "Board-ready Market Report",
		Enabled:     true,
	}

	legacyRoute.normalizeLegacyFields()

	if legacyRoute.DisplayName != "Board-ready Market Report" {
		t.Fatalf("DisplayName = %q", legacyRoute.DisplayName)
	}
	if legacyRoute.ProductSlug != "board-ready-market-report-4fwq0k9h7" {
		t.Fatalf("ProductSlug = %q", legacyRoute.ProductSlug)
	}
	if legacyRoute.RouteID != routeID || legacyRoute.PathPattern != "/research/board" {
		t.Fatalf("legacy normalization changed technical identity: %#v", legacyRoute)
	}
	if legacyRoute.LifecycleStatus != RouteLifecyclePublished {
		t.Fatalf("LifecycleStatus = %q", legacyRoute.LifecycleStatus)
	}
}
