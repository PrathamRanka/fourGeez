package memory

import (
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestSellerEntitlementRepositoryAppliesProjectionAndHistoryAtomically(t *testing.T) {
	t.Parallel()

	repository := NewSellerEntitlementRepository()
	entitlement, reconciliation := testMemoryEntitlement(t, nil, memoryTime(2026, time.September, 18, 10))
	if err := repository.Apply(t.Context(), entitlement, reconciliation, 0); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.Get(t.Context(), entitlement.SellerID())
	if err != nil || stored.SourceRevision() != "00000000000000000001" {
		t.Fatalf("Get() = (%#v, %v)", stored.Snapshot(), err)
	}
	if err := repository.Apply(t.Context(), entitlement, reconciliation, 0); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("duplicate Apply() error = %v", err)
	}
	updated, updatedReconciliation := testMemoryEntitlement(t, &stored, memoryTime(2026, time.September, 18, 11))
	if err := repository.Apply(t.Context(), updated, updatedReconciliation, 9); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("stale Apply() error = %v", err)
	}
	if err := repository.Apply(t.Context(), updated, updatedReconciliation, stored.Version()); err != nil {
		t.Fatal(err)
	}
}

func testMemoryEntitlement(t *testing.T, current *billing.SellerEntitlement, now domain.Timestamp) (billing.SellerEntitlement, billing.EntitlementReconciliation) {
	t.Helper()
	sellerID := mustMemoryID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	entitlement, reconciliation, err := billing.ReconcileSellerEntitlement(current, billing.EntitlementCandidate{
		SellerID: sellerID, PlanID: billing.PlanStarter, PlanVersion: 1, Status: billing.EntitlementStatusActive,
		BillingPeriodStart: memoryTime(2026, time.September, 1, 0), BillingPeriodEnd: memoryTime(2026, time.October, 1, 0), AccessEndsAt: memoryTime(2026, time.October, 1, 0),
		Source: billing.EntitlementSourceBillingProvider, Provider: billing.EntitlementProviderStripe,
		ProviderCustomerID: "cus_123", ProviderSubscriptionID: "sub_123", ProviderPriceID: "price_123", LastProviderEventID: "evt_123",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	return entitlement, reconciliation
}

func memoryTime(year int, month time.Month, day int, hour int) domain.Timestamp {
	return domain.NewTimestamp(time.Date(year, month, day, hour, 0, 0, 0, time.UTC))
}
