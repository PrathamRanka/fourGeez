package transactions

import (
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
)

var (
	// ErrReceiptAccess prevents receipt disclosure outside its buyer or seller.
	ErrReceiptAccess = errors.New("purchase receipt was not found")
	// ErrReceiptUnavailable reports a transaction without finalized payment.
	ErrReceiptUnavailable = errors.New("purchase receipt is not available")
	// ErrReceiptEvidenceInvalid reports evidence that cannot back a receipt.
	ErrReceiptEvidenceInvalid = errors.New("purchase receipt evidence is invalid")
)

// ReceiptTransaction contains the safe authoritative transaction fields.
type ReceiptTransaction struct {
	TransactionID        domain.ID             `json:"transactionId"`
	IntentID             domain.ID             `json:"intentId"`
	SellerID             domain.ID             `json:"sellerId"`
	RouteID              domain.ID             `json:"routeId"`
	BuyerID              string                `json:"buyerId"`
	PurchaseSessionID    string                `json:"purchaseSessionId,omitempty"`
	ProductDisplayName   string                `json:"productDisplayName,omitempty"`
	ProductSlug          string                `json:"productSlug,omitempty"`
	PurchaseChannel      PurchaseChannel       `json:"purchaseChannel,omitempty"`
	PaymentRail          PaymentRail           `json:"paymentRail,omitempty"`
	PaymentDestinationID domain.ID             `json:"paymentDestinationId,omitempty"`
	Status               TransactionStatus     `json:"status"`
	Amount               domain.Amount         `json:"amount"`
	Asset                string                `json:"asset"`
	Network              string                `json:"network"`
	PaymentFinality      PaymentFinality       `json:"paymentFinality"`
	PaymentReference     string                `json:"paymentReference"`
	ReconciledAt         *domain.Timestamp     `json:"reconciledAt"`
	UpstreamStatus       *int                  `json:"upstreamStatus"`
	ResponseHash         *intents.SHA256Digest `json:"responseHash"`
	FailureCode          string                `json:"failureCode,omitempty"`
	CreatedAt            domain.Timestamp      `json:"createdAt"`
	UpdatedAt            domain.Timestamp      `json:"updatedAt"`
}

// ReceiptEvidence contains a verified signed chain and its boundary hashes.
type ReceiptEvidence struct {
	Verified      bool                 `json:"verified"`
	EventCount    int                  `json:"eventCount"`
	RootEventHash intents.SHA256Digest `json:"rootEventHash"`
	HeadEventHash intents.SHA256Digest `json:"headEventHash"`
	Events        []evidence.Event     `json:"events"`
}

// PurchaseReceipt is the versioned downloadable machine-readable receipt.
type PurchaseReceipt struct {
	SchemaVersion string             `json:"schemaVersion"`
	Transaction   ReceiptTransaction `json:"transaction"`
	Evidence      ReceiptEvidence    `json:"evidence"`
}
