package disputes

import (
	"strings"
	"unicode/utf8"

	"github.com/fourgeez/agentpay/internal/domain"
)

// newDispute constructs the classified dispute model.
func newDispute(params Params, status Status, code, explanation string) Dispute {
	return Dispute{
		DisputeID:          params.DisputeID,
		TransactionID:      params.TransactionID,
		Reason:             params.Reason,
		Statement:          strings.TrimSpace(params.Statement),
		Status:             status,
		RuleVersion:        RuleVersion,
		ClassificationCode: code,
		Explanation:        explanation,
		CreatedAt:          params.CreatedAt,
	}
}

// classify applies the rule associated with a dispute reason.
func classify(reason Reason, facts Facts) (Status, string, string) {
	switch reason {
	case ReasonUnauthorized:
		if facts.AuthorizationValid == nil {
			return insufficientEvidence()
		}
		if !*facts.AuthorizationValid {
			return StatusRefundRecommended, CodeAuthorizationNotValid, "Recorded authorization evidence is not valid; a refund is recommended."
		}
		return StatusDenied, CodeAuthorizationValid, "Recorded authorization evidence is valid, so the unauthorized claim is denied."
	case ReasonDuplicate:
		if facts.DuplicatePayment == nil {
			return insufficientEvidence()
		}
		if *facts.DuplicatePayment {
			return StatusRefundRecommended, CodeDuplicatePaymentConfirmed, "A duplicate payment was confirmed; a refund is recommended."
		}
		return StatusDenied, CodeDuplicatePaymentNotFound, "No duplicate payment was found, so the duplicate claim is denied."
	case ReasonWrongAmount:
		if facts.ExpectedAmount == nil || facts.PaidAmount == nil {
			return insufficientEvidence()
		}
		if facts.ExpectedAmount.Compare(*facts.PaidAmount) != 0 {
			return StatusRefundRecommended, CodeWrongAmountConfirmed, "The paid amount differs from the frozen intent amount; a refund is recommended."
		}
		return StatusDenied, CodeAmountMatchesIntent, "The paid amount matches the frozen intent amount, so the wrong-amount claim is denied."
	case ReasonNotDelivered:
		if facts.DeliverySucceeded == nil {
			return insufficientEvidence()
		}
		if !*facts.DeliverySucceeded {
			return StatusRefundRecommended, CodeDeliveryNotConfirmed, "Successful seller delivery was not recorded; a refund is recommended."
		}
		return StatusDenied, CodeDeliveryConfirmed, "Successful seller delivery was recorded, so the non-delivery claim is denied."
	case ReasonQualityOrOutput:
		return StatusSellerReview, CodeQualityReviewRequired, "Output quality requires seller review and is not judged automatically."
	default:
		panic("validated dispute reason reached classifier")
	}
}

// insufficientEvidence returns the manual-review fallback classification.
func insufficientEvidence() (Status, string, string) {
	return StatusSellerReview, CodeInsufficientEvidence, "Recorded evidence is insufficient for an automatic recommendation; seller review is required."
}

// validateParams validates a dispute creation request.
func validateParams(params Params) error {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.DisputeID.String(), domain.DisputeIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("disputeId", "format", "must be a dispute identifier"))
	}
	if _, err := domain.ParseID(params.TransactionID.String(), domain.TransactionIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("transactionId", "format", "must be a transaction identifier"))
	}
	if !validReason(params.Reason) {
		validationErrors = append(validationErrors, domain.NewValidationError("reason", "supported", "must use a documented dispute reason"))
	}
	if utf8.RuneCountInString(params.Statement) > maximumStatementLength {
		validationErrors = append(validationErrors, domain.NewValidationError("statement", "length", "must not exceed 2000 characters"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	if len(validationErrors) > 0 {
		return validationErrors
	}
	return nil
}

// validReason reports whether a reason belongs to the public dispute vocabulary.
func validReason(reason Reason) bool {
	switch reason {
	case ReasonUnauthorized, ReasonDuplicate, ReasonWrongAmount, ReasonNotDelivered, ReasonQualityOrOutput:
		return true
	default:
		return false
	}
}
