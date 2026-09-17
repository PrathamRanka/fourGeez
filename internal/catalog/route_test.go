package catalog

import (
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestNewPaidRoute(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	threshold := domain.MustParseAmount("5000000")
	paidRoute, err := NewPaidRoute(PaidRouteParams{
		RouteID:                 mustCatalogID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		SellerID:                mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		Method:                  RouteMethodPost,
		PathPattern:             "/research/board",
		Description:             "Generate a board-ready market report",
		MIMEType:                "application/json",
		Amount:                  domain.MustParseAmount("35000000"),
		Asset:                   "test-usdc",
		Network:                 "test-network",
		PayTo:                   "0x1234567890abcdef",
		ApprovalThresholdAmount: &threshold,
		UpstreamTimeoutSeconds:  20,
		CreatedAt:               createdAt,
	})
	if err != nil {
		t.Fatalf("NewPaidRoute() error = %v", err)
	}

	if !paidRoute.Enabled || paidRoute.Version != 1 {
		t.Fatalf("route enabled/version = %v/%d, want true/1", paidRoute.Enabled, paidRoute.Version)
	}
	if paidRoute.CreatedAt != paidRoute.UpdatedAt {
		t.Fatalf("CreatedAt = %s UpdatedAt = %s", paidRoute.CreatedAt, paidRoute.UpdatedAt)
	}
}

func TestNewPaidRouteValidation(t *testing.T) {
	t.Parallel()

	valid := PaidRouteParams{
		RouteID:                mustCatalogID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		SellerID:               mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		Method:                 RouteMethodPost,
		PathPattern:            "/research",
		Description:            "Generate market research",
		MIMEType:               "application/json",
		Amount:                 domain.MustParseAmount("10000"),
		Asset:                  "test-usdc",
		Network:                "test-network",
		PayTo:                  "0x1234567890abcdef",
		UpstreamTimeoutSeconds: 20,
		CreatedAt:              domain.NewTimestamp(time.Now()),
	}

	tests := []struct {
		name      string
		mutate    func(*PaidRouteParams)
		wantField string
	}{
		{name: "route ID prefix", mutate: func(params *PaidRouteParams) {
			params.RouteID = mustCatalogID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
		}, wantField: "routeId"},
		{name: "seller ID prefix", mutate: func(params *PaidRouteParams) {
			params.SellerID = mustCatalogID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
		}, wantField: "sellerId"},
		{name: "unsupported method", mutate: func(params *PaidRouteParams) { params.Method = RouteMethod("DELETE") }, wantField: "method"},
		{name: "invalid path", mutate: func(params *PaidRouteParams) { params.PathPattern = "research" }, wantField: "pathPattern"},
		{name: "description required", mutate: func(params *PaidRouteParams) { params.Description = "" }, wantField: "description"},
		{name: "description too long", mutate: func(params *PaidRouteParams) { params.Description = strings.Repeat("d", 501) }, wantField: "description"},
		{name: "MIME type required", mutate: func(params *PaidRouteParams) { params.MIMEType = "" }, wantField: "mimeType"},
		{name: "zero price", mutate: func(params *PaidRouteParams) { params.Amount = domain.MustParseAmount("0") }, wantField: "amount"},
		{name: "asset required", mutate: func(params *PaidRouteParams) { params.Asset = "" }, wantField: "asset"},
		{name: "network required", mutate: func(params *PaidRouteParams) { params.Network = "" }, wantField: "network"},
		{name: "payTo required", mutate: func(params *PaidRouteParams) { params.PayTo = "" }, wantField: "payTo"},
		{name: "timeout too low", mutate: func(params *PaidRouteParams) { params.UpstreamTimeoutSeconds = 0 }, wantField: "upstreamTimeoutSeconds"},
		{name: "timeout too high", mutate: func(params *PaidRouteParams) { params.UpstreamTimeoutSeconds = 31 }, wantField: "upstreamTimeoutSeconds"},
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

func TestPaidRouteChangePrice(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	paidRoute, err := NewPaidRoute(PaidRouteParams{
		RouteID:                mustCatalogID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		SellerID:               mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		Method:                 RouteMethodPost,
		PathPattern:            "/research",
		Description:            "Generate market research",
		MIMEType:               "application/json",
		Amount:                 domain.MustParseAmount("35000000"),
		Asset:                  "test-usdc",
		Network:                "test-network",
		PayTo:                  "0x1234567890abcdef",
		UpstreamTimeoutSeconds: 20,
		CreatedAt:              createdAt,
	})
	if err != nil {
		t.Fatalf("NewPaidRoute() error = %v", err)
	}

	changedAt := createdAt.Add(time.Minute)
	if err := paidRoute.ChangePrice(domain.MustParseAmount("40000000"), changedAt); err != nil {
		t.Fatalf("ChangePrice() error = %v", err)
	}
	if paidRoute.Amount.String() != "40000000" || paidRoute.Version != 2 || paidRoute.UpdatedAt != changedAt {
		t.Fatalf("updated route = %#v", paidRoute)
	}

	if err := paidRoute.ChangePrice(domain.MustParseAmount("0"), changedAt.Add(time.Minute)); err == nil {
		t.Fatal("ChangePrice() accepted zero")
	}
	if paidRoute.Amount.String() != "40000000" || paidRoute.Version != 2 {
		t.Fatal("failed price change mutated route")
	}
}
