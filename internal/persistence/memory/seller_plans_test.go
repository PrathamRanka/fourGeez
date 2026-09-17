package memory

import (
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// TestSellerPlanRepositoryUsesIdempotentCreateAndOptimisticUpdate verifies safety.
func TestSellerPlanRepositoryUsesIdempotentCreateAndOptimisticUpdate(t *testing.T) {
	t.Parallel()

	repository := NewSellerPlanRepository()
	assignment := testMemorySellerPlan(t)
	stored, created, err := repository.CreateIfAbsent(t.Context(), assignment)
	if err != nil || !created || stored.Version() != 1 {
		t.Fatalf("CreateIfAbsent() = (%#v, %v, %v)", stored, created, err)
	}
	stored, created, err = repository.CreateIfAbsent(t.Context(), assignment)
	if err != nil || created || stored.Version() != 1 {
		t.Fatalf("duplicate CreateIfAbsent() = (%#v, %v, %v)", stored, created, err)
	}
	if err := assignment.ChangePlan(
		billing.PlanGrowth,
		1,
		assignment.UpdatedAt().Add(time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if err := repository.Update(t.Context(), assignment, 9); err != persistence.ErrConditionFailed {
		t.Fatalf("Update() error = %v", err)
	}
	if err := repository.Update(t.Context(), assignment, 1); err != nil {
		t.Fatal(err)
	}
}

// testMemorySellerPlan creates one valid assignment fixture.
func testMemorySellerPlan(t *testing.T) billing.SellerPlan {
	t.Helper()
	assignedAt := domain.NewTimestamp(
		time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	)
	assignment, err := billing.NewSellerPlan(billing.SellerPlanParams{
		SellerID:           mustMemoryID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		PlanID:             billing.PlanStarter,
		PlanVersion:        1,
		Status:             billing.SellerPlanStatusActive,
		BillingPeriodStart: domain.NewTimestamp(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)),
		BillingPeriodEnd:   domain.NewTimestamp(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)),
		AssignedAt:         assignedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return assignment
}
