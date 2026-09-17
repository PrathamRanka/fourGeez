package catalog

import (
	"mime"
	"regexp"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	maximumRouteDescriptionLength = 500
	maximumMIMETypeLength         = 120
	maximumAssetLength            = 160
	maximumNetworkLength          = 80
	maximumPayToLength            = 160
	minimumUpstreamTimeoutSeconds = 1
	maximumUpstreamTimeoutSeconds = 30
)

var (
	sellerSlugPattern = regexp.MustCompile(`^[a-z0-9-]{3,48}$`)
	routePathPattern  = regexp.MustCompile(`^/[A-Za-z0-9/_-]+$`)
)

// RouteMethod is an HTTP method supported by a paid route.
type RouteMethod string

const (
	RouteMethodGet  RouteMethod = "GET"
	RouteMethodPost RouteMethod = "POST"
)

// PaidRouteParams contains the inputs required to publish a paid API route.
type PaidRouteParams struct {
	RouteID                 domain.ID
	SellerID                domain.ID
	Method                  RouteMethod
	PathPattern             string
	Description             string
	MIMEType                string
	Amount                  domain.Amount
	Asset                   string
	Network                 string
	PayTo                   string
	ApprovalThresholdAmount *domain.Amount
	UpstreamTimeoutSeconds  int
	CreatedAt               domain.Timestamp
}

// PaidRoute is a seller-owned API operation with an exact payment requirement.
type PaidRoute struct {
	RouteID                 domain.ID        `json:"routeId"`
	SellerID                domain.ID        `json:"sellerId"`
	Method                  RouteMethod      `json:"method"`
	PathPattern             string           `json:"pathPattern"`
	Description             string           `json:"description"`
	MIMEType                string           `json:"mimeType"`
	Amount                  domain.Amount    `json:"amount"`
	Asset                   string           `json:"asset"`
	Network                 string           `json:"network"`
	PayTo                   string           `json:"payTo"`
	ApprovalThresholdAmount *domain.Amount   `json:"approvalThresholdAmount,omitempty"`
	UpstreamTimeoutSeconds  int              `json:"upstreamTimeoutSeconds"`
	Enabled                 bool             `json:"enabled"`
	CreatedAt               domain.Timestamp `json:"createdAt"`
	UpdatedAt               domain.Timestamp `json:"updatedAt"`
	Version                 uint64           `json:"version"`
}

// NewPaidRoute validates and creates an enabled route.
func NewPaidRoute(params PaidRouteParams) (PaidRoute, error) {
	validationErrors := validatePaidRouteParams(params)
	if len(validationErrors) > 0 {
		return PaidRoute{}, validationErrors
	}

	var approvalThreshold *domain.Amount
	if params.ApprovalThresholdAmount != nil {
		thresholdCopy := *params.ApprovalThresholdAmount
		approvalThreshold = &thresholdCopy
	}

	return PaidRoute{
		RouteID:                 params.RouteID,
		SellerID:                params.SellerID,
		Method:                  params.Method,
		PathPattern:             params.PathPattern,
		Description:             strings.TrimSpace(params.Description),
		MIMEType:                strings.TrimSpace(params.MIMEType),
		Amount:                  params.Amount,
		Asset:                   strings.TrimSpace(params.Asset),
		Network:                 strings.TrimSpace(params.Network),
		PayTo:                   strings.TrimSpace(params.PayTo),
		ApprovalThresholdAmount: approvalThreshold,
		UpstreamTimeoutSeconds:  params.UpstreamTimeoutSeconds,
		Enabled:                 true,
		CreatedAt:               params.CreatedAt,
		UpdatedAt:               params.CreatedAt,
		Version:                 1,
	}, nil
}

// ChangePrice updates the route price for future purchase intents. Existing
// intents retain their frozen amount.
func (paidRoute *PaidRoute) ChangePrice(amount domain.Amount, changedAt domain.Timestamp) error {
	if amount.IsZero() {
		return domain.NewValidationError("amount", "positive", "must be greater than zero")
	}
	if changedAt.Before(paidRoute.UpdatedAt) {
		return domain.NewValidationError("updatedAt", "chronology", "cannot occur before the previous update")
	}
	if paidRoute.Amount == amount {
		return nil
	}

	paidRoute.Amount = amount
	paidRoute.UpdatedAt = changedAt
	paidRoute.Version++
	return nil
}

func validatePaidRouteParams(params PaidRouteParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.RouteID.String(), domain.RouteIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("routeId", "format", "must be a route identifier"))
	}
	if _, err := domain.ParseID(params.SellerID.String(), domain.SellerIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "format", "must be a seller identifier"))
	}
	if params.Method != RouteMethodGet && params.Method != RouteMethodPost {
		validationErrors = append(validationErrors, domain.NewValidationError("method", "supported", "must be GET or POST"))
	}
	if !routePathPattern.MatchString(params.PathPattern) {
		validationErrors = append(validationErrors, domain.NewValidationError("pathPattern", "format", "must be a literal absolute route path"))
	}

	description := strings.TrimSpace(params.Description)
	if description == "" || len(description) > maximumRouteDescriptionLength {
		validationErrors = append(validationErrors, domain.NewValidationError("description", "length", "must contain 1-500 characters"))
	}

	mimeType := strings.TrimSpace(params.MIMEType)
	if mimeType == "" || len(mimeType) > maximumMIMETypeLength {
		validationErrors = append(validationErrors, domain.NewValidationError("mimeType", "length", "must contain 1-120 characters"))
	} else if _, _, err := mime.ParseMediaType(mimeType); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("mimeType", "format", "must be a valid media type"))
	}

	if params.Amount.IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("amount", "positive", "must be greater than zero"))
	}
	if value := strings.TrimSpace(params.Asset); value == "" || len(value) > maximumAssetLength {
		validationErrors = append(validationErrors, domain.NewValidationError("asset", "length", "must contain 1-160 characters"))
	}
	if value := strings.TrimSpace(params.Network); value == "" || len(value) > maximumNetworkLength {
		validationErrors = append(validationErrors, domain.NewValidationError("network", "length", "must contain 1-80 characters"))
	}
	if value := strings.TrimSpace(params.PayTo); value == "" || len(value) > maximumPayToLength {
		validationErrors = append(validationErrors, domain.NewValidationError("payTo", "length", "must contain 1-160 characters"))
	}
	if params.UpstreamTimeoutSeconds < minimumUpstreamTimeoutSeconds || params.UpstreamTimeoutSeconds > maximumUpstreamTimeoutSeconds {
		validationErrors = append(validationErrors, domain.NewValidationError("upstreamTimeoutSeconds", "range", "must be between 1 and 30 seconds"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	return validationErrors
}
