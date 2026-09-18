package payments

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// TestPaidRouteServiceResolvesFrozenIntent verifies authoritative route binding.
func TestPaidRouteServiceResolvesFrozenIntent(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	resolved, err := fixture.service.Resolve(
		t.Context(),
		PaidRouteRequest{
			Slug:      fixture.seller.Slug,
			Method:    fixture.route.Method,
			ProxyPath: fixture.route.PathPattern,
			IntentID:  fixture.purchaseIntent.IntentID(),
			BuyerID:   fixture.purchaseIntent.BuyerID(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Seller.SellerID != fixture.seller.SellerID ||
		resolved.Route.RouteID != fixture.route.RouteID ||
		resolved.PurchaseIntent.IntentHash() != fixture.purchaseIntent.IntentHash() {
		t.Fatalf("resolved = %#v", resolved)
	}
	if resolved.Requirements.Amount != fixture.purchaseIntent.Amount() ||
		resolved.Requirements.PayTo != fixture.route.PayTo {
		t.Fatalf("requirements = %#v", resolved.Requirements)
	}
}

// TestPaidRouteServiceRejectsRouteAndIntentMismatches verifies immutable binding.
func TestPaidRouteServiceRejectsRouteAndIntentMismatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*PaidRouteRequest)
	}{
		{
			name: "seller slug",
			mutate: func(request *PaidRouteRequest) {
				request.Slug = "other-seller"
			},
		},
		{
			name: "method",
			mutate: func(request *PaidRouteRequest) {
				request.Method = catalog.RouteMethodPost
			},
		},
		{
			name: "path",
			mutate: func(request *PaidRouteRequest) {
				request.ProxyPath = "/different"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fixture := newPaidRouteFixture(t, false)
			request := PaidRouteRequest{
				Slug:      fixture.seller.Slug,
				Method:    fixture.route.Method,
				ProxyPath: fixture.route.PathPattern,
				IntentID:  fixture.purchaseIntent.IntentID(),
				BuyerID:   fixture.purchaseIntent.BuyerID(),
			}
			test.mutate(&request)
			_, err := fixture.service.Resolve(t.Context(), request)
			if !errors.Is(err, ErrPaidRouteMismatch) {
				t.Fatalf("Resolve() error = %v", err)
			}
		})
	}
}

// TestPaidRouteServiceRequiresBoundApproval verifies approval preconditions.
func TestPaidRouteServiceRequiresBoundApproval(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, true)
	request := PaidRouteRequest{
		Slug:      fixture.seller.Slug,
		Method:    fixture.route.Method,
		ProxyPath: fixture.route.PathPattern,
		IntentID:  fixture.purchaseIntent.IntentID(),
		BuyerID:   fixture.purchaseIntent.BuyerID(),
	}

	if _, err := fixture.service.Resolve(
		t.Context(),
		request,
	); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("Resolve() missing approval error = %v", err)
	}

	request.ApprovalToken = fixture.approvalToken
	if _, err := fixture.service.Resolve(t.Context(), request); err != nil {
		t.Fatalf("Resolve() approved error = %v", err)
	}

	request.ApprovalToken += "modified"
	if _, err := fixture.service.Resolve(
		t.Context(),
		request,
	); !errors.Is(err, ErrApprovalInvalid) {
		t.Fatalf("Resolve() modified approval error = %v", err)
	}
}

// TestPaidRouteServiceRejectsExpiredIntent verifies challenge expiry.
func TestPaidRouteServiceRejectsExpiredIntent(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	fixture.clock.Value = fixture.purchaseIntent.ExpiresAt().Time()
	_, err := fixture.service.Resolve(
		t.Context(),
		PaidRouteRequest{
			Slug:      fixture.seller.Slug,
			Method:    fixture.route.Method,
			ProxyPath: fixture.route.PathPattern,
			IntentID:  fixture.purchaseIntent.IntentID(),
			BuyerID:   fixture.purchaseIntent.BuyerID(),
		},
	)
	if !errors.Is(err, ErrIntentExpired) {
		t.Fatalf("Resolve() error = %v", err)
	}
}

type paidRouteFixture struct {
	service        *PaidRouteService
	clock          *mutableClock
	seller         catalog.Seller
	route          catalog.PaidRoute
	purchaseIntent intents.PurchaseIntent
	approvalToken  string
}

