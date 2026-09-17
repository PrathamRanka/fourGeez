package transactions

import (
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

// Snapshot contains the complete persisted representation of a transaction.
type Snapshot struct {
	TransactionID     domain.ID             `json:"transactionId"`
	IntentID          domain.ID             `json:"intentId"`
	SellerID          domain.ID             `json:"sellerId"`
	RouteID           domain.ID             `json:"routeId"`
	BuyerID           string                `json:"buyerId"`
	Amount            domain.Amount         `json:"amount"`
	Asset             string                `json:"asset"`
	Network           string                `json:"network"`
	Status            TransactionStatus     `json:"status"`
	PaymentIdentifier string                `json:"paymentIdentifier,omitempty"`
	PaymentProofHash  intents.SHA256Digest  `json:"paymentProofHash,omitempty"`
	PaymentReference  string                `json:"paymentReference,omitempty"`
	PaymentFinality   PaymentFinality       `json:"paymentFinality,omitempty"`
	ReconciledAt      *domain.Timestamp     `json:"reconciledAt,omitempty"`
	UpstreamStatus    *int                  `json:"upstreamStatus,omitempty"`
	ResponseHash      *intents.SHA256Digest `json:"responseHash,omitempty"`
	ResponseSummary   *ResponseSummary      `json:"responseSummary,omitempty"`
	FailureCode       string                `json:"failureCode,omitempty"`
	CreatedAt         domain.Timestamp      `json:"createdAt"`
	UpdatedAt         domain.Timestamp      `json:"updatedAt"`
	Version           uint64                `json:"version"`
}

// Snapshot returns an isolated transaction persistence representation.
func (transaction Transaction) Snapshot() Snapshot {
	return Snapshot{
		TransactionID:     transaction.transactionID,
		IntentID:          transaction.intentID,
		SellerID:          transaction.sellerID,
		RouteID:           transaction.routeID,
		BuyerID:           transaction.buyerID,
		Amount:            transaction.amount,
		Asset:             transaction.asset,
		Network:           transaction.network,
		Status:            transaction.status,
		PaymentIdentifier: transaction.paymentIdentifier,
		PaymentProofHash:  transaction.paymentProofHash,
		PaymentReference:  transaction.paymentReference,
		PaymentFinality:   transaction.paymentFinality,
		ReconciledAt:      transaction.ReconciledAt(),
		UpstreamStatus:    transaction.UpstreamStatus(),
		ResponseHash:      transaction.ResponseHash(),
		ResponseSummary:   transaction.ResponseSummary(),
		FailureCode:       transaction.failureCode,
		CreatedAt:         transaction.createdAt,
		UpdatedAt:         transaction.updatedAt,
		Version:           transaction.version,
	}
}

// Restore validates and recreates a transaction from storage.
func Restore(snapshot Snapshot) (Transaction, error) {
	if _, err := domain.ParseID(snapshot.TransactionID.String(), domain.TransactionIDPrefix); err != nil {
		return Transaction{}, domain.NewValidationError("transactionId", "format", "stored transaction ID is invalid")
	}
	if snapshot.Version == 0 || snapshot.CreatedAt.Time().IsZero() || snapshot.UpdatedAt.Time().IsZero() || !validStoredStatus(snapshot.Status) {
		return Transaction{}, domain.NewValidationError("status", "persistence", "stored transaction metadata is invalid")
	}
	paymentFinality := snapshot.PaymentFinality
	reconciledAt := snapshot.ReconciledAt
	if paymentFinality == "" && snapshot.PaymentIdentifier != "" {
		paymentFinality = PaymentFinalityConfirmed
		if reconciledAt == nil {
			reconciledAt = &snapshot.UpdatedAt
		}
	}
	if paymentFinality != "" && !validPaymentFinality(paymentFinality) {
		return Transaction{}, domain.NewValidationError(
			"paymentFinality",
			"persistence",
			"stored payment finality is invalid",
		)
	}
	return Transaction{
		transactionID:     snapshot.TransactionID,
		intentID:          snapshot.IntentID,
		sellerID:          snapshot.SellerID,
		routeID:           snapshot.RouteID,
		buyerID:           snapshot.BuyerID,
		amount:            snapshot.Amount,
		asset:             snapshot.Asset,
		network:           snapshot.Network,
		status:            snapshot.Status,
		paymentIdentifier: snapshot.PaymentIdentifier,
		paymentProofHash:  snapshot.PaymentProofHash,
		paymentReference:  snapshot.PaymentReference,
		paymentFinality:   paymentFinality,
		reconciledAt:      reconciledAt,
		upstreamStatus:    snapshot.UpstreamStatus,
		responseHash:      snapshot.ResponseHash,
		responseSummary:   snapshot.ResponseSummary,
		failureCode:       snapshot.FailureCode,
		createdAt:         snapshot.CreatedAt,
		updatedAt:         snapshot.UpdatedAt,
		version:           snapshot.Version,
	}, nil
}

// validPaymentFinality reports whether a stored observation belongs to the contract.
func validPaymentFinality(finality PaymentFinality) bool {
	switch finality {
	case PaymentFinalityConfirmed, PaymentFinalityFinalized, PaymentFinalityFailed:
		return true
	default:
		return false
	}
}

// validStoredStatus reports whether a transaction status is part of the state machine.
func validStoredStatus(status TransactionStatus) bool {
	switch status {
	case StatusProposed, StatusApprovalPending, StatusApproved, StatusPaymentRequired, StatusPaymentVerified,
		StatusForwarded, StatusFulfilled, StatusFailed, StatusDisputed, StatusRefundRecommended, StatusResolved:
		return true
	default:
		return false
	}
}
