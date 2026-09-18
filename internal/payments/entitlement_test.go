package payments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestAuthoritativeCommerceAuthorizerFailsClosed(t *testing.T) {
	t.Parallel()

	now := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	sellerID := mustPaymentID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	tests := []struct {
		name            string
		entitlement     billing.SellerEntitlement
		repositoryError error
		wantError       error
	}{
		{name: "active", entitlement: paymentEntitlement(t, sellerID, billing.EntitlementStatusActive, now.Add(time.Hour))},
		{name: "expired boundary", entitlement: paymentEntitlement(t, sellerID, billing.EntitlementStatusActive, now), wantError: ErrSubscriptionInactive},
		{name: "grace", entitlement: paymentEntitlement(t, sellerID, billing.EntitlementStatusGrace, now.Add(time.Hour)), wantError: ErrSubscriptionInactive},
		{name: "missing", repositoryError: persistence.ErrNotFound, wantError: ErrSubscriptionInactive},
		{name: "unavailable", repositoryError: errors.New("database unavailable"), wantError: ErrCommerceAuthorizationUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer := NewAuthoritativeCommerceAuthorizer(
				&paymentEntitlementReader{entitlement: test.entitlement, err: test.repositoryError},
				domain.FixedClock{Value: now.Time()},
			)
			err := authorizer.AuthorizeCommerce(t.Context(), sellerID, CommerceOperationSettlement)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("AuthorizeCommerce() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

type paymentEntitlementReader struct {
	entitlement billing.SellerEntitlement
	err         error
}

func (reader *paymentEntitlementReader) Get(context.Context, domain.ID) (billing.SellerEntitlement, error) {
	return reader.entitlement, reader.err
}

func paymentEntitlement(
	t *testing.T,
	sellerID domain.ID,
	status billing.EntitlementStatus,
	accessEndsAt domain.Timestamp,
) billing.SellerEntitlement {
	t.Helper()
	assignedAt := accessEndsAt.Add(-30 * 24 * time.Hour)
	var graceEndsAt *domain.Timestamp
	if status == billing.EntitlementStatusGrace {
		value := accessEndsAt.Add(billing.EntitlementGraceDuration)
		graceEndsAt = &value
	}
	entitlement, err := billing.NewSellerEntitlement(billing.SellerEntitlementParams{
		SellerID: sellerID, PlanID: billing.PlanStarter, PlanVersion: 1, Status: status,
		BillingPeriodStart: assignedAt, BillingPeriodEnd: accessEndsAt, AccessEndsAt: accessEndsAt, GraceEndsAt: graceEndsAt,
		EntitlementEpoch: 1, Source: billing.EntitlementSourceLocal, SourceRevision: "00000000000000000001",
		Provider: billing.EntitlementProviderLocal, AssignedAt: assignedAt, UpdatedAt: assignedAt, Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return entitlement
}
