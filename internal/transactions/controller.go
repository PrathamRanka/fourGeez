package transactions

import (
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

// RequireApproval moves a proposed transaction into approval pending.
func (transaction *Transaction) RequireApproval(at domain.Timestamp) error {
	return transaction.transition(StatusApprovalPending, at)
}

// MarkApproved records successful approval.
func (transaction *Transaction) MarkApproved(at domain.Timestamp) error {
	return transaction.transition(StatusApproved, at)
}

// RequirePayment marks the transaction ready for a payment challenge.
func (transaction *Transaction) RequirePayment(at domain.Timestamp) error {
	return transaction.transition(StatusPaymentRequired, at)
}

// VerifyPayment records a verified payment identifier and proof hash.
func (transaction *Transaction) VerifyPayment(
	paymentIdentifier string,
	proofHash intents.SHA256Digest,
	at domain.Timestamp,
) error {
	if strings.TrimSpace(paymentIdentifier) == "" {
		return domain.NewValidationError("paymentIdentifier", "required", "is required")
	}
	if _, err := intents.ParseSHA256Digest(proofHash.String()); err != nil {
		return domain.NewValidationError(
			"paymentProofHash",
			"format",
			"must be a SHA-256 digest",
		)
	}
	if err := transaction.transition(StatusPaymentVerified, at); err != nil {
		return err
	}

	transaction.paymentIdentifier = strings.TrimSpace(paymentIdentifier)
	transaction.paymentProofHash = proofHash
	return nil
}

// MarkForwarded claims the transaction for one seller invocation.
func (transaction *Transaction) MarkForwarded(at domain.Timestamp) error {
	return transaction.transition(StatusForwarded, at)
}

// MarkFulfilled records successful seller delivery.
func (transaction *Transaction) MarkFulfilled(
	status int,
	responseHash intents.SHA256Digest,
	summary ResponseSummary,
	at domain.Timestamp,
) error {
	if status < 200 || status > 299 {
		return domain.NewValidationError(
			"upstreamStatus",
			"success",
			"must be a successful HTTP status",
		)
	}
	if _, err := intents.ParseSHA256Digest(responseHash.String()); err != nil {
		return domain.NewValidationError(
			"responseHash",
			"format",
			"must be a SHA-256 digest",
		)
	}
	if summary.ContentLength < 0 {
		return domain.NewValidationError(
			"responseSummary.contentLength",
			"non_negative",
			"must not be negative",
		)
	}
	if err := transaction.transition(StatusFulfilled, at); err != nil {
		return err
	}

	transaction.upstreamStatus = intPointer(status)
	responseHashCopy := responseHash
	transaction.responseHash = &responseHashCopy
	summaryCopy := summary
	transaction.responseSummary = &summaryCopy
	return nil
}

// MarkFailed records a stable delivery failure.
func (transaction *Transaction) MarkFailed(
	failureCode string,
	status *int,
	responseHash *intents.SHA256Digest,
	at domain.Timestamp,
) error {
	if strings.TrimSpace(failureCode) == "" {
		return domain.NewValidationError("failureCode", "required", "is required")
	}
	if responseHash != nil {
		if _, err := intents.ParseSHA256Digest(responseHash.String()); err != nil {
			return domain.NewValidationError(
				"responseHash",
				"format",
				"must be a SHA-256 digest",
			)
		}
	}
	if err := transaction.transition(StatusFailed, at); err != nil {
		return err
	}

	transaction.failureCode = strings.TrimSpace(failureCode)
	if status != nil {
		transaction.upstreamStatus = intPointer(*status)
	}
	if responseHash != nil {
		responseHashCopy := *responseHash
		transaction.responseHash = &responseHashCopy
	}
	return nil
}

// OpenDispute marks a completed transaction as disputed.
func (transaction *Transaction) OpenDispute(at domain.Timestamp) error {
	return transaction.transition(StatusDisputed, at)
}

// RecommendRefund records a deterministic refund recommendation.
func (transaction *Transaction) RecommendRefund(at domain.Timestamp) error {
	return transaction.transition(StatusRefundRecommended, at)
}

// Resolve closes a disputed transaction.
func (transaction *Transaction) Resolve(at domain.Timestamp) error {
	return transaction.transition(StatusResolved, at)
}

// transition applies one guarded transaction state change.
func (transaction *Transaction) transition(
	next TransactionStatus,
	at domain.Timestamp,
) error {
	if at.Before(transaction.updatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	if !allowedTransition(transaction.status, next) {
		return InvalidTransitionError{
			From: transaction.status,
			To:   next,
		}
	}

	transaction.status = next
	transaction.updatedAt = at
	transaction.version++
	return nil
}
