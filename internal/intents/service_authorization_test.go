package intents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/settlement"
)

func TestCreateFreezesFreshAuthorizedProductAndDestination(t *testing.T) {
	t.Parallel()
	now := domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC))
	route := authorizedIntentRoute(t, now)
	destinationID := mustIntentID(t, "dst_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.PaymentDestinationIDPrefix)
	authorizer := &intentAuthorizer{route: route, destination: settlement.PaymentDestination{DestinationID: destinationID, SellerID: route.SellerID, Asset: route.Asset, Network: route.Network, Address: route.PayTo, Status: settlement.PaymentDestinationStatusActive}}
	service := NewServiceWithCommerceAuthorizer(&intentTestRepository{}, &intentRouteRepository{route: route}, authorizer, domain.NewULIDGenerator(domain.FixedClock{Value: now.Time()}, nil), domain.FixedClock{Value: now.Time()})
	digest, _ := ParseSHA256Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	purchaseIntent, err := service.Create(t.Context(), "buyer-agent", CreateIntentRequest{RouteID: route.RouteID, RequestBodyHash: digest, MaximumAmount: route.Amount})
	if err != nil {
		t.Fatal(err)
	}
	if authorizer.calls != 1 || purchaseIntent.ProductDisplayName() != route.DisplayName || purchaseIntent.ProductSlug() != route.ProductSlug || purchaseIntent.PaymentDestinationID() != destinationID || purchaseIntent.PayTo() != route.PayTo {
		t.Fatalf("purchase intent = %#v calls=%d", purchaseIntent.Snapshot(), authorizer.calls)
	}
	if purchaseIntent.PurchaseChannel() != PurchaseChannelAgent || purchaseIntent.PurchaseSessionID() != "" {
		t.Fatalf("agent purchase identity = (%s, %q)", purchaseIntent.PurchaseChannel(), purchaseIntent.PurchaseSessionID())
	}
}

func TestCreateDisablesHistoricalBuyerApprovalForLeanV1(t *testing.T) {
	t.Parallel()
	now := domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC))
	route := authorizedIntentRoute(t, now)
	threshold := domain.MustParseAmount("1")
	route.ApprovalThresholdAmount = &threshold
	destinationID := mustIntentID(t, "dst_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.PaymentDestinationIDPrefix)
	authorizer := &intentAuthorizer{route: route, destination: settlement.PaymentDestination{DestinationID: destinationID, SellerID: route.SellerID, Asset: route.Asset, Network: route.Network, Address: route.PayTo, Status: settlement.PaymentDestinationStatusActive}}
	service := NewServiceWithCommerceAuthorizer(&intentTestRepository{}, &intentRouteRepository{route: route}, authorizer, domain.NewULIDGenerator(domain.FixedClock{Value: now.Time()}, nil), domain.FixedClock{Value: now.Time()})
	digest, _ := ParseSHA256Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	purchaseIntent, err := service.Create(t.Context(), "buyer-agent", CreateIntentRequest{RouteID: route.RouteID, RequestBodyHash: digest, MaximumAmount: route.Amount})
	if err != nil {
		t.Fatal(err)
	}
	if purchaseIntent.RequiresApproval() || purchaseIntent.Status() != PurchaseIntentStatusReady {
		t.Fatalf("purchase intent approval state = (%t, %s), want disabled and ready", purchaseIntent.RequiresApproval(), purchaseIntent.Status())
	}
}

func TestCreateBindsBrowserPurchaseSessionToIntent(t *testing.T) {
	t.Parallel()
	now := domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC))
	route := authorizedIntentRoute(t, now)
	destinationID := mustIntentID(t, "dst_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.PaymentDestinationIDPrefix)
	authorizer := &intentAuthorizer{route: route, destination: settlement.PaymentDestination{DestinationID: destinationID, SellerID: route.SellerID, Asset: route.Asset, Network: route.Network, Address: route.PayTo, Status: settlement.PaymentDestinationStatusActive}}
	service := NewServiceWithCommerceAuthorizer(&intentTestRepository{}, &intentRouteRepository{route: route}, authorizer, domain.NewULIDGenerator(domain.FixedClock{Value: now.Time()}, nil), domain.FixedClock{Value: now.Time()})
	digest, _ := ParseSHA256Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	purchaseIntent, err := service.Create(t.Context(), "browser:bps_01K5D09YJ0C0M7RJM4FWQ0K9H9", CreateIntentRequest{RouteID: route.RouteID, RequestBodyHash: digest, MaximumAmount: route.Amount})
	if err != nil {
		t.Fatal(err)
	}
	if purchaseIntent.PurchaseChannel() != PurchaseChannelBrowser || purchaseIntent.PurchaseSessionID() != "bps_01K5D09YJ0C0M7RJM4FWQ0K9H9" {
		t.Fatalf("browser purchase identity = (%s, %q)", purchaseIntent.PurchaseChannel(), purchaseIntent.PurchaseSessionID())
	}
}

