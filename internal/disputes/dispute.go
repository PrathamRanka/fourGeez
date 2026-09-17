package disputes

import (
	"strings"
	"unicode/utf8"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	RuleVersion            = "dispute-rules-v1"
	maximumStatementLength = 2000
)

type Reason string

const (
	ReasonUnauthorized    Reason = "unauthorized"
	ReasonDuplicate       Reason = "duplicate"
	ReasonWrongAmount     Reason = "wrong_amount"
	ReasonNotDelivered    Reason = "not_delivered"
	ReasonQualityOrOutput Reason = "quality_or_output"
)

type Status string

const (
	StatusOpen              Status = "open"
	StatusRefundRecommended Status = "refund_recommended"
	StatusSellerReview      Status = "seller_review"
	StatusDenied            Status = "denied"
	StatusResolved          Status = "resolved"
)

const (
	CodeAuthorizationNotValid     = "authorization_not_valid"
	CodeAuthorizationValid        = "authorization_valid"
	CodeDuplicatePaymentConfirmed = "duplicate_payment_confirmed"
	CodeDuplicatePaymentNotFound  = "duplicate_payment_not_found"
	CodeWrongAmountConfirmed      = "wrong_amount_confirmed"
	CodeAmountMatchesIntent       = "amount_matches_intent"
	CodeDeliveryNotConfirmed      = "delivery_not_confirmed"
	CodeDeliveryConfirmed         = "delivery_confirmed"
	CodeQualityReviewRequired     = "quality_review_required"
	CodeInsufficientEvidence      = "insufficient_evidence"
)

type Params struct {
	DisputeID     domain.ID
	TransactionID domain.ID
	Reason        Reason
	Statement     string
	CreatedAt     domain.Timestamp
}

// Facts contains recorded values used by the rule corresponding to Reason.
// Pointer booleans distinguish a recorded false value from missing evidence.
type Facts struct {
	AuthorizationValid *bool
	DuplicatePayment   *bool
	ExpectedAmount     *domain.Amount
	PaidAmount         *domain.Amount
	DeliverySucceeded  *bool
}

type Dispute struct {
	DisputeID          domain.ID        `json:"disputeId"`
	TransactionID      domain.ID        `json:"transactionId"`
	Reason             Reason           `json:"reason"`
	Statement          string           `json:"statement,omitempty"`
	Status             Status           `json:"status"`
	RuleVersion        string           `json:"ruleVersion"`
	ClassificationCode string           `json:"classificationCode"`
	Explanation        string           `json:"explanation"`
	CreatedAt          domain.Timestamp `json:"createdAt"`
}

func Classify(params Params, facts Facts) (Dispute, error) {
	if err := validateParams(params); err != nil {
		return Dispute{}, err
	}
	status, code, explanation := classify(params.Reason, facts)
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
	}, nil
}

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

func insufficientEvidence() (Status, string, string) {
	return StatusSellerReview, CodeInsufficientEvidence, "Recorded evidence is insufficient for an automatic recommendation; seller review is required."
}

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

func validReason(reason Reason) bool {
	switch reason {
	case ReasonUnauthorized, ReasonDuplicate, ReasonWrongAmount, ReasonNotDelivered, ReasonQualityOrOutput:
		return true
	default:
		return false
	}
}
