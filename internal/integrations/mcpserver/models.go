package mcpserver

import (
	"context"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const (
	SellerResourceURI                   = "agentpay://seller"
	StorefrontResourceURI               = "agentpay://storefront"
	RoutesResourceURI                   = "agentpay://routes"
	TransactionSummaryResourceURI       = "agentpay://transactions/summary"
	IntegrationDocumentationResourceURI = "agentpay://integration/documentation"
	ClaudeCodeSetupResourceURI          = "agentpay://integration/setup/v1/claude-code"
	CodexSetupResourceURI               = "agentpay://integration/setup/v1/codex"
	GenericMCPSetupResourceURI          = "agentpay://integration/setup/v1/generic-mcp"
	ClaudeCodeSetupV2ResourceURI        = "agentpay://integration/setup/v2/claude-code"
	CodexSetupV2ResourceURI             = "agentpay://integration/setup/v2/codex"
	GenericMCPSetupV2ResourceURI        = "agentpay://integration/setup/v2/generic-mcp"
	SetupPromptName                     = "prepare_agentpay_integration"
	JSONMIMEType                        = "application/json"
	MarkdownMIMEType                    = "text/markdown"
	maximumSummaryTransactions          = 100
)

// ResourceDescriptor defines one bounded MCP resource exposed by AgentPay.
type ResourceDescriptor struct {
	URI         string
	Name        string
	Description string
	MIMEType    string
}

// ResourceDocument is the serialized content returned for one MCP resource.
type ResourceDocument struct {
	URI      string
	MIMEType string
	Text     string
}

// SellerResource is the private-field-free seller integration view.
type SellerResource struct {
	SellerID        domain.ID            `json:"sellerId"`
	Name            string               `json:"name"`
	Slug            string               `json:"slug"`
	UpstreamBaseURL string               `json:"upstreamBaseUrl"`
	Status          catalog.SellerStatus `json:"status"`
	CreatedAt       domain.Timestamp     `json:"createdAt"`
	UpdatedAt       domain.Timestamp     `json:"updatedAt"`
	Version         uint64               `json:"version"`
}

// RoutesResource contains every configured route for the authenticated seller.
type RoutesResource struct {
	Items []catalog.PaidRoute `json:"items"`
}

// TransactionSummaryResource is a bounded and redacted recent-sales view.
type TransactionSummaryResource struct {
	SampleSize int                                    `json:"sampleSize"`
	HasMore    bool                                   `json:"hasMore"`
	ByStatus   map[transactions.TransactionStatus]int `json:"byStatus"`
	Recent     []TransactionSummaryItem               `json:"recent"`
}

// TransactionSummaryItem omits buyer and payment-proof identifiers.
type TransactionSummaryItem struct {
	TransactionID domain.ID                      `json:"transactionId"`
	RouteID       domain.ID                      `json:"routeId"`
	Status        transactions.TransactionStatus `json:"status"`
	Amount        domain.Amount                  `json:"amount"`
	Asset         string                         `json:"asset"`
	Network       string                         `json:"network"`
	CreatedAt     domain.Timestamp               `json:"createdAt"`
	UpdatedAt     domain.Timestamp               `json:"updatedAt"`
}

// AccessTokenAuthorizer verifies one short-lived capability and current state.
type AccessTokenAuthorizer interface {
	AuthorizeAccessToken(context.Context, string) (integrations.Principal, error)
}

// CatalogReader reads seller-owned catalog records without mutating them.
type CatalogReader interface {
	GetSeller(context.Context, domain.ID) (catalog.Seller, error)
	ListRoutesBySeller(context.Context, domain.ID) ([]catalog.PaidRoute, error)
}

// TransactionReader reads a bounded page of seller transactions.
type TransactionReader interface {
	ListBySeller(
		context.Context,
		domain.ID,
		int,
		string,
	) ([]transactions.Transaction, *string, error)
}
