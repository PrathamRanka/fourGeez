package intents

import (
	"regexp"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	maximumBuyerIDLength = 160
	maximumAssetLength   = 160
	maximumNetworkLength = 80
	intentSchemaVersion  = "1"
)

var requestPathPattern = regexp.MustCompile(`^/[A-Za-z0-9/_-]+$`)

// RequestMethod is a paid request method frozen into a purchase intent.
type RequestMethod string

const (
	RequestMethodGet  RequestMethod = "GET"
	RequestMethodPost RequestMethod = "POST"
)

// PurchaseIntentStatus is the lifecycle state established at creation.
type PurchaseIntentStatus string

const (
	PurchaseIntentStatusReady           PurchaseIntentStatus = "ready"
	PurchaseIntentStatusApprovalPending PurchaseIntentStatus = "approval_pending"
)

// PurchaseIntentParams contains every execution-relevant field frozen at creation.
type PurchaseIntentParams struct {
	IntentID         domain.ID
	SellerID         domain.ID
	RouteID          domain.ID
	BuyerID          string
	RequestMethod    RequestMethod
	RequestPath      string
	RequestBodyHash  SHA256Digest
	Amount           domain.Amount
	Asset            string
	Network          string
	MaximumAmount    domain.Amount
	RequiresApproval bool
	CreatedAt        domain.Timestamp
	ExpiresAt        domain.Timestamp
}

// PurchaseIntent protects execution-relevant values from mutation after creation.
type PurchaseIntent struct {
	intentID         domain.ID
	sellerID         domain.ID
	routeID          domain.ID
	buyerID          string
	requestMethod    RequestMethod
	requestPath      string
	requestBodyHash  SHA256Digest
	amount           domain.Amount
	asset            string
	network          string
	maximumAmount    domain.Amount
	requiresApproval bool
	intentHash       SHA256Digest
	status           PurchaseIntentStatus
	createdAt        domain.Timestamp
	expiresAt        domain.Timestamp
}

type intentHashPayload struct {
	SchemaVersion    string        `json:"schemaVersion"`
	IntentID         string        `json:"intentId"`
	SellerID         string        `json:"sellerId"`
	RouteID          string        `json:"routeId"`
	BuyerID          string        `json:"buyerId"`
	RequestMethod    RequestMethod `json:"requestMethod"`
	RequestPath      string        `json:"requestPath"`
	RequestBodyHash  string        `json:"requestBodyHash"`
	Amount           string        `json:"amount"`
	Asset            string        `json:"asset"`
	Network          string        `json:"network"`
	MaximumAmount    string        `json:"maximumAmount"`
	RequiresApproval bool          `json:"requiresApproval"`
	CreatedAt        string        `json:"createdAt"`
	ExpiresAt        string        `json:"expiresAt"`
}

// NewPurchaseIntent validates, hashes, and freezes a purchase proposal.
func NewPurchaseIntent(params PurchaseIntentParams) (PurchaseIntent, error) {
	validationErrors := validatePurchaseIntentParams(params)
	if len(validationErrors) > 0 {
		return PurchaseIntent{}, validationErrors
	}

	status := PurchaseIntentStatusReady
	if params.RequiresApproval {
		status = PurchaseIntentStatusApprovalPending
	}

	intentHash, err := hashCanonicalValue(intentHashDomain, intentHashPayload{
		SchemaVersion:    intentSchemaVersion,
		IntentID:         params.IntentID.String(),
		SellerID:         params.SellerID.String(),
		RouteID:          params.RouteID.String(),
		BuyerID:          strings.TrimSpace(params.BuyerID),
		RequestMethod:    params.RequestMethod,
		RequestPath:      params.RequestPath,
		RequestBodyHash:  params.RequestBodyHash.String(),
		Amount:           params.Amount.String(),
		Asset:            strings.TrimSpace(params.Asset),
		Network:          strings.TrimSpace(params.Network),
		MaximumAmount:    params.MaximumAmount.String(),
		RequiresApproval: params.RequiresApproval,
		CreatedAt:        params.CreatedAt.String(),
		ExpiresAt:        params.ExpiresAt.String(),
	})
	if err != nil {
		return PurchaseIntent{}, err
	}

	return PurchaseIntent{
		intentID:         params.IntentID,
		sellerID:         params.SellerID,
		routeID:          params.RouteID,
		buyerID:          strings.TrimSpace(params.BuyerID),
		requestMethod:    params.RequestMethod,
		requestPath:      params.RequestPath,
		requestBodyHash:  params.RequestBodyHash,
		amount:           params.Amount,
		asset:            strings.TrimSpace(params.Asset),
		network:          strings.TrimSpace(params.Network),
		maximumAmount:    params.MaximumAmount,
		requiresApproval: params.RequiresApproval,
		intentHash:       intentHash,
		status:           status,
		createdAt:        params.CreatedAt,
		expiresAt:        params.ExpiresAt,
	}, nil
}

