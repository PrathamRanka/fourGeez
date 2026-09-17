package evidence

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

// ErrChainInvalid reports evidence tampering or invalid ordering.
var ErrChainInvalid = errors.New("evidence chain is invalid")

// EventType identifies a stable evidence event.
type EventType string

const (
	EventIntentCreated     EventType = "intent.created"
	EventApprovalRequested EventType = "approval.requested"
	EventApprovalDecided   EventType = "approval.decided"
	EventApprovalResolved  EventType = "approval.resolved"
	EventPaymentChallenged EventType = "payment.challenged"
	EventPaymentVerified   EventType = "payment.verified"
	EventProxyForwarded    EventType = "proxy.forwarded"
	EventDeliverySucceeded EventType = "delivery.succeeded"
	EventDeliveryFailed    EventType = "delivery.failed"
	EventDisputeOpened     EventType = "dispute.opened"
	EventDisputeClassified EventType = "dispute.classified"
	EventDisputeResolved   EventType = "dispute.resolved"
)

// ActorType identifies the party responsible for an evidence event.
type ActorType string

const (
	ActorSystem   ActorType = "system"
	ActorSeller   ActorType = "seller"
	ActorBuyer    ActorType = "buyer"
	ActorApprover ActorType = "approver"
)

// Signature contains the signing key reference and encoded signature.
type Signature struct {
	KeyID string
	Value string
}

// Signer is the external signing boundary implemented by KMS in production.
type Signer interface {
	Sign(ctx context.Context, digest []byte) (Signature, error)
	Verify(ctx context.Context, keyID string, digest []byte, signature string) (bool, error)
}

// Repository persists and loads one append-only transaction evidence chain.
type Repository interface {
	Append(context.Context, Event) error
	ListByTransaction(context.Context, domain.ID) ([]Event, error)
}

// PaymentChallengeFacts are the allowlisted payment challenge fields.
type PaymentChallengeFacts struct {
	Amount  string
	Asset   string
	Network string
}

// PaymentVerificationFacts are safe results derived from a payment proof.
type PaymentVerificationFacts struct {
	PaymentIdentifier string
	PaymentProofHash  intents.SHA256Digest
}

// ProxyForwardingFacts identify the seller operation without request content.
type ProxyForwardingFacts struct {
	SellerID string
	RouteID  string
	Method   string
	Path     string
}

// DeliveryFacts contain allowlisted seller response metadata only.
type DeliveryFacts struct {
	Succeeded     bool
	StatusCode    int
	ResponseHash  intents.SHA256Digest
	ContentType   string
	ContentLength int64
	FailureCode   string
}

// EventParams contains the unsigned values for a new evidence event.
type EventParams struct {
	EventID       domain.ID
	TransactionID domain.ID
	Sequence      uint64
	EventType     EventType
	ActorType     ActorType
	ActorID       string
	Payload       map[string]any
	CreatedAt     domain.Timestamp
}

// Event is one signed, append-only link in a transaction evidence chain.
type Event struct {
	EventID           domain.ID             `json:"eventId"`
	TransactionID     domain.ID             `json:"transactionId"`
	Sequence          uint64                `json:"sequence"`
	EventType         EventType             `json:"eventType"`
	ActorType         ActorType             `json:"actorType"`
	ActorID           string                `json:"actorId,omitempty"`
	Payload           map[string]any        `json:"payload"`
	PreviousEventHash *intents.SHA256Digest `json:"previousEventHash"`
	EventHash         intents.SHA256Digest  `json:"eventHash"`
	KMSKeyID          string                `json:"kmsKeyId"`
	KMSSignature      string                `json:"kmsSignature"`
	CreatedAt         domain.Timestamp      `json:"createdAt"`
}

// hashPayload is the canonical unsigned representation of an evidence event.
type hashPayload struct {
	SchemaVersion     string         `json:"schemaVersion"`
	EventID           string         `json:"eventId"`
	TransactionID     string         `json:"transactionId"`
	Sequence          uint64         `json:"sequence"`
	EventType         EventType      `json:"eventType"`
	ActorType         ActorType      `json:"actorType"`
	ActorID           string         `json:"actorId,omitempty"`
	Payload           map[string]any `json:"payload"`
	PreviousEventHash *string        `json:"previousEventHash"`
	CreatedAt         string         `json:"createdAt"`
}
