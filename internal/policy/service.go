package policy

import "github.com/fourgeez/agentpay/internal/domain"

// evaluateApprovalThreshold contains the deterministic threshold business rule.
func evaluateApprovalThreshold(
	amount domain.Amount,
	threshold *domain.Amount,
) (ApprovalDecision, error) {
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
