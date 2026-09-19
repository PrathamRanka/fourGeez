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
	SellerID                     domain.ID         `json:"sellerId"`
	OwnerSubject                 string            `json:"ownerSubject"`
	Slug                         string            `json:"slug"`
	Name                         string            `json:"name"`
	UpstreamBaseURL              string            `json:"upstreamBaseUrl"`
	SigningSecretRef             string            `json:"signingSecretRef,omitempty"`
	VerifiedUpstreamBaseURL      string            `json:"verifiedUpstreamBaseUrl,omitempty"`
	VerifiedSigningSecretRefHash string            `json:"verifiedSigningSecretRefHash,omitempty"`
	ServiceEndpointVerifiedAt    *domain.Timestamp `json:"serviceEndpointVerifiedAt,omitempty"`
	Status                       SellerStatus      `json:"status"`
	CreatedAt                    domain.Timestamp  `json:"createdAt"`
	UpdatedAt                    domain.Timestamp  `json:"updatedAt"`
	Version                      uint64            `json:"version"`
}

// RouteMethod is an HTTP method supported by a paid route.
type RouteMethod string

const (
	RouteMethodGet  RouteMethod = "GET"
	RouteMethodPost RouteMethod = "POST"
)

// RouteLifecycleStatus identifies whether a route can accept new purchases.
type RouteLifecycleStatus string

const (
	RouteLifecycleDraft             RouteLifecycleStatus = "draft"
	RouteLifecyclePublished         RouteLifecycleStatus = "published"
	RouteLifecyclePaused            RouteLifecycleStatus = "paused"
	RouteLifecycleArchived          RouteLifecycleStatus = "archived"
	RouteLifecycleEmergencyDisabled RouteLifecycleStatus = "emergency_disabled"
)

// PaidRouteParams contains the inputs required to publish a paid API route.
type PaidRouteParams struct {
	RouteID                 domain.ID
	SellerID                domain.ID
	DisplayName             string
	ProductSlug             string
	Method                  RouteMethod
	PathPattern             string
	Description             string
	MIMEType                string
	InputSchema             JSONSchema
	OutputSchema            JSONSchema
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
	RouteID                 domain.ID            `json:"routeId"`
	SellerID                domain.ID            `json:"sellerId"`
	DisplayName             string               `json:"displayName"`
	ProductSlug             string               `json:"productSlug"`
	Method                  RouteMethod          `json:"method"`
	PathPattern             string               `json:"pathPattern"`
	Description             string               `json:"description"`
	MIMEType                string               `json:"mimeType"`
	InputSchema             JSONSchema           `json:"inputSchema"`
	OutputSchema            JSONSchema           `json:"outputSchema"`
	Amount                  domain.Amount        `json:"amount"`
	Asset                   string               `json:"asset"`
	Network                 string               `json:"network"`
	PayTo                   string               `json:"payTo"`
	ApprovalThresholdAmount *domain.Amount       `json:"approvalThresholdAmount,omitempty"`
	UpstreamTimeoutSeconds  int                  `json:"upstreamTimeoutSeconds"`
	LifecycleStatus         RouteLifecycleStatus `json:"lifecycleStatus"`
	Enabled                 bool                 `json:"enabled"`
	CreatedAt               domain.Timestamp     `json:"createdAt"`
	UpdatedAt               domain.Timestamp     `json:"updatedAt"`
	Version                 uint64               `json:"version"`
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
	DisplayName             string         `json:"displayName"`
	ProductSlug             string         `json:"productSlug"`
	Method                  RouteMethod    `json:"method"`
	PathPattern             string         `json:"pathPattern"`
	Description             string         `json:"description"`
	MIMEType                string         `json:"mimeType"`
	InputSchema             JSONSchema     `json:"inputSchema,omitempty"`
	OutputSchema            JSONSchema     `json:"outputSchema,omitempty"`
	Amount                  domain.Amount  `json:"amount"`
	Asset                   string         `json:"asset"`
	Network                 string         `json:"network"`
	PayTo                   string         `json:"payTo"`
	ApprovalThresholdAmount *domain.Amount `json:"approvalThresholdAmount"`
	UpstreamTimeoutSeconds  int            `json:"upstreamTimeoutSeconds"`
	PublishImmediately      *bool          `json:"publishImmediately,omitempty"`
}

// UpdateRoutePriceRequest is the route-price mutation HTTP request.
type UpdateRoutePriceRequest struct {
	Amount          domain.Amount `json:"amount"`
	ExpectedVersion uint64        `json:"expectedVersion"`
}

// RouteVersionRequest guards a route lifecycle mutation with optimistic concurrency.
type RouteVersionRequest struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
}

// PublishRouteRequest binds seller approval to one validated route contract.
type PublishRouteRequest struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	ContractHash    string `json:"contractHash"`
}

// PaidRouteList contains all seller-owned routes visible to the dashboard.
type PaidRouteList struct {
	Items []PaidRoute `json:"items"`
}

// ConfigureStorefrontRequest contains safe mutable seller configuration.
type ConfigureStorefrontRequest struct {
	Name            string `json:"name"`
	UpstreamBaseURL string `json:"upstreamBaseUrl"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

// RouteValidationCheck records one deterministic publication precondition.
type RouteValidationCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// RouteValidationResult contains all publication checks for one route.
type RouteValidationResult struct {
	SellerID     domain.ID              `json:"sellerId"`
	RouteID      domain.ID              `json:"routeId"`
	Valid        bool                   `json:"valid"`
	Checks       []RouteValidationCheck `json:"checks"`
	Version      uint64                 `json:"version"`
	ContractHash string                 `json:"contractHash"`
}

// StorefrontSeller is the public seller identity in discovery documents.
type StorefrontSeller struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// StorefrontManifest publishes the seller's enabled paid routes.
type StorefrontManifest struct {
	Seller StorefrontSeller `json:"seller"`
	Routes []PaidRoute      `json:"routes"`
}

// Repository is the persistence boundary consumed by catalog use cases.
type Repository interface {
	PublicDirectoryRepository
	CreateSeller(ctx context.Context, seller Seller) error
	GetSeller(ctx context.Context, sellerID domain.ID) (Seller, error)
	ResolveSellerByOwnerSubject(ctx context.Context, ownerSubject string) (Seller, error)
	ResolveSellerBySlug(ctx context.Context, slug string) (Seller, error)
	UpdateSeller(ctx context.Context, seller Seller, expectedVersion uint64) error
	CreateRoute(ctx context.Context, route PaidRoute) error
	GetRoute(ctx context.Context, routeID domain.ID) (PaidRoute, error)
	UpdateRoute(ctx context.Context, route PaidRoute, expectedVersion uint64) error
	ListRoutesBySeller(ctx context.Context, sellerID domain.ID) ([]PaidRoute, error)
}
