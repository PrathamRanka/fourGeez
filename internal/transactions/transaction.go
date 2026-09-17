package transactions

import (
	"fmt"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

type TransactionStatus string

const (
	StatusProposed          TransactionStatus = "PROPOSED"
	StatusApprovalPending   TransactionStatus = "APPROVAL_PENDING"
	StatusApproved          TransactionStatus = "APPROVED"
	StatusPaymentRequired   TransactionStatus = "PAYMENT_REQUIRED"
	StatusPaymentVerified   TransactionStatus = "PAYMENT_VERIFIED"
	StatusForwarded         TransactionStatus = "FORWARDED"
	StatusFulfilled         TransactionStatus = "FULFILLED"
	StatusFailed            TransactionStatus = "FAILED"
	StatusDisputed          TransactionStatus = "DISPUTED"
	StatusRefundRecommended TransactionStatus = "REFUND_RECOMMENDED"
	StatusResolved          TransactionStatus = "RESOLVED"
)

type TransactionParams struct {
	TransactionID domain.ID
	IntentID      domain.ID
	SellerID      domain.ID
	RouteID       domain.ID
	BuyerID       string
	Amount        domain.Amount
	Asset         string
	Network       string
	CreatedAt     domain.Timestamp
}

type ResponseSummary struct {
	ContentType   string `json:"contentType,omitempty"`
	ContentLength int64  `json:"contentLength"`
}

type Transaction struct {
	transactionID     domain.ID
	intentID          domain.ID
	sellerID          domain.ID
	routeID           domain.ID
	buyerID           string
	amount            domain.Amount
	asset             string
	network           string
	status            TransactionStatus
	paymentIdentifier string
	paymentProofHash  intents.SHA256Digest
	upstreamStatus    *int
	responseHash      *intents.SHA256Digest
	responseSummary   *ResponseSummary
	failureCode       string
	createdAt         domain.Timestamp
	updatedAt         domain.Timestamp
	version           uint64
}

type InvalidTransitionError struct {
	From TransactionStatus
	To   TransactionStatus
}

func (transitionError InvalidTransitionError) Error() string {
	return fmt.Sprintf("transaction cannot transition from %s to %s", transitionError.From, transitionError.To)
}

func NewTransaction(params TransactionParams) (Transaction, error) {
	validationErrors := validateTransactionParams(params)
	if len(validationErrors) > 0 {
		return Transaction{}, validationErrors
	}
	return Transaction{
		transactionID: params.TransactionID,
		intentID:      params.IntentID,
		sellerID:      params.SellerID,
		routeID:       params.RouteID,
		buyerID:       strings.TrimSpace(params.BuyerID),
		amount:        params.Amount,
		asset:         strings.TrimSpace(params.Asset),
		network:       strings.TrimSpace(params.Network),
		status:        StatusProposed,
		createdAt:     params.CreatedAt,
		updatedAt:     params.CreatedAt,
		version:       1,
	}, nil
}

func (transaction *Transaction) RequireApproval(at domain.Timestamp) error {
	return transaction.transition(StatusApprovalPending, at)
}

func (transaction *Transaction) MarkApproved(at domain.Timestamp) error {
	return transaction.transition(StatusApproved, at)
}

func (transaction *Transaction) RequirePayment(at domain.Timestamp) error {
	return transaction.transition(StatusPaymentRequired, at)
}

func (transaction *Transaction) VerifyPayment(paymentIdentifier string, proofHash intents.SHA256Digest, at domain.Timestamp) error {
	if strings.TrimSpace(paymentIdentifier) == "" {
		return domain.NewValidationError("paymentIdentifier", "required", "is required")
	}
	if _, err := intents.ParseSHA256Digest(proofHash.String()); err != nil {
		return domain.NewValidationError("paymentProofHash", "format", "must be a SHA-256 digest")
	}
	if err := transaction.transition(StatusPaymentVerified, at); err != nil {
		return err
	}
	transaction.paymentIdentifier = strings.TrimSpace(paymentIdentifier)
	transaction.paymentProofHash = proofHash
	return nil
}

func (transaction *Transaction) MarkForwarded(at domain.Timestamp) error {
	return transaction.transition(StatusForwarded, at)
}

func (transaction *Transaction) MarkFulfilled(status int, responseHash intents.SHA256Digest, summary ResponseSummary, at domain.Timestamp) error {
	if status < 200 || status > 299 {
		return domain.NewValidationError("upstreamStatus", "success", "must be a successful HTTP status")
	}
	if _, err := intents.ParseSHA256Digest(responseHash.String()); err != nil {
		return domain.NewValidationError("responseHash", "format", "must be a SHA-256 digest")
	}
	if summary.ContentLength < 0 {
		return domain.NewValidationError("responseSummary.contentLength", "non_negative", "must not be negative")
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

func (transaction *Transaction) MarkFailed(failureCode string, status *int, responseHash *intents.SHA256Digest, at domain.Timestamp) error {
	if strings.TrimSpace(failureCode) == "" {
		return domain.NewValidationError("failureCode", "required", "is required")
	}
	if responseHash != nil {
		if _, err := intents.ParseSHA256Digest(responseHash.String()); err != nil {
			return domain.NewValidationError("responseHash", "format", "must be a SHA-256 digest")
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

func (transaction *Transaction) OpenDispute(at domain.Timestamp) error {
	return transaction.transition(StatusDisputed, at)
}

func (transaction *Transaction) RecommendRefund(at domain.Timestamp) error {
	return transaction.transition(StatusRefundRecommended, at)
}

func (transaction *Transaction) Resolve(at domain.Timestamp) error {
	return transaction.transition(StatusResolved, at)
}

func (transaction *Transaction) transition(next TransactionStatus, at domain.Timestamp) error {
	if at.Before(transaction.updatedAt) {
		return domain.NewValidationError("updatedAt", "chronology", "cannot occur before the previous update")
	}
	if !allowedTransition(transaction.status, next) {
		return InvalidTransitionError{From: transaction.status, To: next}
	}
	transaction.status = next
	transaction.updatedAt = at
	transaction.version++
	return nil
}

func allowedTransition(current, next TransactionStatus) bool {
	switch current {
	case StatusProposed:
		return next == StatusApprovalPending || next == StatusPaymentRequired
	case StatusApprovalPending:
		return next == StatusApproved
	case StatusApproved:
		return next == StatusPaymentRequired
	case StatusPaymentRequired:
		return next == StatusPaymentVerified
	case StatusPaymentVerified:
		return next == StatusForwarded
	case StatusForwarded:
		return next == StatusFulfilled || next == StatusFailed
	case StatusFulfilled, StatusFailed:
		return next == StatusDisputed
	case StatusDisputed:
		return next == StatusRefundRecommended || next == StatusResolved
	case StatusRefundRecommended:
		return next == StatusResolved
	default:
		return false
	}
}

func validateTransactionParams(params TransactionParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	validateID := func(identifier domain.ID, prefix domain.IDPrefix, field string) {
		if _, err := domain.ParseID(identifier.String(), prefix); err != nil {
			validationErrors = append(validationErrors, domain.NewValidationError(field, "format", "has an invalid domain identifier"))
		}
	}
	validateID(params.TransactionID, domain.TransactionIDPrefix, "transactionId")
	validateID(params.IntentID, domain.IntentIDPrefix, "intentId")
	validateID(params.SellerID, domain.SellerIDPrefix, "sellerId")
	validateID(params.RouteID, domain.RouteIDPrefix, "routeId")
	if strings.TrimSpace(params.BuyerID) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("buyerId", "required", "is required"))
	}
	if params.Amount.IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("amount", "positive", "must be greater than zero"))
	}
	if strings.TrimSpace(params.Asset) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("asset", "required", "is required"))
	}
	if strings.TrimSpace(params.Network) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("network", "required", "is required"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	return validationErrors
}

func intPointer(value int) *int { return &value }

func (transaction Transaction) TransactionID() domain.ID  { return transaction.transactionID }
func (transaction Transaction) IntentID() domain.ID       { return transaction.intentID }
func (transaction Transaction) SellerID() domain.ID       { return transaction.sellerID }
func (transaction Transaction) RouteID() domain.ID        { return transaction.routeID }
func (transaction Transaction) BuyerID() string           { return transaction.buyerID }
func (transaction Transaction) Amount() domain.Amount     { return transaction.amount }
func (transaction Transaction) Asset() string             { return transaction.asset }
func (transaction Transaction) Network() string           { return transaction.network }
func (transaction Transaction) Status() TransactionStatus { return transaction.status }
func (transaction Transaction) PaymentIdentifier() string { return transaction.paymentIdentifier }
func (transaction Transaction) PaymentProofHash() intents.SHA256Digest {
	return transaction.paymentProofHash
}
func (transaction Transaction) UpstreamStatus() *int {
	if transaction.upstreamStatus == nil {
		return nil
	}
	return intPointer(*transaction.upstreamStatus)
}
func (transaction Transaction) ResponseHash() *intents.SHA256Digest {
	if transaction.responseHash == nil {
		return nil
	}
	digest := *transaction.responseHash
	return &digest
}
func (transaction Transaction) ResponseSummary() *ResponseSummary {
	if transaction.responseSummary == nil {
		return nil
	}
	summary := *transaction.responseSummary
	return &summary
}
func (transaction Transaction) FailureCode() string         { return transaction.failureCode }
func (transaction Transaction) CreatedAt() domain.Timestamp { return transaction.createdAt }
func (transaction Transaction) UpdatedAt() domain.Timestamp { return transaction.updatedAt }
func (transaction Transaction) Version() uint64             { return transaction.version }
