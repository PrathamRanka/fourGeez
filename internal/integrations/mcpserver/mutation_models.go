package mcpserver

import (
	"context"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
)

// Confirmation records the seller action authorizing one exact MCP change.
type Confirmation struct {
	Approved    bool   `json:"approved"`
	Summary     string `json:"summary"`
	ConfirmedAt string `json:"confirmedAt"`
}

// ConfigureStorefrontInput updates the existing credential-bound storefront.
type ConfigureStorefrontInput struct {
	IdempotencyKey string                             `json:"idempotencyKey"`
	Confirmation   Confirmation                       `json:"confirmation"`
	Storefront     catalog.ConfigureStorefrontRequest `json:"storefront"`
}

// ConfigureRouteInput creates one unpublished paid-route draft.
type ConfigureRouteInput struct {
	IdempotencyKey string             `json:"idempotencyKey"`
	Confirmation   Confirmation       `json:"confirmation"`
	Route          RouteConfiguration `json:"route"`
}

// RouteConfiguration is the MCP wire model for atomic-unit route values.
type RouteConfiguration struct {
	Method                  catalog.RouteMethod `json:"method"`
	PathPattern             string              `json:"pathPattern"`
	Description             string              `json:"description"`
	MIMEType                string              `json:"mimeType"`
	Amount                  string              `json:"amount"`
	Asset                   string              `json:"asset"`
	Network                 string              `json:"network"`
	PayTo                   string              `json:"payTo"`
	ApprovalThresholdAmount *string             `json:"approvalThresholdAmount"`
	UpstreamTimeoutSeconds  int                 `json:"upstreamTimeoutSeconds"`
}

// ChangeRoutePriceInput updates future-intent pricing for one route.
type ChangeRoutePriceInput struct {
	IdempotencyKey string                  `json:"idempotencyKey"`
	Confirmation   Confirmation            `json:"confirmation"`
	RouteID        string                  `json:"routeId"`
	Price          RoutePriceConfiguration `json:"price"`
}

// RoutePriceConfiguration is the MCP wire model for a guarded price update.
type RoutePriceConfiguration struct {
	Amount          string `json:"amount"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

// ValidateRouteInput runs deterministic publication checks.
type ValidateRouteInput struct {
	IdempotencyKey string       `json:"idempotencyKey"`
	Confirmation   Confirmation `json:"confirmation"`
	RouteID        string       `json:"routeId"`
}

// PublishRouteInput conditionally publishes one validated route draft.
type PublishRouteInput struct {
	IdempotencyKey  string       `json:"idempotencyKey"`
	Confirmation    Confirmation `json:"confirmation"`
	RouteID         string       `json:"routeId"`
	ExpectedVersion uint64       `json:"expectedVersion"`
}

// MutationResult is the stable replayable output shared by MCP mutations.
type MutationResult struct {
	Operation  string                         `json:"operation"`
	Seller     *catalog.SellerResponse        `json:"seller,omitempty"`
	Route      *catalog.PaidRoute             `json:"route,omitempty"`
	Validation *catalog.RouteValidationResult `json:"validation,omitempty"`
}

// CatalogMutator is the bounded catalog boundary used by MCP tools.
type CatalogMutator interface {
	ConfigureStorefrontForIntegration(
		context.Context,
		domain.ID,
		catalog.ConfigureStorefrontRequest,
	) (catalog.SellerResponse, error)
	CreateDraftRouteForIntegration(
		context.Context,
		domain.ID,
		catalog.CreateRouteRequest,
	) (catalog.PaidRoute, error)
	UpdateRoutePriceForIntegration(
		context.Context,
		domain.ID,
		domain.ID,
		catalog.UpdateRoutePriceRequest,
	) (catalog.PaidRoute, error)
	ValidateRouteForIntegration(
		context.Context,
		domain.ID,
		domain.ID,
	) (catalog.RouteValidationResult, error)
	PublishRouteForIntegration(
		context.Context,
		domain.ID,
		domain.ID,
		uint64,
	) (catalog.PaidRoute, error)
}
