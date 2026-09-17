package notifications

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
)

var (
	ErrSubscriptionNotFound = errors.New("webhook subscription was not found")
	ErrSigningUnavailable   = errors.New("webhook signing is unavailable")
)

// EventType identifies one seller webhook contract.
type EventType string

const (
	EventPaymentVerified      EventType = "payment.verified"
	EventFulfillmentSucceeded EventType = "fulfillment.succeeded"
	EventFulfillmentFailed    EventType = "fulfillment.failed"
	EventDisputeChanged       EventType = "dispute.changed"
)

// SubscriptionStatus identifies whether deliveries may target a subscription.
type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusDisabled SubscriptionStatus = "disabled"
)

// SubscriptionParams contains the values required to create a subscription.
type SubscriptionParams struct {
	SubscriptionID domain.ID
	SellerID       domain.ID
	EndpointURL    string
	EventTypes     []EventType
	SecretRef      string
	CreatedAt      domain.Timestamp
}

// Subscription stores webhook configuration without secret material.
type Subscription struct {
	subscriptionID domain.ID
	sellerID       domain.ID
	endpointURL    string
	eventTypes     []EventType
	secretRef      string
	status         SubscriptionStatus
	createdAt      domain.Timestamp
	updatedAt      domain.Timestamp
	version        uint64
}

// CreateSubscriptionRequest contains seller-approved webhook configuration.
type CreateSubscriptionRequest struct {
	EndpointURL string      `json:"endpointUrl"`
	EventTypes  []EventType `json:"eventTypes"`
}

// SubscriptionView is the public subscription representation.
type SubscriptionView struct {
	SubscriptionID domain.ID          `json:"subscriptionId"`
	SellerID       domain.ID          `json:"sellerId"`
	EndpointURL    string             `json:"endpointUrl"`
	EventTypes     []EventType        `json:"eventTypes"`
	Status         SubscriptionStatus `json:"status"`
	CreatedAt      domain.Timestamp   `json:"createdAt"`
	UpdatedAt      domain.Timestamp   `json:"updatedAt"`
	Version        uint64             `json:"version"`
}

// SubscriptionCreated returns signing secret material exactly once.
type SubscriptionCreated struct {
	SubscriptionView
	SigningSecret string `json:"signingSecret"`
}

// WebhookEvent is the versioned canonical body delivered to sellers.
type WebhookEvent struct {
	SchemaVersion string           `json:"schemaVersion"`
	EventID       domain.ID        `json:"eventId"`
	SellerID      domain.ID        `json:"sellerId"`
	EventType     EventType        `json:"eventType"`
	OccurredAt    domain.Timestamp `json:"occurredAt"`
	Payload       map[string]any   `json:"payload"`
}

// SignatureHeaders contains the replay-safe webhook authentication headers.
type SignatureHeaders struct {
	EventID   string
	Timestamp string
	Signature string
}

// SignedEvent contains canonical bytes and their signature headers.
type SignedEvent struct {
	Body    []byte
	Headers SignatureHeaders
}

// Repository persists seller-scoped webhook subscriptions.
type Repository interface {
	Create(context.Context, Subscription) error
	Get(context.Context, domain.ID, domain.ID) (Subscription, error)
	ListBySeller(context.Context, domain.ID) ([]Subscription, error)
}

// SellerAuthorizer verifies that an authenticated subject owns a seller.
type SellerAuthorizer interface {
	AuthorizeSeller(context.Context, string, domain.ID) error
}

// SecretGenerator creates unpredictable webhook signing material.
type SecretGenerator interface {
	NewSecret() (string, error)
}

// SecretStore persists outbound signing secrets outside domain records.
type SecretStore interface {
	PutSecret(context.Context, string, []byte) error
}

// SecretProvider loads outbound signing secrets by opaque reference.
type SecretProvider interface {
	GetSecret(context.Context, string) ([]byte, error)
}

// SubscriptionID returns the subscription identifier.
func (subscription Subscription) SubscriptionID() domain.ID {
	return subscription.subscriptionID
}

// SellerID returns the owning seller identifier.
func (subscription Subscription) SellerID() domain.ID {
	return subscription.sellerID
}

// EndpointURL returns the configured public HTTPS destination.
func (subscription Subscription) EndpointURL() string {
	return subscription.endpointURL
}

// EventTypes returns an isolated event allowlist.
func (subscription Subscription) EventTypes() []EventType {
	return append([]EventType(nil), subscription.eventTypes...)
}

// SecretRef returns the opaque signing-secret reference.
func (subscription Subscription) SecretRef() string {
	return subscription.secretRef
}

// Status returns whether delivery is enabled.
func (subscription Subscription) Status() SubscriptionStatus {
	return subscription.status
}

// CreatedAt returns the creation timestamp.
func (subscription Subscription) CreatedAt() domain.Timestamp {
	return subscription.createdAt
}

// UpdatedAt returns the latest configuration timestamp.
func (subscription Subscription) UpdatedAt() domain.Timestamp {
	return subscription.updatedAt
}

// Version returns the optimistic concurrency version.
func (subscription Subscription) Version() uint64 {
	return subscription.version
}
