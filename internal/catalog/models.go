package catalog

import (
	"context"

	"github.com/fourgeez/agentpay/internal/domain"
)

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

// SellerResponse is the public seller representation without private ownership fields.
type SellerResponse struct {
	SellerID         domain.ID        `json:"sellerId"`
	Name             string           `json:"name"`
	Slug             string           `json:"slug"`
	UpstreamBaseURL  string           `json:"upstreamBaseUrl"`
	Status           SellerStatus     `json:"status"`
	CreatedAt        domain.Timestamp `json:"createdAt"`
	UpdatedAt        domain.Timestamp `json:"updatedAt"`
	Version          uint64           `json:"version"`
	OwnerSubject     string           `json:"-"`
	SigningSecretRef string           `json:"-"`
}

// CreateSellerRequest is the seller-onboarding HTTP request.
type CreateSellerRequest struct {
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	UpstreamBaseURL string `json:"upstreamBaseUrl"`
}

// CreateRouteRequest is the paid-route creation HTTP request.
type CreateRouteRequest struct {
	Method                  RouteMethod    `json:"method"`
	PathPattern             string         `json:"pathPattern"`
	Description             string         `json:"description"`
	MIMEType                string         `json:"mimeType"`
	Amount                  domain.Amount  `json:"amount"`
	Asset                   string         `json:"asset"`
	Network                 string         `json:"network"`
	PayTo                   string         `json:"payTo"`
	ApprovalThresholdAmount *domain.Amount `json:"approvalThresholdAmount"`
	UpstreamTimeoutSeconds  int            `json:"upstreamTimeoutSeconds"`
}

// UpdateRoutePriceRequest is the route-price mutation HTTP request.
type UpdateRoutePriceRequest struct {
	Amount          domain.Amount `json:"amount"`
	ExpectedVersion uint64        `json:"expectedVersion"`
}

// Repository is the persistence boundary consumed by catalog use cases.
type Repository interface {
	CreateSeller(ctx context.Context, seller Seller) error
	GetSeller(ctx context.Context, sellerID domain.ID) (Seller, error)
	CreateRoute(ctx context.Context, route PaidRoute) error
	GetRoute(ctx context.Context, routeID domain.ID) (PaidRoute, error)
	UpdateRoute(ctx context.Context, route PaidRoute, expectedVersion uint64) error
	ListRoutesBySeller(ctx context.Context, sellerID domain.ID) ([]PaidRoute, error)
}
