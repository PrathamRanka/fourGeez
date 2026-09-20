package catalog

import (
	"errors"
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
		DisplayName:             "Board-ready Market Report",
		ProductSlug:             "board-ready-market-report",
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
		DisplayName:            "Market Research",
		ProductSlug:            "market-research",
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
		{name: "timeout too high", mutate: func(params *PaidRouteParams) { params.UpstreamTimeoutSeconds = 26 }, wantField: "upstreamTimeoutSeconds"},
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
		DisplayName:            "Market Research",
		ProductSlug:            "market-research",
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
	if paidRoute.LifecycleStatus != RouteLifecyclePaused || paidRoute.Enabled {
		t.Fatalf("published price edit remained publicly available: %#v", paidRoute)
	}

	if err := paidRoute.ChangePrice(domain.MustParseAmount("0"), changedAt.Add(time.Minute)); err == nil {
		t.Fatal("ChangePrice() accepted zero")
	}
	if paidRoute.Amount.String() != "40000000" || paidRoute.Version != 2 {
		t.Fatal("failed price change mutated route")
	}
}

func TestPaidRouteUpdateDraftChangesTheWholeSellerControlledContract(t *testing.T) {
	t.Parallel()

	route := newLifecycleTestRoute(t, RouteLifecycleDraft)
	changedAt := route.UpdatedAt.Add(time.Minute)
	err := route.UpdateDraft(UpdateRouteDraftRequest{
		DisplayName:            "Executive Research Brief",
		Method:                 RouteMethodGet,
		PathPattern:            "/research/executive",
		Description:            "Return a concise executive research brief",
		MIMEType:               "application/json",
		InputSchema:            JSONSchema(`{"type":"object","properties":{"topic":{"type":"string"}},"required":["topic"],"additionalProperties":false}`),
		OutputSchema:           JSONSchema(`{"type":"object","properties":{"summary":{"type":"string"}},"required":["summary"],"additionalProperties":false}`),
		Amount:                 domain.MustParseAmount("40000000"),
		UpstreamTimeoutSeconds: 25,
	}, changedAt)
	if err != nil {
		t.Fatal(err)
	}
	if route.DisplayName != "Executive Research Brief" || route.Method != RouteMethodGet || route.PathPattern != "/research/executive" {
		t.Fatalf("updated route identity = %#v", route)
	}
	if route.Amount.String() != "40000000" || route.UpstreamTimeoutSeconds != 25 || route.Version != 2 || route.UpdatedAt != changedAt {
		t.Fatalf("updated route terms = %#v", route)
	}
}

func TestPaidRouteUpdateDraftRejectsLiveProduct(t *testing.T) {
	t.Parallel()

	route := newLifecycleTestRoute(t, RouteLifecyclePublished)
	before := route
	err := route.UpdateDraft(UpdateRouteDraftRequest{
		DisplayName:            route.DisplayName,
		Method:                 route.Method,
		PathPattern:            route.PathPattern,
		Description:            route.Description,
		MIMEType:               route.MIMEType,
		InputSchema:            route.InputSchema,
		OutputSchema:           route.OutputSchema,
		Amount:                 domain.MustParseAmount("40000000"),
		UpstreamTimeoutSeconds: route.UpstreamTimeoutSeconds,
	}, route.UpdatedAt.Add(time.Minute))
	if !errors.Is(err, ErrRoutePublished) {
		t.Fatalf("UpdateDraft() error = %v, want ErrRoutePublished", err)
	}
	if route != before {
		t.Fatal("rejected live edit mutated the product")
	}
}

