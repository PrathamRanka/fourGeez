package sellerworkspace

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
)

type CurrentSellerFinder interface {
	GetCurrentSeller(context.Context, string) (catalog.SellerResponse, error)
}

type AccountVerificationReader interface {
	EmailVerified(context.Context, string) (bool, error)
}

type ContextPrincipalSource struct {
	sellers      CurrentSellerFinder
	verification AccountVerificationReader
}

func NewContextPrincipalSource(sellers CurrentSellerFinder, verification AccountVerificationReader) *ContextPrincipalSource {
	return &ContextPrincipalSource{sellers: sellers, verification: verification}
}

func (source *ContextPrincipalSource) CurrentPrincipal(ctx context.Context) (Principal, error) {
	principal, ok := api.PrincipalFromContext(ctx)
	if !ok || principal.Kind != api.PrincipalSeller {
		return Principal{}, ErrAuthenticationRequired
	}
	seller, err := source.sellers.GetCurrentSeller(ctx, principal.Subject)
	if err != nil {
		return Principal{}, err
	}
	verified := false
	if source.verification != nil {
		verified, err = source.verification.EmailVerified(ctx, principal.Subject)
		if err != nil {
			return Principal{}, err
		}
	}
	return Principal{Subject: principal.Subject, SellerID: &seller.SellerID, EmailVerified: verified}, nil
}

type StaticAccountVerification bool

func (verification StaticAccountVerification) EmailVerified(context.Context, string) (bool, error) {
	return bool(verification), nil
}

type UnavailableBillingPortal struct{}

func (UnavailableBillingPortal) PlanState(context.Context, domain.ID) (ProviderPlanState, error) {
	return ProviderPlanState{Provider: "stripe", PortalAvailable: false}, nil
}

func (UnavailableBillingPortal) CreatePortalSession(context.Context, domain.ID, string) (BillingPortalSession, error) {
	return BillingPortalSession{}, errors.New("billing portal is unavailable")
}