// IntentID returns the immutable intent identifier.
func (purchaseIntent PurchaseIntent) IntentID() domain.ID { return purchaseIntent.intentID }

// SellerID returns the seller whose offer was selected.
func (purchaseIntent PurchaseIntent) SellerID() domain.ID { return purchaseIntent.sellerID }

// RouteID returns the paid route whose quote was frozen.
func (purchaseIntent PurchaseIntent) RouteID() domain.ID { return purchaseIntent.routeID }

// BuyerID returns the buyer/agent identity that created the intent.
func (purchaseIntent PurchaseIntent) BuyerID() string { return purchaseIntent.buyerID }

// RequestMethod returns the frozen HTTP method.
func (purchaseIntent PurchaseIntent) RequestMethod() RequestMethod {
	return purchaseIntent.requestMethod
}

// RequestPath returns the frozen paid route path.
func (purchaseIntent PurchaseIntent) RequestPath() string { return purchaseIntent.requestPath }

// RequestBodyHash returns the canonical body digest.
func (purchaseIntent PurchaseIntent) RequestBodyHash() SHA256Digest {
	return purchaseIntent.requestBodyHash
}

// Amount returns the seller's frozen exact price.
func (purchaseIntent PurchaseIntent) Amount() domain.Amount { return purchaseIntent.amount }

// Asset returns the frozen payment asset.
func (purchaseIntent PurchaseIntent) Asset() string { return purchaseIntent.asset }

// Network returns the frozen payment network.
func (purchaseIntent PurchaseIntent) Network() string { return purchaseIntent.network }

// MaximumAmount returns the buyer-provided spending ceiling.
func (purchaseIntent PurchaseIntent) MaximumAmount() domain.Amount {
	return purchaseIntent.maximumAmount
}

// RequiresApproval reports the policy output frozen into the intent.
func (purchaseIntent PurchaseIntent) RequiresApproval() bool {
	return purchaseIntent.requiresApproval
}

// IntentHash returns the digest that approvals and payment execution bind to.
func (purchaseIntent PurchaseIntent) IntentHash() SHA256Digest { return purchaseIntent.intentHash }

// Status returns the initial intent lifecycle state.
func (purchaseIntent PurchaseIntent) Status() PurchaseIntentStatus { return purchaseIntent.status }

// CreatedAt returns the intent creation time.
func (purchaseIntent PurchaseIntent) CreatedAt() domain.Timestamp { return purchaseIntent.createdAt }

// ExpiresAt returns the time after which the intent cannot execute.
func (purchaseIntent PurchaseIntent) ExpiresAt() domain.Timestamp { return purchaseIntent.expiresAt }

func validatePurchaseIntentParams(params PurchaseIntentParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.IntentID.String(), domain.IntentIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("intentId", "format", "must be a purchase-intent identifier"))
	}
	if _, err := domain.ParseID(params.SellerID.String(), domain.SellerIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "format", "must be a seller identifier"))
	}
	if _, err := domain.ParseID(params.RouteID.String(), domain.RouteIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("routeId", "format", "must be a route identifier"))
	}

	buyerID := strings.TrimSpace(params.BuyerID)
	if buyerID == "" || len(buyerID) > maximumBuyerIDLength {
		validationErrors = append(validationErrors, domain.NewValidationError("buyerId", "length", "must contain 1-160 characters"))
	}
	if params.RequestMethod != RequestMethodGet && params.RequestMethod != RequestMethodPost {
		validationErrors = append(validationErrors, domain.NewValidationError("requestMethod", "supported", "must be GET or POST"))
	}
	if !requestPathPattern.MatchString(params.RequestPath) {
		validationErrors = append(validationErrors, domain.NewValidationError("requestPath", "format", "must be a literal absolute route path"))
	}
	if !sha256Pattern.MatchString(params.RequestBodyHash.String()) {
		validationErrors = append(validationErrors, domain.NewValidationError("requestBodyHash", "format", "must be a SHA-256 digest"))
	}
	if params.Amount.IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("amount", "positive", "must be greater than zero"))
	}
	if params.Amount.Compare(params.MaximumAmount) > 0 {
		validationErrors = append(validationErrors, domain.NewValidationError("maximumAmount", "limit", "must be greater than or equal to the seller's exact price"))
	}
	if value := strings.TrimSpace(params.Asset); value == "" || len(value) > maximumAssetLength {
		validationErrors = append(validationErrors, domain.NewValidationError("asset", "length", "must contain 1-160 characters"))
	}
	if value := strings.TrimSpace(params.Network); value == "" || len(value) > maximumNetworkLength {
		validationErrors = append(validationErrors, domain.NewValidationError("network", "length", "must contain 1-80 characters"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	if params.ExpiresAt.Time().IsZero() || !params.ExpiresAt.Time().After(params.CreatedAt.Time()) {
		validationErrors = append(validationErrors, domain.NewValidationError("expiresAt", "chronology", "must occur after creation"))
	}
	return validationErrors
}
