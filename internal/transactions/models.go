package transactions

import (
	"fmt"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

// TransactionStatus identifies a transaction lifecycle state.
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

// TransactionParams contains the values required to create a transaction.
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

// ResponseSummary contains allowlisted seller response metadata.
type ResponseSummary struct {
	ContentType   string `json:"contentType,omitempty"`
	ContentLength int64  `json:"contentLength"`
}

// Transaction records payment, forwarding, delivery, and dispute state.
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

// InvalidTransitionError reports an unsupported transaction state change.
type InvalidTransitionError struct {
	From TransactionStatus
	To   TransactionStatus
}

// Error describes the rejected transaction transition.
func (transitionError InvalidTransitionError) Error() string {
	return fmt.Sprintf(
		"transaction cannot transition from %s to %s",
		transitionError.From,
		transitionError.To,
	)
}

// TransactionID returns the transaction identifier.
func (transaction Transaction) TransactionID() domain.ID {
	return transaction.transactionID
}

// IntentID returns the bound purchase-intent identifier.
func (transaction Transaction) IntentID() domain.ID {
	return transaction.intentID
}

// SellerID returns the owning seller identifier.
func (transaction Transaction) SellerID() domain.ID {
	return transaction.sellerID
}

// RouteID returns the paid-route identifier.
func (transaction Transaction) RouteID() domain.ID {
	return transaction.routeID
}

// BuyerID returns the buyer identity.
func (transaction Transaction) BuyerID() string {
	return transaction.buyerID
}

// Amount returns the exact transaction amount.
func (transaction Transaction) Amount() domain.Amount {
	return transaction.amount
}

// Asset returns the payment asset.
func (transaction Transaction) Asset() string {
	return transaction.asset
}

// Network returns the payment network.
func (transaction Transaction) Network() string {
	return transaction.network
}

// Status returns the current transaction state.
func (transaction Transaction) Status() TransactionStatus {
	return transaction.status
}

// PaymentIdentifier returns the replay-protection identifier.
func (transaction Transaction) PaymentIdentifier() string {
	return transaction.paymentIdentifier
}

// PaymentProofHash returns the non-sensitive proof digest.
func (transaction Transaction) PaymentProofHash() intents.SHA256Digest {
	return transaction.paymentProofHash
}

// UpstreamStatus returns a copy of the seller HTTP status.
func (transaction Transaction) UpstreamStatus() *int {
	if transaction.upstreamStatus == nil {
		return nil
	}
	return intPointer(*transaction.upstreamStatus)
}

// ResponseHash returns a copy of the captured response digest.
func (transaction Transaction) ResponseHash() *intents.SHA256Digest {
	if transaction.responseHash == nil {
		return nil
	}
	digest := *transaction.responseHash
	return &digest
}

// ResponseSummary returns a copy of allowlisted response metadata.
func (transaction Transaction) ResponseSummary() *ResponseSummary {
	if transaction.responseSummary == nil {
		return nil
	}
	summary := *transaction.responseSummary
	return &summary
}

// FailureCode returns the stable transaction failure code.
func (transaction Transaction) FailureCode() string {
	return transaction.failureCode
}

// CreatedAt returns the creation timestamp.
func (transaction Transaction) CreatedAt() domain.Timestamp {
	return transaction.createdAt
}

// UpdatedAt returns the latest state-change timestamp.
func (transaction Transaction) UpdatedAt() domain.Timestamp {
	return transaction.updatedAt
}

// Version returns the optimistic-concurrency version.
func (transaction Transaction) Version() uint64 {
	return transaction.version
}
