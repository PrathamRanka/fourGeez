package policy

import (
	"testing"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestEvaluateApprovalThreshold(t *testing.T) {
	t.Parallel()

	fifty := domain.MustParseAmount("50000000")
	tests := []struct {
		name            string
		amount          domain.Amount
		threshold       *domain.Amount
		wantRequirement ApprovalRequirement
		wantReason      ApprovalReason
	}{
		{
			name:            "no threshold",
			amount:          domain.MustParseAmount("35000000"),
			wantRequirement: ApprovalNotRequired,
			wantReason:      ApprovalReasonNoThreshold,
		},
		{
			name:            "below threshold",
			amount:          domain.MustParseAmount("35000000"),
			threshold:       &fifty,
			wantRequirement: ApprovalNotRequired,
			wantReason:      ApprovalReasonBelowThreshold,
		},
		{
			name:            "equal to threshold",
			amount:          domain.MustParseAmount("50000000"),
			threshold:       &fifty,
			wantRequirement: ApprovalRequired,
			wantReason:      ApprovalReasonThresholdReached,
		},
		{
			name:            "above threshold",
			amount:          domain.MustParseAmount("75000000"),
			threshold:       &fifty,
			wantRequirement: ApprovalRequired,
			wantReason:      ApprovalReasonThresholdReached,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decision, err := EvaluateApprovalThreshold(test.amount, test.threshold)
			if err != nil {
				t.Fatalf("EvaluateApprovalThreshold() error = %v", err)
			}
			if decision.Requirement != test.wantRequirement || decision.Reason != test.wantReason {
				t.Fatalf("decision = %#v, want requirement=%q reason=%q", decision, test.wantRequirement, test.wantReason)
			}
			if decision.PolicyVersion != ApprovalThresholdPolicyVersion {
				t.Fatalf("PolicyVersion = %q", decision.PolicyVersion)
			}
		})
	}
}

func TestEvaluateApprovalThresholdWithZeroThresholdRequiresApproval(t *testing.T) {
	t.Parallel()

	zero := domain.MustParseAmount("0")
	decision, err := EvaluateApprovalThreshold(domain.MustParseAmount("1"), &zero)
	if err != nil {
		t.Fatalf("EvaluateApprovalThreshold() error = %v", err)
	}
	if decision.Requirement != ApprovalRequired {
		t.Fatalf("Requirement = %q, want %q", decision.Requirement, ApprovalRequired)
	}
}

func TestEvaluateApprovalThresholdRejectsZeroPurchaseAmount(t *testing.T) {
	t.Parallel()

	_, err := EvaluateApprovalThreshold(domain.MustParseAmount("0"), nil)
	if err == nil {
		t.Fatal("EvaluateApprovalThreshold() accepted a zero purchase amount")
	}

	validationError, ok := err.(domain.ValidationError)
	if !ok || validationError.Field != "amount" {
		t.Fatalf("error = %#v, want amount validation error", err)
	}
}

func TestApprovalDecisionRequiresApproval(t *testing.T) {
	t.Parallel()

	if !(ApprovalDecision{Requirement: ApprovalRequired}).RequiresApproval() {
		t.Fatal("required decision returned false")
	}
	if (ApprovalDecision{Requirement: ApprovalNotRequired}).RequiresApproval() {
		t.Fatal("not-required decision returned true")
	}
}
