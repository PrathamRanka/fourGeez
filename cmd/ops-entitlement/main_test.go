package main

import (
	"strings"
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

func TestOperationRequestDigestBindsEnvironmentVersionAndSeller(t *testing.T) {
	base := testOperationOptions(t)
	digest, err := operationRequestDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*operationOptions)
	}{
		{name: "environment", mutate: func(options *operationOptions) { options.Environment = "demo" }},
		{name: "version", mutate: func(options *operationOptions) { options.ExpectedVersion++ }},
		{name: "seller", mutate: func(options *operationOptions) {
			options.SellerID, _ = domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.SellerIDPrefix)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := base
			test.mutate(&changed)
			changedDigest, err := operationRequestDigest(changed)
			if err != nil {
				t.Fatal(err)
			}
			if changedDigest == digest {
				t.Fatalf("digest did not bind %s", test.name)
			}
		})
	}
}

func TestExactOperationReplayIsIdempotent(t *testing.T) {
	options := testOperationOptions(t)
	requestDigest, err := operationRequestDigest(options)
	if err != nil {
		t.Fatal(err)
	}
	stored := billing.LaunchEntitlementOperation{
		OperationID: options.OperationID, SchemaVersion: billing.LaunchEntitlementOperationSchemaVersion,
		Environment: options.Environment, SellerID: options.SellerID,
		RequestSHA256: requestDigest, PlanSHA256: strings.Repeat("a", 64), AppliedVersion: 3,
	}
	replay, err := validateReplay(stored, options, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if replay.AppliedVersion != 3 {
		t.Fatalf("replay = %#v", replay)
	}
}

func TestOperationReplayRejectsChangedRequest(t *testing.T) {
	options := testOperationOptions(t)
	requestDigest, err := operationRequestDigest(options)
	if err != nil {
		t.Fatal(err)
	}
	stored := billing.LaunchEntitlementOperation{
		OperationID: options.OperationID, SchemaVersion: billing.LaunchEntitlementOperationSchemaVersion,
		Environment: options.Environment, SellerID: options.SellerID,
		RequestSHA256: requestDigest, PlanSHA256: strings.Repeat("a", 64), AppliedVersion: 3,
	}
	options.ExpectedVersion++
	if _, err := validateReplay(stored, options, strings.Repeat("a", 64)); err == nil {
		t.Fatal("changed replay was accepted")
	}
}

func TestEnvironmentBindingRejectsWrongEnvironmentResources(t *testing.T) {
	if err := validateEnvironmentBinding("agentpay", "dev", "123456789012", "agentpay-demo-main", "arn:aws:iam::123456789012:role/agentpay-dev-launch-entitlement-operator"); err == nil {
		t.Fatal("wrong-environment table was accepted")
	}
	if err := validateEnvironmentBinding("agentpay", "dev", "123456789012", "agentpay-dev-main", "arn:aws:iam::123456789012:role/agentpay-demo-launch-entitlement-operator"); err == nil {
		t.Fatal("wrong-environment role was accepted")
	}
}

func TestConfirmationBindsReviewedPlan(t *testing.T) {
	options := testOperationOptions(t)
	planDigest := strings.Repeat("b", 64)
	phrase := confirmationPhrase(options, planDigest)
	if !strings.Contains(phrase, options.Environment) || !strings.Contains(phrase, options.SellerID.String()) || !strings.Contains(phrase, planDigest) {
		t.Fatalf("confirmation phrase = %q", phrase)
	}
	wrongSeller := options
	wrongSeller.SellerID, _ = domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.SellerIDPrefix)
	if confirmationPhrase(wrongSeller, planDigest) == phrase {
		t.Fatal("wrong seller produced the reviewed confirmation phrase")
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
	options := testOperationOptions(t)
	if got := confirmationPhrase(options, strings.Repeat("c", 64)); got != "apply:v1:dev:leo_0123456789abcdef0123456789abcdef:grant:sel_01K5D09YJ0C0M7RJM4FWQ0K9H7:0:"+strings.Repeat("c", 64) {
		t.Fatalf("confirmationPhrase() = %q", got)
	}
}

func TestCallerUsesConfiguredLaunchEntitlementRole(t *testing.T) {
	roleARN := "arn:aws:iam::937319036732:role/agentpay-dev-launch-entitlement-operator"
	callerARN := "arn:aws:sts::937319036732:assumed-role/agentpay-dev-launch-entitlement-operator/agentpay-entitlement-session-1"
	if err := validateOperatorCaller(callerARN, roleARN); err != nil {
		t.Fatal(err)
	}
	if err := validateOperatorCaller("arn:aws:iam::937319036732:user/admin", roleARN); err == nil {
		t.Fatal("direct administrator was accepted instead of the configured operator role")
	}
	if err := validateOperatorCaller("arn:aws:iam::937319036732:root", roleARN); err == nil {
		t.Fatal("root was accepted instead of the configured operator role")
	}
	if err := validateOperatorCaller("arn:aws:sts::937319036732:assumed-role/agentpay-demo-launch-entitlement-operator/session-1", roleARN); err == nil {
		t.Fatal("wrong operator role was accepted")
	}
}

func testOperationOptions(t *testing.T) operationOptions {
	t.Helper()
	sellerID, err := domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	if err != nil {
		t.Fatal(err)
	}
	effectiveAt := time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)
	return operationOptions{
		OperationID: "leo_0123456789abcdef0123456789abcdef",
		Environment: "dev", AWSAccountID: "123456789012", AWSRegion: "ap-south-1",
		TableName: "agentpay-dev-main", OperatorRoleARN: "arn:aws:iam::123456789012:role/agentpay-dev-launch-entitlement-operator",
		Action: actionGrant, SellerID: sellerID, PlanID: billing.PlanStarter,
		AccessEndsAt: effectiveAt.Add(30 * 24 * time.Hour), EffectiveAt: effectiveAt, ExpectedVersion: 0,
	}
}