// TestDraftPaidRoutePublication verifies explicit publication state changes.
func TestDraftPaidRoutePublication(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	paidRoute, err := NewDraftPaidRoute(PaidRouteParams{
		RouteID:                mustCatalogID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		SellerID:               mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		DisplayName:            "Market Research",
		ProductSlug:            "market-research",
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
		t.Fatal(err)
	}
	if paidRoute.Enabled {
		t.Fatal("draft route was published during construction")
	}
	publishedAt := createdAt.Add(time.Minute)
	if err := paidRoute.Publish(publishedAt); err != nil {
		t.Fatal(err)
	}
	if !paidRoute.Enabled || paidRoute.Version != 2 || paidRoute.UpdatedAt != publishedAt {
		t.Fatalf("published route = %#v", paidRoute)
	}
	if err := paidRoute.Publish(publishedAt.Add(time.Minute)); !errors.Is(err, ErrRoutePublished) {
		t.Fatalf("second Publish() error = %v", err)
	}
}

// TestPaidRouteLifecycleTransitions verifies every documented route state change.
func TestPaidRouteLifecycleTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		start       RouteLifecycleStatus
		transition  func(*PaidRoute, domain.Timestamp) error
		want        RouteLifecycleStatus
		wantEnabled bool
	}{
		{
			name:  "draft publishes",
			start: RouteLifecycleDraft,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Publish(changedAt)
			},
			want:        RouteLifecyclePublished,
			wantEnabled: true,
		},
		{
			name:  "published pauses",
			start: RouteLifecyclePublished,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Pause(changedAt)
			},
			want:        RouteLifecyclePaused,
			wantEnabled: false,
		},
		{
			name:  "paused resumes",
			start: RouteLifecyclePaused,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Publish(changedAt)
			},
			want:        RouteLifecyclePublished,
			wantEnabled: true,
		},
		{
			name:  "published emergency disables",
			start: RouteLifecyclePublished,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.EmergencyDisable(changedAt)
			},
			want:        RouteLifecycleEmergencyDisabled,
			wantEnabled: false,
		},
		{
			name:  "emergency disabled route resumes",
			start: RouteLifecycleEmergencyDisabled,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Publish(changedAt)
			},
			want:        RouteLifecyclePublished,
			wantEnabled: true,
		},
		{
			name:  "draft archives",
			start: RouteLifecycleDraft,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Archive(changedAt)
			},
			want:        RouteLifecycleArchived,
			wantEnabled: false,
		},
		{
			name:  "paused archives",
			start: RouteLifecyclePaused,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Archive(changedAt)
			},
			want:        RouteLifecycleArchived,
			wantEnabled: false,
		},
		{
			name:  "emergency disabled route archives",
			start: RouteLifecycleEmergencyDisabled,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Archive(changedAt)
			},
			want:        RouteLifecycleArchived,
			wantEnabled: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			route := newLifecycleTestRoute(t, test.start)
			previousVersion := route.Version
			changedAt := route.UpdatedAt.Add(time.Minute)

			if err := test.transition(&route, changedAt); err != nil {
				t.Fatalf("transition error = %v", err)
			}
			if route.LifecycleStatus != test.want {
				t.Fatalf("LifecycleStatus = %q, want %q", route.LifecycleStatus, test.want)
			}
			if route.Enabled != test.wantEnabled {
				t.Fatalf("Enabled = %v, want %v", route.Enabled, test.wantEnabled)
			}
			if route.Version != previousVersion+1 {
				t.Fatalf("Version = %d, want %d", route.Version, previousVersion+1)
			}
			if route.UpdatedAt != changedAt {
				t.Fatalf("UpdatedAt = %s, want %s", route.UpdatedAt, changedAt)
			}
		})
	}
}

// TestPaidRouteRejectsInvalidLifecycleTransitions protects terminal and unsafe states.
func TestPaidRouteRejectsInvalidLifecycleTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		start      RouteLifecycleStatus
		transition func(*PaidRoute, domain.Timestamp) error
	}{
		{
			name:  "draft cannot pause",
			start: RouteLifecycleDraft,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Pause(changedAt)
			},
		},
		{
			name:  "draft cannot emergency disable",
			start: RouteLifecycleDraft,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.EmergencyDisable(changedAt)
			},
		},
		{
			name:  "published cannot archive",
			start: RouteLifecyclePublished,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Archive(changedAt)
			},
		},
		{
			name:  "archived cannot publish",
			start: RouteLifecycleArchived,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Publish(changedAt)
			},
		},
		{
			name:  "paused cannot pause again",
			start: RouteLifecyclePaused,
			transition: func(route *PaidRoute, changedAt domain.Timestamp) error {
				return route.Pause(changedAt)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			route := newLifecycleTestRoute(t, test.start)
			before := route

			err := test.transition(&route, route.UpdatedAt.Add(time.Minute))
			if !errors.Is(err, ErrRouteLifecycleTransition) {
				t.Fatalf("transition error = %v, want ErrRouteLifecycleTransition", err)
			}
			if route != before {
				t.Fatalf("failed transition mutated route: %#v", route)
			}
		})
	}
}

// newLifecycleTestRoute creates one valid route in the requested lifecycle state.
func newLifecycleTestRoute(t *testing.T, status RouteLifecycleStatus) PaidRoute {
	t.Helper()

	createdAt := domain.NewTimestamp(
		time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	)
	route, err := NewDraftPaidRoute(PaidRouteParams{
		RouteID:                mustCatalogID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		SellerID:               mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		DisplayName:            "Market Research",
		ProductSlug:            "market-research",
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
		t.Fatalf("NewDraftPaidRoute() error = %v", err)
	}

	route.LifecycleStatus = status
	route.Enabled = status == RouteLifecyclePublished
	return route
}
