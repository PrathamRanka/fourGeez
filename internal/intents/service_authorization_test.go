package intents

import (
	"context"
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

type intentTestRepository struct{ purchaseIntent PurchaseIntent }

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

func authorizedIntentRoute(t *testing.T, now domain.Timestamp) catalog.PaidRoute {
	t.Helper()
	route, err := catalog.NewPaidRoute(catalog.PaidRouteParams{RouteID: mustIntentID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix), SellerID: mustIntentID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix), DisplayName: "Research Report", ProductSlug: "research-report", Method: catalog.RouteMethodPost, PathPattern: "/research", Description: "Research", MIMEType: "application/json", Amount: domain.MustParseAmount("35000000"), Asset: "USDC", Network: "eip155:84532", PayTo: "0x1111111111111111111111111111111111111111", UpstreamTimeoutSeconds: 20, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	return route
}
