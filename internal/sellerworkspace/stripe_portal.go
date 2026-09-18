package sellerworkspace

import (
	"context"

	"github.com/fourgeez/agentpay/internal/domain"
)

type StripePortalClient interface {
	FetchSellerPlanState(context.Context, domain.ID) (ProviderPlanState, error)
	CreateCustomerPortalSession(context.Context, domain.ID, string) (BillingPortalSession, error)
}

type StripeBillingPortal struct {
	client StripePortalClient
}

func NewStripeBillingPortal(client StripePortalClient) *StripeBillingPortal {
	return &StripeBillingPortal{client: client}
}

func (portal *StripeBillingPortal) PlanState(ctx context.Context, sellerID domain.ID) (ProviderPlanState, error) {
	state, err := portal.client.FetchSellerPlanState(ctx, sellerID)
	if err != nil {
		return ProviderPlanState{}, err
	}
	state.Provider = "stripe"
	return state, nil
}

func (portal *StripeBillingPortal) CreatePortalSession(ctx context.Context, sellerID domain.ID, returnURL string) (BillingPortalSession, error) {
	return portal.client.CreateCustomerPortalSession(ctx, sellerID, returnURL)
}
