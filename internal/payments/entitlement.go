package payments

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

var (
	ErrSubscriptionInactive             = errors.New("seller subscription is inactive")
	ErrCommerceAuthorizationUnavailable = errors.New("commerce authorization is unavailable")
)

type SellerEntitlementReader interface {
	Get(context.Context, domain.ID) (billing.SellerEntitlement, error)
}

type AuthoritativeCommerceAuthorizer struct {
	repository SellerEntitlementReader
	clock      domain.Clock
}

func NewAuthoritativeCommerceAuthorizer(
	repository SellerEntitlementReader,
	clock domain.Clock,
) *AuthoritativeCommerceAuthorizer {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &AuthoritativeCommerceAuthorizer{repository: repository, clock: clock}
}

func (authorizer *AuthoritativeCommerceAuthorizer) AuthorizeCommerce(
	ctx context.Context,
	sellerID domain.ID,
	_ CommerceOperation,
) error {
	if authorizer == nil || authorizer.repository == nil {
		return ErrCommerceAuthorizationUnavailable
	}
	entitlement, err := authorizer.repository.Get(ctx, sellerID)
	if errors.Is(err, billing.ErrSellerEntitlementNotFound) || errors.Is(err, persistence.ErrNotFound) {
		return ErrSubscriptionInactive
	}
	if err != nil {
		return errors.Join(ErrCommerceAuthorizationUnavailable, err)
	}
	if entitlement.SellerID() != sellerID ||
		!entitlement.AllowsNetworkAccess(domain.NewTimestamp(authorizer.clock.Now())) {
		return ErrSubscriptionInactive
	}
	return nil
}

var _ CommerceAuthorizer = (*AuthoritativeCommerceAuthorizer)(nil)
