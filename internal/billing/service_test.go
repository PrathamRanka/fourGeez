package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

// TestPlanCatalogDefinesVersionedOperationalEntitlements verifies BIL-001 limits.
func TestPlanCatalogDefinesVersionedOperationalEntitlements(t *testing.T) {
	t.Parallel()

	plans := V1PlanCatalog()
	if len(plans) != 3 {
		t.Fatalf("plan count = %d, want 3", len(plans))
	}
	if plans[0].PlanID != PlanStarter ||
		plans[0].Limits.PublishedRoutes != 5 ||
		plans[0].Features.Approvals {
		t.Fatalf("starter plan = %#v", plans[0])
	}
	if plans[1].PlanID != PlanGrowth ||
		!plans[1].Features.Approvals ||
		!plans[1].Features.AdvancedAnalytics {
		t.Fatalf("growth plan = %#v", plans[1])
	}
	if plans[2].PlanID != PlanScale ||
		!plans[2].Features.PrioritySupport {
		t.Fatalf("scale plan = %#v", plans[2])
	}
}

// TestServiceEnsuresDefaultSellerPlan verifies deterministic starter assignment.
func TestServiceEnsuresDefaultSellerPlan(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	sellerID := mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	repository := newBillingRepository()
	service := NewService(
		repository,
		billingSellerAuthorizer{sellerID: sellerID},
		clock,
	)

	first, err := service.GetSellerPlan(
		context.Background(),
		"seller-user",
		sellerID,
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.GetSellerPlan(
		context.Background(),
		"seller-user",
		sellerID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Plan.PlanID != PlanStarter ||
		first.Assignment.Version != 1 ||
		first.Assignment.BillingPeriodStart.String() != "2026-09-01T00:00:00Z" ||
		first.Assignment.BillingPeriodEnd.String() != "2026-10-01T00:00:00Z" ||
		second.Assignment.Version != 1 ||
		repository.createCalls != 1 {
		t.Fatalf("seller plan results = (%#v, %#v), creates = %d", first, second, repository.createCalls)
	}
}

// TestServiceChangesPlanWithoutSettlementFields verifies the billing boundary.
func TestServiceChangesPlanWithoutSettlementFields(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	sellerID := mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	repository := newBillingRepository()
	service := NewService(
		repository,
		billingSellerAuthorizer{sellerID: sellerID},
		clock,
	)
	current, err := service.GetSellerPlan(context.Background(), "seller-user", sellerID)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.AssignPlan(
		context.Background(),
		sellerID,
		PlanGrowth,
		current.Assignment.Version,
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Plan.PlanID != PlanGrowth || updated.Assignment.Version != 2 {
		t.Fatalf("updated seller plan = %#v", updated)
	}
	_, err = service.AssignPlan(
		context.Background(),
		sellerID,
		PlanID("unlimited"),
		updated.Assignment.Version,
	)
	if !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("AssignPlan() error = %v, want ErrPlanNotFound", err)
	}
}

type billingRepository struct {
	assignments map[domain.ID]SellerPlan
	createCalls int
}

// newBillingRepository creates an empty seller-plan fixture repository.
func newBillingRepository() *billingRepository {
	return &billingRepository{assignments: make(map[domain.ID]SellerPlan)}
}

// Get returns one seller plan fixture.
func (repository *billingRepository) Get(
	_ context.Context,
	sellerID domain.ID,
) (SellerPlan, error) {
	assignment, exists := repository.assignments[sellerID]
	if !exists {
		return SellerPlan{}, ErrSellerPlanNotFound
	}
	return assignment, nil
}

// CreateIfAbsent stores one default seller plan exactly once.
func (repository *billingRepository) CreateIfAbsent(
	_ context.Context,
	assignment SellerPlan,
) (SellerPlan, bool, error) {
	if stored, exists := repository.assignments[assignment.SellerID()]; exists {
		return stored, false, nil
	}
	repository.assignments[assignment.SellerID()] = assignment
	repository.createCalls++
	return assignment, true, nil
}

// Update stores one optimistic plan change.
func (repository *billingRepository) Update(
	_ context.Context,
	assignment SellerPlan,
	expectedVersion uint64,
) error {
	stored, exists := repository.assignments[assignment.SellerID()]
	if !exists || stored.Version() != expectedVersion ||
		assignment.Version() != expectedVersion+1 {
		return ErrSellerPlanConflict
	}
	repository.assignments[assignment.SellerID()] = assignment
	return nil
}

type billingSellerAuthorizer struct {
	sellerID domain.ID
}

// AuthorizeSeller verifies the expected seller fixture.
func (authorizer billingSellerAuthorizer) AuthorizeSeller(
	_ context.Context,
	ownerSubject string,
	sellerID domain.ID,
) error {
	if ownerSubject != "seller-user" || sellerID != authorizer.sellerID {
		return ErrSellerPlanNotFound
	}
	return nil
}

// mustBillingID parses one stable seller fixture identifier.
func mustBillingID(t *testing.T, raw string) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, domain.SellerIDPrefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
