package catalog

import "github.com/fourgeez/agentpay/internal/domain"

// SellerStatus represents whether a storefront may serve paid routes.
type SellerStatus string

const (
	SellerStatusDraft     SellerStatus = "draft"
	SellerStatusActive    SellerStatus = "active"
	SellerStatusSuspended SellerStatus = "suspended"
)

// SellerParams contains the immutable inputs required to create a seller.
type SellerParams struct {
	SellerID        domain.ID
	OwnerSubject    string
	Slug            string
	Name            string
	UpstreamBaseURL string
	CreatedAt       domain.Timestamp
}

// Seller is an API provider that publishes paid routes through AgentPay.
type Seller struct {
	SellerID         domain.ID        `json:"sellerId"`
	OwnerSubject     string           `json:"ownerSubject"`
	Slug             string           `json:"slug"`
	Name             string           `json:"name"`
	UpstreamBaseURL  string           `json:"upstreamBaseUrl"`
	SigningSecretRef string           `json:"signingSecretRef,omitempty"`
	Status           SellerStatus     `json:"status"`
	CreatedAt        domain.Timestamp `json:"createdAt"`
	UpdatedAt        domain.Timestamp `json:"updatedAt"`
	Version          uint64           `json:"version"`
}

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
