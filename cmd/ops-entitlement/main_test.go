package main

import (
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

func TestBuildGrantCandidateUsesOperatorBoundary(t *testing.T) {
	now := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)
	sellerID, err := domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	if err != nil {
		t.Fatal(err)
	}
	accessEndsAt := now.Add(30 * 24 * time.Hour)
	candidate, err := buildCandidate(operationOptions{
		Action:       actionGrant,
		SellerID:     sellerID,
		PlanID:       billing.PlanStarter,
		AccessEndsAt: accessEndsAt,
	}, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Status != billing.EntitlementStatusActive ||
		candidate.Source != billing.EntitlementSourceOperator ||
		candidate.Provider != billing.EntitlementProviderOperator ||
		!candidate.AccessEndsAt.Time().Equal(accessEndsAt) {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestBuildGrantCandidateRequiresUTCExpiry(t *testing.T) {
	now := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)
	sellerID, _ := domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	_, err := buildCandidate(operationOptions{
		Action: actionGrant, SellerID: sellerID, PlanID: billing.PlanStarter,
		AccessEndsAt: time.Date(2026, time.October, 19, 5, 30, 0, 0, time.FixedZone("IST", 5*60*60+30*60)),
	}, nil, now)
	if err == nil {
		t.Fatal("non-UTC access boundary was accepted")
	}
}

func TestBuildSuspensionCandidatePreservesCommercialPeriod(t *testing.T) {
	now := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)
	sellerID, _ := domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	current, _, err := billing.ReconcileSellerEntitlement(nil, billing.EntitlementCandidate{
		SellerID: sellerID, PlanID: billing.PlanGrowth, PlanVersion: 1,
		Status:             billing.EntitlementStatusActive,
		BillingPeriodStart: domain.NewTimestamp(now.Add(-24 * time.Hour)),
		BillingPeriodEnd:   domain.NewTimestamp(now.Add(29 * 24 * time.Hour)),
		AccessEndsAt:       domain.NewTimestamp(now.Add(29 * 24 * time.Hour)),
		Source:             billing.EntitlementSourceOperator, Provider: billing.EntitlementProviderOperator,
	}, domain.NewTimestamp(now.Add(-24*time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := buildCandidate(operationOptions{
		Action: actionSuspend, SellerID: sellerID, ExpectedVersion: 1,
	}, &current, now)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Status != billing.EntitlementStatusSuspended ||
		candidate.StatusReason != billing.EntitlementStatusReasonAdministrative ||
		candidate.PlanID != current.PlanID() ||
		candidate.AccessEndsAt != current.AccessEndsAt() {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestConfirmationPhraseBindsActionSellerAndVersion(t *testing.T) {
	sellerID, _ := domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	if got := confirmationPhrase(actionGrant, sellerID, 0); got != "grant:sel_01K5D09YJ0C0M7RJM4FWQ0K9H7:0" {
		t.Fatalf("confirmationPhrase() = %q", got)
	}
}

func TestCallerUsesConfiguredLaunchEntitlementRole(t *testing.T) {
	roleARN := "arn:aws:iam::937319036732:role/agentpay-dev-launch-entitlement-operator"
	callerARN := "arn:aws:sts::937319036732:assumed-role/agentpay-dev-launch-entitlement-operator/session-1"
	if err := validateOperatorCaller(callerARN, roleARN); err != nil {
		t.Fatal(err)
	}
	if err := validateOperatorCaller("arn:aws:iam::937319036732:user/admin", roleARN); err == nil {
		t.Fatal("direct administrator was accepted instead of the configured operator role")
	}
}
