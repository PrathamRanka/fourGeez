package disputes

import "github.com/fourgeez/agentpay/internal/domain"

const (
	// RuleVersion identifies the deterministic dispute classifier.
	RuleVersion = "dispute-rules-v1"

	maximumStatementLength = 2000
)

// Reason identifies the buyer's dispute category.
type Reason string

const (
	ReasonUnauthorized    Reason = "unauthorized"
	ReasonDuplicate       Reason = "duplicate"
	ReasonWrongAmount     Reason = "wrong_amount"
	ReasonNotDelivered    Reason = "not_delivered"
	ReasonQualityOrOutput Reason = "quality_or_output"
)

// Status identifies the current dispute resolution state.
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

// Params contains the values needed to open a dispute.
type Params struct {
	DisputeID     domain.ID
	TransactionID domain.ID
	Reason        Reason
	Statement     string
	CreatedAt     domain.Timestamp
}

// Facts contains recorded values used by the selected dispute rule.
type Facts struct {
	AuthorizationValid *bool
	DuplicatePayment   *bool
	ExpectedAmount     *domain.Amount
	PaidAmount         *domain.Amount
	DeliverySucceeded  *bool
}

// Dispute is a deterministic classification of a transaction complaint.
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