// newPaidRouteFixture creates persisted-looking paid-route dependencies.
func newPaidRouteFixture(t *testing.T, requiresApproval bool) paidRouteFixture {
	t.Helper()

	now := time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)
	clock := &mutableClock{Value: now}
	sellerID := mustPaymentID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	routeID := mustPaymentID(
		t,
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.RouteIDPrefix,
	)
	intentID := mustPaymentID(
		t,
		"int_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.IntentIDPrefix,
	)
	sessionID := mustPaymentID(
		t,
		"aps_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.ApprovalIDPrefix,
	)
	createdAt := domain.NewTimestamp(now)
	seller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID:        sellerID,
		OwnerSubject:    "seller-owner",
		Slug:            "demo-seller",
		Name:            "Demo Seller",
		UpstreamBaseURL: "https://seller.example",
		CreatedAt:       createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := seller.Activate(
		"arn:aws:secretsmanager:us-east-1:123456789012:secret:seller/demo",
		createdAt.Add(time.Second),
	); err != nil {
		t.Fatal(err)
	}
	route, err := catalog.NewPaidRoute(catalog.PaidRouteParams{
		RouteID:                routeID,
		SellerID:               sellerID,
		DisplayName:            "Weather Report",
		ProductSlug:            "weather-report",
		Method:                 catalog.RouteMethodGet,
		PathPattern:            "/weather",
		Description:            "Weather report",
		MIMEType:               "application/json",
		Amount:                 domain.MustParseAmount("10000"),
		Asset:                  BaseSepoliaUSDCAsset,
		Network:                BaseSepoliaNetwork,
		PayTo:                  "0x1111111111111111111111111111111111111111",
		UpstreamTimeoutSeconds: 20,
		CreatedAt:              createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	bodyHash, err := intents.HashRequestBody(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	purchaseIntent, err := intents.NewPurchaseIntent(intents.PurchaseIntentParams{
		IntentID:         intentID,
		SellerID:         sellerID,
		RouteID:          routeID,
		BuyerID:          "agent-123",
		RequestMethod:    intents.RequestMethodGet,
		RequestPath:      route.PathPattern,
		RequestBodyHash:  bodyHash,
		Amount:           route.Amount,
		Asset:            route.Asset,
		Network:          route.Network,
		MaximumAmount:    route.Amount,
		RequiresApproval: requiresApproval,
		CreatedAt:        createdAt,
		ExpiresAt:        createdAt.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	signer, err := approvals.NewApprovalTokenSigner(
		[]byte(strings.Repeat("s", 32)),
	)
	if err != nil {
		t.Fatal(err)
	}
	approvalRepository := &paidRouteApprovalRepository{}
	approvalToken := ""
	if requiresApproval {
		session, grants, createErr := approvals.NewSession(
			approvals.SessionParams{
				SessionID:      sessionID,
				IntentID:       intentID,
				IntentHash:     purchaseIntent.IntentHash(),
				ApproverLabels: []string{"Finance", "Security"},
				CreatedAt:      createdAt,
				ExpiresAt:      purchaseIntent.ExpiresAt(),
			},
			approvals.NewSecureTokenGenerator(
				strings.NewReader(
					strings.Repeat("a", 32)+
						strings.Repeat("b", 32),
				),
			),
		)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, createErr = session.Decide(
			grants[0].Token,
			approvals.DecisionApprove,
			createdAt.Add(time.Minute),
			signer,
		); createErr != nil {
			t.Fatal(createErr)
		}
		result, createErr := session.Decide(
			grants[1].Token,
			approvals.DecisionApprove,
			createdAt.Add(2*time.Minute),
			signer,
		)
		if createErr != nil {
			t.Fatal(createErr)
		}
		approvalRepository.session = session
		approvalToken = result.ApprovalToken
	}

	service := NewPaidRouteService(
		&paidRouteCatalogRepository{seller: seller, route: route},
		&paidRouteIntentRepository{purchaseIntent: purchaseIntent},
		approvalRepository,
		signer,
		clock,
		"https://api.example",
	)
	return paidRouteFixture{
		service:        service,
		clock:          clock,
		seller:         seller,
		route:          route,
		purchaseIntent: purchaseIntent,
		approvalToken:  approvalToken,
	}
}

type mutableClock struct {
	Value time.Time
}

// Now returns the current mutable fixture time.
func (clock *mutableClock) Now() time.Time {
	return clock.Value
}

type paidRouteCatalogRepository struct {
	seller catalog.Seller
	route  catalog.PaidRoute
}

// ResolveSellerBySlug returns the matching active seller.
func (repository *paidRouteCatalogRepository) ResolveSellerBySlug(
	_ context.Context,
	slug string,
) (catalog.Seller, error) {
	if slug != repository.seller.Slug {
		return catalog.Seller{}, persistence.ErrNotFound
	}
	return repository.seller, nil
}

// GetRoute returns the configured paid route.
func (repository *paidRouteCatalogRepository) GetRoute(
	_ context.Context,
	routeID domain.ID,
) (catalog.PaidRoute, error) {
	if routeID != repository.route.RouteID {
		return catalog.PaidRoute{}, persistence.ErrNotFound
	}
	return repository.route, nil
}

type paidRouteIntentRepository struct {
	purchaseIntent intents.PurchaseIntent
}

// Get returns the requested immutable purchase intent.
func (repository *paidRouteIntentRepository) Get(
	_ context.Context,
	intentID domain.ID,
) (intents.PurchaseIntent, error) {
	if intentID != repository.purchaseIntent.IntentID() {
		return intents.PurchaseIntent{}, persistence.ErrNotFound
	}
	return repository.purchaseIntent, nil
}

type paidRouteApprovalRepository struct {
	session approvals.Session
}

// Get returns the approved session addressed by the token claims.
func (repository *paidRouteApprovalRepository) Get(
	_ context.Context,
	sessionID domain.ID,
) (approvals.Session, error) {
	if sessionID == "" || sessionID != repository.session.SessionID() {
		return approvals.Session{}, persistence.ErrNotFound
	}
	return repository.session, nil
}

// mustPaymentID parses a stable prefixed test identifier.
func mustPaymentID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()

	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
