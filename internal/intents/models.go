package intents

import (
	"context"
	"encoding/json"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/settlement"
)

// SHA256Digest is a lowercase SHA-256 digest encoded as hexadecimal.
type SHA256Digest string

// String returns the lowercase hexadecimal digest.
func (digest SHA256Digest) String() string {
	return string(digest)
}

// MarshalJSON serializes the digest as a string.
func (digest SHA256Digest) MarshalJSON() ([]byte, error) {
	return json.Marshal(digest.String())
}

// UnmarshalJSON validates a digest from a JSON string.
func (digest *SHA256Digest) UnmarshalJSON(encoded []byte) error {
	var raw string
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return domain.NewValidationError(
			"sha256",
			"format",
			"must be a JSON string containing a SHA-256 digest",
		)
	}

	parsed, err := ParseSHA256Digest(raw)
	if err != nil {
		return err
	}
	*digest = parsed
	return nil
}

// RequestMethod is a paid request method frozen into a purchase intent.
type RequestMethod string

// PurchaseChannel identifies the caller authority that owns an intent.
type PurchaseChannel string

const (
	RequestMethodGet       RequestMethod   = "GET"
	RequestMethodPost      RequestMethod   = "POST"
	PurchaseChannelAgent   PurchaseChannel = "agent"
	PurchaseChannelBrowser PurchaseChannel = "browser"
)

// PurchaseIntentStatus is the lifecycle state established at creation.
type PurchaseIntentStatus string

const (
	PurchaseIntentStatusReady           PurchaseIntentStatus = "ready"
	PurchaseIntentStatusApprovalPending PurchaseIntentStatus = "approval_pending"
)

// PurchaseIntentParams contains every execution-relevant field frozen at creation.
type PurchaseIntentParams struct {
	IntentID             domain.ID
	SellerID             domain.ID
	RouteID              domain.ID
	BuyerID              string
	PurchaseSessionID    string
	PurchaseChannel      PurchaseChannel
	ProductDisplayName   string
	ProductSlug          string
	PaymentDestinationID domain.ID
	PayTo                string
	RequestMethod        RequestMethod
	RequestPath          string
	RequestBodyHash      SHA256Digest
	Amount               domain.Amount
	Asset                string
	Network              string
	MaximumAmount        domain.Amount
	RequiresApproval     bool
	CreatedAt            domain.Timestamp
	ExpiresAt            domain.Timestamp
}

// PurchaseIntent protects execution-relevant values from mutation after creation.
type PurchaseIntent struct {
	intentID             domain.ID
	sellerID             domain.ID
	routeID              domain.ID
	buyerID              string
	purchaseSessionID    string
	purchaseChannel      PurchaseChannel
	productDisplayName   string
	productSlug          string
	paymentDestinationID domain.ID
	payTo                string
	requestMethod        RequestMethod
	requestPath          string
	requestBodyHash      SHA256Digest
	amount               domain.Amount
	asset                string
	network              string
	maximumAmount        domain.Amount
	requiresApproval     bool
	intentHash           SHA256Digest
	status               PurchaseIntentStatus
	createdAt            domain.Timestamp
	expiresAt            domain.Timestamp
}

// CreateIntentRequest is the purchase-intent HTTP request.
type CreateIntentRequest struct {
	RouteID         domain.ID     `json:"routeId"`
	RequestBodyHash SHA256Digest  `json:"requestBodyHash"`
	MaximumAmount   domain.Amount `json:"maximumAmount"`
}

// Repository is the persistence boundary consumed by intent use cases.
type Repository interface {
	Create(ctx context.Context, purchaseIntent PurchaseIntent) error
	Get(ctx context.Context, intentID domain.ID) (PurchaseIntent, error)
}

// RouteRepository resolves the authoritative seller quote for an intent.
type RouteRepository interface {
	GetRoute(ctx context.Context, routeID domain.ID) (catalog.PaidRoute, error)
}

// CommerceAuthorizer resolves a fresh published offer and verified destination.
type CommerceAuthorizer interface {
	AuthorizeIntent(context.Context, domain.ID) (catalog.PaidRoute, settlement.PaymentDestination, error)
}

// IntentID returns the immutable intent identifier.
func (purchaseIntent PurchaseIntent) IntentID() domain.ID {
	return purchaseIntent.intentID
}

// SellerID returns the seller whose offer was selected.
func (purchaseIntent PurchaseIntent) SellerID() domain.ID {
	return purchaseIntent.sellerID
}

