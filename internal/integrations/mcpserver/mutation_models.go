package mcpserver

import (
	"context"

	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/integrations/sandbox"
)

const MCPToolResultSchemaVersion = "agentpay.mcp-tool-result.v1"

// ConfigureStorefrontInput updates the existing credential-bound storefront.
type ConfigureStorefrontInput struct {
	IdempotencyKey    string                             `json:"idempotencyKey"`
	ConfirmationGrant string                             `json:"confirmationGrant"`
	Storefront        catalog.ConfigureStorefrontRequest `json:"storefront"`
}

// ConfigureRouteInput creates one unpublished paid-route draft.
type ConfigureRouteInput struct {
	IdempotencyKey        string             `json:"idempotencyKey"`
	ConfirmationGrant     string             `json:"confirmationGrant"`
	ExpectedSellerVersion uint64             `json:"expectedSellerVersion"`
	Route                 RouteConfiguration `json:"route"`
}

// RouteConfiguration is the MCP wire model for atomic-unit route values.
type RouteConfiguration struct {
	DisplayName             string              `json:"displayName"`
	ProductSlug             string              `json:"productSlug"`
	Method                  catalog.RouteMethod `json:"method"`
	PathPattern             string              `json:"pathPattern"`
	Description             string              `json:"description"`
	MIMEType                string              `json:"mimeType"`
	InputSchema             map[string]any      `json:"inputSchema"`
	OutputSchema            map[string]any      `json:"outputSchema"`
	Amount                  string              `json:"amount"`
	Asset                   string              `json:"asset"`
	Network                 string              `json:"network"`
	PayTo                   string              `json:"payTo"`
	ApprovalThresholdAmount *string             `json:"approvalThresholdAmount"`
	UpstreamTimeoutSeconds  int                 `json:"upstreamTimeoutSeconds"`
}

// ChangeRoutePriceInput updates future-intent pricing for one route.
type ChangeRoutePriceInput struct {
	IdempotencyKey    string                  `json:"idempotencyKey"`
	ConfirmationGrant string                  `json:"confirmationGrant"`
	RouteID           string                  `json:"routeId"`
	Price             RoutePriceConfiguration `json:"price"`
}

// RoutePriceConfiguration is the MCP wire model for a guarded price update.
type RoutePriceConfiguration struct {
	Amount          string `json:"amount"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

// ValidateRouteInput runs deterministic publication checks.
type ValidateRouteInput struct {
	RouteID string `json:"routeId"`
}

// SandboxValidateRouteInput runs the pre-publication seller probes.
type SandboxValidateRouteInput struct {
	RouteID string `json:"routeId"`
}

// PublishRouteInput conditionally publishes one validated route draft.
type PublishRouteInput struct {
	IdempotencyKey    string `json:"idempotencyKey"`
	ConfirmationGrant string `json:"confirmationGrant"`
	RouteID           string `json:"routeId"`
	ExpectedVersion   uint64 `json:"expectedVersion"`
	ContractHash      string `json:"contractHash"`
}

// MutationResult is the stable replayable output shared by MCP mutations.
type MutationResult struct {
	SchemaVersion string                         `json:"schemaVersion"`
	Operation     string                         `json:"operation"`
	Seller        *catalog.SellerResponse        `json:"seller,omitempty"`
	Route         *catalog.PaidRoute             `json:"route,omitempty"`
	Validation    *catalog.RouteValidationResult `json:"validation,omitempty"`
	Sandbox       *sandbox.Result                `json:"sandbox,omitempty"`
}

// SandboxValidator runs the complete authoritative seller validation flow.
type SandboxValidator interface {
	Validate(context.Context, domain.ID, domain.ID) (sandbox.Result, error)
}

// CatalogMutator is the bounded catalog boundary used by MCP tools.
type CatalogMutator interface {
	GetSellerForIntegration(context.Context, domain.ID) (catalog.SellerResponse, error)
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
		string,
	) (catalog.PaidRoute, error)
}

type ConfirmationConsumer interface {
	Consume(context.Context, integrations.Principal, authorization.ConfirmationConsumption) error
}
