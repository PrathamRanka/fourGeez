package sellerworkspace

import (
	"context"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
)

func TestCredentialRepositoryReaderRedactsCredentialDigest(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	credential := integrations.RestoreCredential(integrations.Snapshot{
		CredentialID: fixture.credential.CredentialID, SellerID: fixture.seller.SellerID,
		TokenHash: "must-not-leave-repository", Label: "Primary", Scopes: []integrations.Scope{integrations.ScopeRead},
		CreatedAt: fixture.credential.CreatedAt, UpdatedAt: fixture.credential.UpdatedAt, Version: 1,
	})
	reader := NewCredentialRepositoryReader(credentialRepositoryStub{credentials: []integrations.Credential{credential}})
	views, err := reader.ListCredentials(context.Background(), fixture.seller.SellerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].CredentialID != credential.CredentialID() {
		t.Fatalf("views = %#v", views)
	}
}

type credentialRepositoryStub struct{ credentials []integrations.Credential }

func (repository credentialRepositoryStub) ListBySeller(context.Context, domain.ID) ([]integrations.Credential, error) {
	return repository.credentials, nil
}

func TestStripeBillingPortalKeepsProviderIntegrationBehindBoundary(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	client := &stripePortalClientStub{}
	portal := NewStripeBillingPortal(client)
	state, err := portal.PlanState(context.Background(), fixture.seller.SellerID)
	if err != nil || state.Provider != "stripe" || !state.PortalAvailable {
		t.Fatalf("state = %#v, error = %v", state, err)
	}
	session, err := portal.CreatePortalSession(context.Background(), fixture.seller.SellerID, "https://app.agentpay.example/dashboard/billing")
	if err != nil || session.URL == "" || client.sellerID != fixture.seller.SellerID {
		t.Fatalf("session = %#v, error = %v", session, err)
	}
}

type stripePortalClientStub struct{ sellerID domain.ID }

func (client *stripePortalClientStub) FetchSellerPlanState(_ context.Context, sellerID domain.ID) (ProviderPlanState, error) {
	client.sellerID = sellerID
	return ProviderPlanState{CustomerID: "cus_123", SubscriptionID: "sub_123", PriceID: "price_123", PortalAvailable: true}, nil
}
func (client *stripePortalClientStub) CreateCustomerPortalSession(_ context.Context, sellerID domain.ID, returnURL string) (BillingPortalSession, error) {
	client.sellerID = sellerID
	return BillingPortalSession{URL: returnURL + "?stripe=1", ExpiresAt: domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 5, 0, 0, time.UTC))}, nil
}