func TestCancelReturnsConcurrentExecutedWinnerAtExpiration(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC))
	purchaseIntent, err := NewPurchaseIntent(PurchaseIntentParams{
		IntentID:             mustIntentID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:             mustIntentID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		RouteID:              mustIntentID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:              "buyer-agent",
		ProductDisplayName:   "Research Report",
		ProductSlug:          "research-report",
		PaymentDestinationID: mustIntentID(t, "dst_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.PaymentDestinationIDPrefix),
		PayTo:                "0x1111111111111111111111111111111111111111",
		RequestMethod:        RequestMethodPost,
		RequestPath:          "/research",
		RequestBodyHash:      SHA256Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Amount:               domain.MustParseAmount("100"),
		Asset:                "USDC",
		Network:              "eip155:84532",
		MaximumAmount:        domain.MustParseAmount("100"),
		CreatedAt:            createdAt,
		ExpiresAt:            createdAt.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	repository := &intentTestRepository{purchaseIntent: purchaseIntent}
	repository.onUpdate = func(_ PurchaseIntent, _ uint64) error {
		claimed := purchaseIntent
		if err := claimed.Claim(purchaseIntent.ExpiresAt().Add(-time.Second)); err != nil {
			t.Fatal(err)
		}
		repository.purchaseIntent = claimed
		repository.onUpdate = nil
		return persistence.ErrConditionFailed
	}
	service := NewService(
		repository,
		&intentRouteRepository{},
		domain.NewULIDGenerator(domain.FixedClock{Value: createdAt.Time()}, nil),
		domain.FixedClock{Value: purchaseIntent.ExpiresAt().Time()},
	)

	_, err = service.Cancel(t.Context(), purchaseIntent.IntentID(), purchaseIntent.BuyerID())
	if !errors.Is(err, ErrIntentStateConflict) {
		t.Fatalf("Cancel() error = %v, want state conflict for executed winner", err)
	}
}

type intentAuthorizer struct {
	route       catalog.PaidRoute
	destination settlement.PaymentDestination
	calls       int
}

func (authorizer *intentAuthorizer) AuthorizeIntent(context.Context, domain.ID) (catalog.PaidRoute, settlement.PaymentDestination, error) {
	authorizer.calls++
	return authorizer.route, authorizer.destination, nil
}

type intentRouteRepository struct{ route catalog.PaidRoute }

func (repository *intentRouteRepository) GetRoute(context.Context, domain.ID) (catalog.PaidRoute, error) {
	return repository.route, nil
}

type intentTestRepository struct {
	purchaseIntent PurchaseIntent
	onUpdate       func(PurchaseIntent, uint64) error
}

func (repository *intentTestRepository) Create(_ context.Context, purchaseIntent PurchaseIntent) error {
	repository.purchaseIntent = purchaseIntent
	return nil
}
func (repository *intentTestRepository) Get(_ context.Context, intentID domain.ID) (PurchaseIntent, error) {
	if repository.purchaseIntent.IntentID() != intentID {
		return PurchaseIntent{}, persistence.ErrNotFound
	}
	return repository.purchaseIntent, nil
}
func (repository *intentTestRepository) Update(_ context.Context, purchaseIntent PurchaseIntent, expectedVersion uint64) error {
	if repository.onUpdate != nil {
		return repository.onUpdate(purchaseIntent, expectedVersion)
	}
	if repository.purchaseIntent.Version() != expectedVersion || purchaseIntent.Version() != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	repository.purchaseIntent = purchaseIntent
	return nil
}

func authorizedIntentRoute(t *testing.T, now domain.Timestamp) catalog.PaidRoute {
	t.Helper()
	route, err := catalog.NewPaidRoute(catalog.PaidRouteParams{RouteID: mustIntentID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix), SellerID: mustIntentID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix), DisplayName: "Research Report", ProductSlug: "research-report", Method: catalog.RouteMethodPost, PathPattern: "/research", Description: "Research", MIMEType: "application/json", Amount: domain.MustParseAmount("35000000"), Asset: "USDC", Network: "eip155:84532", PayTo: "0x1111111111111111111111111111111111111111", UpstreamTimeoutSeconds: 20, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	return route
}
