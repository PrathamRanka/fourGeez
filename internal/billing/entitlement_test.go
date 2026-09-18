package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestSellerEntitlementNetworkAuthorityUsesExclusiveAccessBoundary(t *testing.T) {
	t.Parallel()

	accessEndsAt := entitlementTime(2026, time.October, 1, 0)
	entitlement := mustNewEntitlement(t, SellerEntitlementParams{
		SellerID:                   mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		PlanID:                     PlanGrowth,
		PlanVersion:                1,
		Status:                     EntitlementStatusActive,
		BillingPeriodStart:         entitlementTime(2026, time.September, 1, 0),
		BillingPeriodEnd:           accessEndsAt,
		AccessEndsAt:               accessEndsAt,
		EntitlementEpoch:           7,
		Source:                     EntitlementSourceBillingProvider,
		SourceRevision:             "00000000000000000007",
		Provider:                   EntitlementProviderStripe,
		ProviderCustomerID:         "cus_123",
		ProviderSubscriptionID:     "sub_123",
		ProviderPriceID:            "price_123",
		CredentialRotationRequired: false,
		AssignedAt:                 entitlementTime(2026, time.September, 1, 0),
		UpdatedAt:                  entitlementTime(2026, time.September, 18, 10),
		Version:                    4,
	})

	tests := []struct {
		name string
		now  domain.Timestamp
		want bool
	}{
		{name: "before", now: accessEndsAt.Add(-time.Nanosecond), want: true},
		{name: "equal", now: accessEndsAt, want: false},
		{name: "after", now: accessEndsAt.Add(time.Nanosecond), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := entitlement.AllowsNetworkAccess(test.now); got != test.want {
				t.Fatalf("AllowsNetworkAccess() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestReconcileSellerEntitlementAppliesLifecycleAndMonotonicRevisions(t *testing.T) {
	t.Parallel()

	sellerID := mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	accessEndsAt := entitlementTime(2026, time.October, 1, 0)
	initial, _, err := ReconcileSellerEntitlement(nil, EntitlementCandidate{
		SellerID:               sellerID,
		PlanID:                 PlanGrowth,
		PlanVersion:            1,
		Status:                 EntitlementStatusActive,
		BillingPeriodStart:     entitlementTime(2026, time.September, 1, 0),
		BillingPeriodEnd:       accessEndsAt,
		AccessEndsAt:           accessEndsAt,
		Source:                 EntitlementSourceBillingProvider,
		Provider:               EntitlementProviderStripe,
		ProviderCustomerID:     "cus_123",
		ProviderSubscriptionID: "sub_123",
		ProviderPriceID:        "price_growth",
		LastProviderEventID:    "evt_paid",
	}, entitlementTime(2026, time.September, 18, 10))
	if err != nil {
		t.Fatal(err)
	}
	if initial.EntitlementEpoch() != 1 || initial.SourceRevision() != "00000000000000000001" || initial.Version() != 1 {
		t.Fatalf("initial counters = epoch %d revision %q version %d", initial.EntitlementEpoch(), initial.SourceRevision(), initial.Version())
	}

	grace, _, err := ReconcileSellerEntitlement(&initial, EntitlementCandidate{
		SellerID:               sellerID,
		PlanID:                 PlanGrowth,
		PlanVersion:            1,
		Status:                 EntitlementStatusGrace,
		BillingPeriodStart:     initial.BillingPeriodStart(),
		BillingPeriodEnd:       initial.BillingPeriodEnd(),
		AccessEndsAt:           initial.AccessEndsAt(),
		Source:                 EntitlementSourceBillingProvider,
		Provider:               EntitlementProviderStripe,
		ProviderCustomerID:     "cus_123",
		ProviderSubscriptionID: "sub_123",
		ProviderPriceID:        "price_growth",
		StatusReason:           EntitlementStatusReasonPaymentFailed,
		LastProviderEventID:    "evt_failed",
	}, accessEndsAt)
	if err != nil {
		t.Fatal(err)
	}
	if grace.Status() != EntitlementStatusGrace || grace.GraceEndsAt() == nil || grace.GraceEndsAt().Time().Sub(accessEndsAt.Time()) != 72*time.Hour {
		t.Fatalf("grace projection = %#v", grace.Snapshot())
	}
	if grace.EntitlementEpoch() != 2 || grace.SourceRevision() != "00000000000000000002" || grace.Version() != 2 {
		t.Fatalf("grace counters = epoch %d revision %q version %d", grace.EntitlementEpoch(), grace.SourceRevision(), grace.Version())
	}

	suspended, _, err := ReconcileSellerEntitlement(&grace, EntitlementCandidate{
		SellerID:               sellerID,
		PlanID:                 PlanGrowth,
		PlanVersion:            1,
		Status:                 EntitlementStatusGrace,
		BillingPeriodStart:     grace.BillingPeriodStart(),
		BillingPeriodEnd:       grace.BillingPeriodEnd(),
		AccessEndsAt:           grace.AccessEndsAt(),
		Source:                 EntitlementSourceBillingProvider,
		Provider:               EntitlementProviderStripe,
		ProviderCustomerID:     "cus_123",
		ProviderSubscriptionID: "sub_123",
		ProviderPriceID:        "price_growth",
		StatusReason:           EntitlementStatusReasonPaymentFailed,
	}, accessEndsAt.Add(72*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if suspended.Status() != EntitlementStatusSuspended || suspended.GraceEndsAt() != nil || suspended.EntitlementEpoch() != 3 {
		t.Fatalf("expired grace projection = %#v", suspended.Snapshot())
	}

	reactivated, _, err := ReconcileSellerEntitlement(&suspended, EntitlementCandidate{
		SellerID:               sellerID,
		PlanID:                 PlanScale,
		PlanVersion:            1,
		Status:                 EntitlementStatusActive,
		BillingPeriodStart:     entitlementTime(2026, time.October, 4, 0),
		BillingPeriodEnd:       entitlementTime(2026, time.November, 4, 0),
		AccessEndsAt:           entitlementTime(2026, time.November, 4, 0),
		Source:                 EntitlementSourceBillingProvider,
		Provider:               EntitlementProviderStripe,
		ProviderCustomerID:     "cus_123",
		ProviderSubscriptionID: "sub_456",
		ProviderPriceID:        "price_scale",
	}, entitlementTime(2026, time.October, 4, 1))
	if err != nil {
		t.Fatal(err)
	}
	if reactivated.Status() != EntitlementStatusActive || !reactivated.CredentialRotationRequired() || reactivated.EntitlementEpoch() != 4 {
		t.Fatalf("reactivated projection = %#v", reactivated.Snapshot())
	}
}

func TestReconcileSellerEntitlementKeepsScheduledCancellationActiveUntilBoundary(t *testing.T) {
	t.Parallel()

	accessEndsAt := entitlementTime(2026, time.October, 1, 0)
	candidate := EntitlementCandidate{
		SellerID:               mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		PlanID:                 PlanStarter,
		PlanVersion:            1,
		Status:                 EntitlementStatusActive,
		BillingPeriodStart:     entitlementTime(2026, time.September, 1, 0),
		BillingPeriodEnd:       accessEndsAt,
		AccessEndsAt:           accessEndsAt,
		CancelAtPeriodEnd:      true,
		Source:                 EntitlementSourceBillingProvider,
		Provider:               EntitlementProviderStripe,
		ProviderCustomerID:     "cus_123",
		ProviderSubscriptionID: "sub_123",
		ProviderPriceID:        "price_starter",
		StatusReason:           EntitlementStatusReasonCancellationRequested,
	}
	active, _, err := ReconcileSellerEntitlement(nil, candidate, accessEndsAt.Add(-time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	if active.Status() != EntitlementStatusActive || !active.CancelAtPeriodEnd() {
		t.Fatalf("before boundary = %#v", active.Snapshot())
	}
	cancelled, _, err := ReconcileSellerEntitlement(&active, candidate, accessEndsAt)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status() != EntitlementStatusCancelled || cancelled.GraceEndsAt() != nil || cancelled.EntitlementEpoch() != 2 {
		t.Fatalf("at boundary = %#v", cancelled.Snapshot())
	}
}

func TestServiceFailsClosedForMissingEntitlement(t *testing.T) {
	t.Parallel()

	service := NewService(newBillingRepository(), billingSellerAuthorizer{}, domain.FixedClock{Value: time.Now().UTC()})
	_, err := service.ResolveSellerPlan(t.Context(), mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"))
	if !errors.Is(err, ErrSellerEntitlementNotFound) {
		t.Fatalf("ResolveSellerPlan() error = %v, want ErrSellerEntitlementNotFound", err)
	}
}

func TestServicePersistsBeforeInvalidatingEntitlementCache(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 2)
	repository := &orderedEntitlementRepository{order: &order}
	service := NewService(repository, billingSellerAuthorizer{}, domain.FixedClock{Value: entitlementTime(2026, time.September, 18, 10).Time()})
	service.SetEntitlementInvalidator(orderedEntitlementInvalidator{order: &order})
	_, err := service.ReconcileEntitlement(t.Context(), EntitlementCandidate{
		SellerID:               mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		PlanID:                 PlanStarter,
		PlanVersion:            1,
		Status:                 EntitlementStatusActive,
		BillingPeriodStart:     entitlementTime(2026, time.September, 1, 0),
		BillingPeriodEnd:       entitlementTime(2026, time.October, 1, 0),
		AccessEndsAt:           entitlementTime(2026, time.October, 1, 0),
		Source:                 EntitlementSourceBillingProvider,
		Provider:               EntitlementProviderStripe,
		ProviderCustomerID:     "cus_123",
		ProviderSubscriptionID: "sub_123",
		ProviderPriceID:        "price_starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "persist" || order[1] != "invalidate" {
		t.Fatalf("operation order = %v", order)
	}
}

func TestStripeReconciliationCannotClearFraudQuarantine(t *testing.T) {
	t.Parallel()

	now := entitlementTime(2026, time.September, 18, 10)
	current := mustNewEntitlement(t, SellerEntitlementParams{
		SellerID: mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"), PlanID: PlanGrowth, PlanVersion: 1,
		Status: EntitlementStatusSuspended, BillingPeriodStart: entitlementTime(2026, time.September, 1, 0),
		BillingPeriodEnd: entitlementTime(2026, time.October, 1, 0), AccessEndsAt: entitlementTime(2026, time.October, 1, 0),
		EntitlementEpoch: 8, Source: EntitlementSourceOperator, SourceRevision: "00000000000000000008",
		StatusReason: EntitlementStatusReasonFraudQuarantine, Provider: EntitlementProviderStripe,
		ProviderCustomerID: "cus_123", ProviderSubscriptionID: "sub_123", ProviderPriceID: "price_growth",
		CredentialRotationRequired: true, AssignedAt: entitlementTime(2026, time.September, 1, 0), UpdatedAt: now, Version: 5,
	})
	candidate := candidateFromEntitlement(current)
	candidate.Status = EntitlementStatusActive
	candidate.StatusReason = ""
	candidate.Source = EntitlementSourceBillingProvider
	reconciled, _, err := ReconcileSellerEntitlement(&current, candidate, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Status() != EntitlementStatusSuspended || reconciled.StatusReason() != EntitlementStatusReasonFraudQuarantine || reconciled.EntitlementEpoch() != current.EntitlementEpoch() {
		t.Fatalf("Stripe cleared quarantine: %#v", reconciled.Snapshot())
	}
}

func TestServiceAppendsEntitlementSecurityAuditAfterPersistence(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 2)
	repository := &orderedEntitlementRepository{order: &order}
	recorder := &entitlementAuditRecorder{order: &order}
	service := NewService(repository, billingSellerAuthorizer{}, domain.FixedClock{Value: entitlementTime(2026, time.September, 18, 10).Time()})
	service.SetAuditRecorder(recorder)
	_, err := service.ReconcileEntitlement(t.Context(), EntitlementCandidate{
		SellerID: mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"), PlanID: PlanStarter, PlanVersion: 1,
		Status: EntitlementStatusActive, BillingPeriodStart: entitlementTime(2026, time.September, 1, 0),
		BillingPeriodEnd: entitlementTime(2026, time.October, 1, 0), AccessEndsAt: entitlementTime(2026, time.October, 1, 0),
		Source: EntitlementSourceBillingProvider, Provider: EntitlementProviderStripe,
		ProviderCustomerID: "cus_123", ProviderSubscriptionID: "sub_123", ProviderPriceID: "price_123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(order) < 2 || order[0] != "persist" || order[1] != "audit" || len(recorder.requests) != 1 || recorder.requests[0].Action != audit.ActionEntitlementChanged {
		t.Fatalf("audit order/requests = %v/%#v", order, recorder.requests)
	}
}

type orderedEntitlementRepository struct {
	order       *[]string
	entitlement *SellerEntitlement
}

func (repository *orderedEntitlementRepository) Get(context.Context, domain.ID) (SellerEntitlement, error) {
	if repository.entitlement == nil {
		return SellerEntitlement{}, persistence.ErrNotFound
	}
	return *repository.entitlement, nil
}

func (repository *orderedEntitlementRepository) Apply(
	_ context.Context,
	entitlement SellerEntitlement,
	_ EntitlementReconciliation,
	_ uint64,
) error {
	*repository.order = append(*repository.order, "persist")
	repository.entitlement = &entitlement
	return nil
}

type orderedEntitlementInvalidator struct{ order *[]string }

func (invalidator orderedEntitlementInvalidator) InvalidateSellerEntitlement(context.Context, domain.ID) error {
	*invalidator.order = append(*invalidator.order, "invalidate")
	return nil
}

type entitlementAuditRecorder struct {
	order    *[]string
	requests []audit.RecordRequest
}

func (recorder *entitlementAuditRecorder) Record(_ context.Context, request audit.RecordRequest) error {
	*recorder.order = append(*recorder.order, "audit")
	recorder.requests = append(recorder.requests, request)
	return nil
}

func mustNewEntitlement(t *testing.T, params SellerEntitlementParams) SellerEntitlement {
	t.Helper()
	entitlement, err := NewSellerEntitlement(params)
	if err != nil {
		t.Fatal(err)
	}
	return entitlement
}

func entitlementTime(year int, month time.Month, day int, hour int) domain.Timestamp {
	return domain.NewTimestamp(time.Date(year, month, day, hour, 0, 0, 0, time.UTC))
}