// RouteID returns the paid route whose quote was frozen.
func (purchaseIntent PurchaseIntent) RouteID() domain.ID {
	return purchaseIntent.routeID
}

// BuyerID returns the buyer identity that created the intent.
func (purchaseIntent PurchaseIntent) BuyerID() string {
	return purchaseIntent.buyerID
}

// PurchaseSessionID returns the browser grant binding when this is a browser purchase.
func (purchaseIntent PurchaseIntent) PurchaseSessionID() string {
	return purchaseIntent.purchaseSessionID
}

// PurchaseChannel returns whether the intent belongs to an agent or browser session.
func (purchaseIntent PurchaseIntent) PurchaseChannel() PurchaseChannel {
	return purchaseIntent.purchaseChannel
}

func (purchaseIntent PurchaseIntent) ProductDisplayName() string {
	return purchaseIntent.productDisplayName
}
func (purchaseIntent PurchaseIntent) ProductSlug() string { return purchaseIntent.productSlug }
func (purchaseIntent PurchaseIntent) PaymentDestinationID() domain.ID {
	return purchaseIntent.paymentDestinationID
}
func (purchaseIntent PurchaseIntent) PayTo() string { return purchaseIntent.payTo }

// RequestMethod returns the frozen HTTP method.
func (purchaseIntent PurchaseIntent) RequestMethod() RequestMethod {
	return purchaseIntent.requestMethod
}

// RequestPath returns the frozen paid-route path.
func (purchaseIntent PurchaseIntent) RequestPath() string {
	return purchaseIntent.requestPath
}

// RequestBodyHash returns the canonical body digest.
func (purchaseIntent PurchaseIntent) RequestBodyHash() SHA256Digest {
	return purchaseIntent.requestBodyHash
}

// Amount returns the seller's frozen exact price.
func (purchaseIntent PurchaseIntent) Amount() domain.Amount {
	return purchaseIntent.amount
}

// Asset returns the frozen payment asset.
func (purchaseIntent PurchaseIntent) Asset() string {
	return purchaseIntent.asset
}

// Network returns the frozen payment network.
func (purchaseIntent PurchaseIntent) Network() string {
	return purchaseIntent.network
}

// MaximumAmount returns the buyer-provided spending ceiling.
func (purchaseIntent PurchaseIntent) MaximumAmount() domain.Amount {
	return purchaseIntent.maximumAmount
}

// RequiresApproval reports the policy output frozen into the intent.
func (purchaseIntent PurchaseIntent) RequiresApproval() bool {
	return purchaseIntent.requiresApproval
}

// IntentHash returns the digest that approvals and execution bind to.
func (purchaseIntent PurchaseIntent) IntentHash() SHA256Digest {
	return purchaseIntent.intentHash
}

// Status returns the initial intent lifecycle state.
func (purchaseIntent PurchaseIntent) Status() PurchaseIntentStatus {
	return purchaseIntent.status
}

// CreatedAt returns the intent creation time.
func (purchaseIntent PurchaseIntent) CreatedAt() domain.Timestamp {
	return purchaseIntent.createdAt
}

// ExpiresAt returns the time after which the intent cannot execute.
func (purchaseIntent PurchaseIntent) ExpiresAt() domain.Timestamp {
	return purchaseIntent.expiresAt
}

// intentHashPayload is the versioned canonical intent hashing shape.
type intentHashPayload struct {
	SchemaVersion        string          `json:"schemaVersion"`
	IntentID             string          `json:"intentId"`
	SellerID             string          `json:"sellerId"`
	RouteID              string          `json:"routeId"`
	BuyerID              string          `json:"buyerId"`
	PurchaseSessionID    string          `json:"purchaseSessionId,omitempty"`
	PurchaseChannel      PurchaseChannel `json:"purchaseChannel"`
	ProductDisplayName   string          `json:"productDisplayName"`
	ProductSlug          string          `json:"productSlug"`
	PaymentDestinationID string          `json:"paymentDestinationId"`
	PayTo                string          `json:"payTo"`
	RequestMethod        RequestMethod   `json:"requestMethod"`
	RequestPath          string          `json:"requestPath"`
	RequestBodyHash      string          `json:"requestBodyHash"`
	Amount               string          `json:"amount"`
	Asset                string          `json:"asset"`
	Network              string          `json:"network"`
	MaximumAmount        string          `json:"maximumAmount"`
	RequiresApproval     bool            `json:"requiresApproval"`
	CreatedAt            string          `json:"createdAt"`
	ExpiresAt            string          `json:"expiresAt"`
}
