package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestPlanCatalogDefinesVersionedOperationalEntitlements(t *testing.T) {
	t.Parallel()
	plans := V1PlanCatalog()
	if len(plans) != 3 || plans[0].PlanID != PlanStarter || plans[1].PlanID != PlanGrowth || plans[2].PlanID != PlanScale {
		t.Fatalf("plan catalog = %#v", plans)
	}
	if plans[0].Features.Approvals || !plans[1].Features.AdvancedAnalytics || !plans[2].Features.PrioritySupport {
		t.Fatalf("plan features = %#v", plans)
	}
}

func TestServiceReadsExistingEntitlementWithoutCreatingDefault(t *testing.T) {
	t.Parallel()
	now := entitlementTime(2026, time.September, 18, 10)
	sellerID := mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	repository := newBillingRepository()
	seedBillingEntitlement(t, repository, sellerID, now)
	service := NewService(repository, billingSellerAuthorizer{sellerID: sellerID}, domain.FixedClock{Value: now.Time()})

	response, err := service.GetSellerPlan(t.Context(), "seller-user", sellerID)
	if err != nil {
		t.Fatal(err)
	}
	if response.Assignment.PlanID != PlanStarter || response.Assignment.NetworkAccess != NetworkAccessEnabled || repository.applyCalls != 1 {
		t.Fatalf("seller entitlement = %#v, applies = %d", response, repository.applyCalls)
	}
}

func TestServiceChangesPlanWithOptimisticConcurrency(t *testing.T) {
	t.Parallel()
	now := entitlementTime(2026, time.September, 18, 10)
	sellerID := mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	repository := newBillingRepository()
	seedBillingEntitlement(t, repository, sellerID, now)
	service := NewService(repository, billingSellerAuthorizer{sellerID: sellerID}, domain.FixedClock{Value: now.Add(time.Minute).Time()})

	updated, err := service.AssignPlan(t.Context(), sellerID, PlanGrowth, 1)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Plan.PlanID != PlanGrowth || updated.Assignment.Version != 2 || updated.Assignment.SourceRevision != "00000000000000000002" {
		t.Fatalf("updated entitlement = %#v", updated)
	}
	_, err = service.AssignPlan(t.Context(), sellerID, PlanScale, 1)
	if !errors.Is(err, ErrSellerEntitlementConflict) {
		t.Fatalf("stale AssignPlan() error = %v", err)
	}
}

type billingRepository struct {
	assignments     map[domain.ID]SellerEntitlement
	reconciliations map[string]EntitlementReconciliation
	applyCalls      int
}

func newBillingRepository() *billingRepository {
	return &billingRepository{assignments: make(map[domain.ID]SellerEntitlement), reconciliations: make(map[string]EntitlementReconciliation)}
}

func (repository *billingRepository) Get(_ context.Context, sellerID domain.ID) (SellerEntitlement, error) {
	entitlement, exists := repository.assignments[sellerID]
	if !exists {
		return SellerEntitlement{}, persistence.ErrNotFound
	}
	return entitlement, nil
}

func (repository *billingRepository) Apply(_ context.Context, entitlement SellerEntitlement, reconciliation EntitlementReconciliation, expectedVersion uint64) error {
	stored, exists := repository.assignments[entitlement.SellerID()]
	if (!exists && expectedVersion != 0) || (exists && stored.Version() != expectedVersion) || entitlement.Version() != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	if exists && entitlement.SourceRevision() <= stored.SourceRevision() {
		return persistence.ErrConditionFailed
	}
	repository.assignments[entitlement.SellerID()] = entitlement
	repository.reconciliations[reconciliation.SourceRevision] = reconciliation
	repository.applyCalls++
	return nil
}

func seedBillingEntitlement(t *testing.T, repository *billingRepository, sellerID domain.ID, now domain.Timestamp) SellerEntitlement {
	t.Helper()
	service := NewService(repository, billingSellerAuthorizer{sellerID: sellerID}, domain.FixedClock{Value: now.Time()})
	response, err := service.ReconcileEntitlement(t.Context(), EntitlementCandidate{
		SellerID: sellerID, PlanID: PlanStarter, PlanVersion: 1, Status: EntitlementStatusActive,
		BillingPeriodStart: entitlementTime(2026, time.September, 1, 0), BillingPeriodEnd: entitlementTime(2026, time.October, 1, 0), AccessEndsAt: entitlementTime(2026, time.October, 1, 0),
		Source: EntitlementSourceBillingProvider, Provider: EntitlementProviderStripe,
		ProviderCustomerID: "cus_123", ProviderSubscriptionID: "sub_123", ProviderPriceID: "price_starter", LastProviderEventID: "evt_seed",
	})
	if err != nil {
		t.Fatal(err)
	}
	return repository.assignments[response.Assignment.SellerID]
}

type billingSellerAuthorizer struct{ sellerID domain.ID }

func (authorizer billingSellerAuthorizer) AuthorizeSeller(_ context.Context, ownerSubject string, sellerID domain.ID) error {
	if ownerSubject != "seller-user" || sellerID != authorizer.sellerID {
		return ErrSellerEntitlementNotFound
	}
	return nil
}

func mustBillingID(t *testing.T, raw string) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, domain.SellerIDPrefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
