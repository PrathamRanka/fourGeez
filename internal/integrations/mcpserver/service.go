package mcpserver

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// ErrResourceNotFound reports a URI outside the declared MCP resource set.
var ErrResourceNotFound = errors.New("MCP resource was not found")

const integrationDocumentation = `# AgentPay coding-agent integration

Inspect the seller, storefront, route, and transaction-summary resources before
proposing repository changes.

Keep the AgentPay integration credential in ignored server-side configuration.
Never place it in browser code, logs, prompts, generated source, or commits.

Commercial mutations use bounded, idempotent tools that require explicit seller
confirmation and operation-specific scopes.
`

// Service assembles seller-scoped MCP resource documents.
type Service struct {
	catalogReader     CatalogReader
	transactionReader TransactionReader
}

// NewService creates the read-only MCP resource service.
func NewService(
	catalogReader CatalogReader,
	transactionReader TransactionReader,
) *Service {
	return &Service{
		catalogReader:     catalogReader,
		transactionReader: transactionReader,
	}
}

// Resources returns the complete fixed resource catalog.
func (service *Service) Resources() []ResourceDescriptor {
	return []ResourceDescriptor{
		{
			URI:         SellerResourceURI,
			Name:        "seller",
			Description: "Authenticated seller profile and storefront state",
			MIMEType:    JSONMIMEType,
		},
		{
			URI:         StorefrontResourceURI,
			Name:        "storefront",
			Description: "Public storefront manifest with enabled paid routes",
			MIMEType:    JSONMIMEType,
		},
		{
			URI:         RoutesResourceURI,
			Name:        "routes",
			Description: "All configured paid routes for this seller",
			MIMEType:    JSONMIMEType,
		},
		{
			URI:         TransactionSummaryResourceURI,
			Name:        "transaction-summary",
			Description: "Redacted summary of the 100 newest seller transactions",
			MIMEType:    JSONMIMEType,
		},
		{
			URI:         IntegrationDocumentationResourceURI,
			Name:        "integration-documentation",
			Description: "AgentPay setup and credential-safety guidance",
			MIMEType:    MarkdownMIMEType,
		},
	}
}

// Read loads and serializes one resource for the credential's fixed seller.
func (service *Service) Read(
	ctx context.Context,
	principal integrations.Principal,
	uri string,
) (ResourceDocument, error) {
	switch uri {
	case SellerResourceURI:
		return service.readSeller(ctx, principal)
	case StorefrontResourceURI:
		return service.readStorefront(ctx, principal)
	case RoutesResourceURI:
		return service.readRoutes(ctx, principal)
	case TransactionSummaryResourceURI:
		return service.readTransactionSummary(ctx, principal)
	case IntegrationDocumentationResourceURI:
		return ResourceDocument{
			URI:      uri,
			MIMEType: MarkdownMIMEType,
			Text:     integrationDocumentation,
		}, nil
	default:
		return ResourceDocument{}, ErrResourceNotFound
	}
}

// readSeller removes ownership and signing-secret fields from the seller view.
func (service *Service) readSeller(
	ctx context.Context,
	principal integrations.Principal,
) (ResourceDocument, error) {
	seller, err := service.catalogReader.GetSeller(ctx, principal.SellerID)
	if err != nil {
		return ResourceDocument{}, err
	}
	resource := SellerResource{
		SellerID:        seller.SellerID,
		Name:            seller.Name,
		Slug:            seller.Slug,
		UpstreamBaseURL: seller.UpstreamBaseURL,
		Status:          seller.Status,
		CreatedAt:       seller.CreatedAt,
		UpdatedAt:       seller.UpdatedAt,
		Version:         seller.Version,
	}
	return jsonResource(SellerResourceURI, resource)
}

// readStorefront returns only routes currently visible to buyers.
func (service *Service) readStorefront(
	ctx context.Context,
	principal integrations.Principal,
) (ResourceDocument, error) {
	seller, err := service.catalogReader.GetSeller(ctx, principal.SellerID)
	if err != nil {
		return ResourceDocument{}, err
	}
	routes, err := service.catalogReader.ListRoutesBySeller(ctx, principal.SellerID)
	if err != nil {
		return ResourceDocument{}, err
	}
	enabledRoutes := make([]catalog.PaidRoute, 0, len(routes))
	for _, route := range routes {
		if route.SellerID == principal.SellerID && route.Enabled {
			enabledRoutes = append(enabledRoutes, route)
		}
	}
	manifest := catalog.StorefrontManifest{
		Seller: catalog.StorefrontSeller{
			Name: seller.Name,
			Slug: seller.Slug,
		},
		Routes: enabledRoutes,
	}
	return jsonResource(StorefrontResourceURI, manifest)
}

// readRoutes returns configured routes including unpublished entries.
func (service *Service) readRoutes(
	ctx context.Context,
	principal integrations.Principal,
) (ResourceDocument, error) {
	routes, err := service.catalogReader.ListRoutesBySeller(ctx, principal.SellerID)
	if err != nil {
		return ResourceDocument{}, err
	}
	sellerRoutes := make([]catalog.PaidRoute, 0, len(routes))
	for _, route := range routes {
		if route.SellerID == principal.SellerID {
			sellerRoutes = append(sellerRoutes, route)
		}
	}
	return jsonResource(RoutesResourceURI, RoutesResource{Items: sellerRoutes})
}

// readTransactionSummary returns bounded metadata without buyer identity.
func (service *Service) readTransactionSummary(
	ctx context.Context,
	principal integrations.Principal,
) (ResourceDocument, error) {
	recentTransactions, nextCursor, err := service.transactionReader.ListBySeller(
		ctx,
		principal.SellerID,
		maximumSummaryTransactions,
		"",
	)
	if err != nil {
		return ResourceDocument{}, err
	}

	statusCounts := make(map[transactions.TransactionStatus]int)
	recent := make([]TransactionSummaryItem, 0, len(recentTransactions))
	for _, transaction := range recentTransactions {
		if transaction.SellerID() != principal.SellerID {
			continue
		}
		statusCounts[transaction.Status()]++
		recent = append(recent, TransactionSummaryItem{
			TransactionID: transaction.TransactionID(),
			RouteID:       transaction.RouteID(),
			Status:        transaction.Status(),
			Amount:        transaction.Amount(),
			Asset:         transaction.Asset(),
			Network:       transaction.Network(),
			CreatedAt:     transaction.CreatedAt(),
			UpdatedAt:     transaction.UpdatedAt(),
		})
	}
	return jsonResource(
		TransactionSummaryResourceURI,
		TransactionSummaryResource{
			SampleSize: len(recent),
			HasMore:    nextCursor != nil,
			ByStatus:   statusCounts,
			Recent:     recent,
		},
	)
}

// jsonResource serializes one resource using the standard JSON encoder.
func jsonResource(uri string, value any) (ResourceDocument, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ResourceDocument{}, err
	}
	return ResourceDocument{
		URI:      uri,
		MIMEType: JSONMIMEType,
		Text:     string(encoded),
	}, nil
}
