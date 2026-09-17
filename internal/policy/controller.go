package policy

import "github.com/fourgeez/agentpay/internal/domain"

// EvaluateApprovalThreshold applies the configured inclusive approval threshold.
func EvaluateApprovalThreshold(
	amount domain.Amount,
	threshold *domain.Amount,
) (ApprovalDecision, error) {
	return evaluateApprovalThreshold(amount, threshold)
}
