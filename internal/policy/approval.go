package policy

import "github.com/fourgeez/agentpay/internal/domain"

// ApprovalThresholdPolicyVersion identifies the deterministic rule recorded in evidence.
const ApprovalThresholdPolicyVersion = "approval-threshold-v1"

// ApprovalRequirement is the result of deterministic threshold evaluation.
type ApprovalRequirement string

const (
	ApprovalNotRequired ApprovalRequirement = "not_required"
	ApprovalRequired    ApprovalRequirement = "required"
)

// ApprovalReason explains why approval is or is not required.
type ApprovalReason string

const (
	ApprovalReasonNoThreshold      ApprovalReason = "no_threshold"
	ApprovalReasonBelowThreshold   ApprovalReason = "below_threshold"
	ApprovalReasonThresholdReached ApprovalReason = "threshold_reached"
)

// ApprovalDecision is an auditable deterministic policy result.
type ApprovalDecision struct {
	Requirement   ApprovalRequirement
	Reason        ApprovalReason
	PolicyVersion string
	Amount        domain.Amount
	Threshold     *domain.Amount
}

// RequiresApproval reports whether execution must wait for buyer authorization.
func (decision ApprovalDecision) RequiresApproval() bool {
	return decision.Requirement == ApprovalRequired
}

// EvaluateApprovalThreshold applies an inclusive approval threshold. A nil
// threshold means that this policy imposes no approval requirement.
func EvaluateApprovalThreshold(amount domain.Amount, threshold *domain.Amount) (ApprovalDecision, error) {
	if amount.IsZero() {
		return ApprovalDecision{}, domain.NewValidationError("amount", "positive", "must be greater than zero")
	}

	decision := ApprovalDecision{
		Requirement:   ApprovalNotRequired,
		Reason:        ApprovalReasonNoThreshold,
		PolicyVersion: ApprovalThresholdPolicyVersion,
		Amount:        amount,
	}
	if threshold == nil {
		return decision, nil
	}

	thresholdCopy := *threshold
	decision.Threshold = &thresholdCopy
	if amount.Compare(thresholdCopy) >= 0 {
		decision.Requirement = ApprovalRequired
		decision.Reason = ApprovalReasonThresholdReached
		return decision, nil
	}

	decision.Reason = ApprovalReasonBelowThreshold
	return decision, nil
}
