package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const (
	testSellerID       = "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	testCredentialID   = "key_01K5D09YJ0C0M7RJM4FWQ0K9H8"
	testRouteID        = "rte_01K5D09YJ0C0M7RJM4FWQ0K9H9"
	testTransactionID  = "txn_01K5D09YJ0C0M7RJM4FWQ0K9HA"
	testIntentID       = "int_01K5D09YJ0C0M7RJM4FWQ0K9HB"
	otherSellerID      = "sel_01K5D09YJ0C0M7RJM4FWQ0K9HC"
	otherRouteID       = "rte_01K5D09YJ0C0M7RJM4FWQ0K9HD"
	otherTransactionID = "txn_01K5D09YJ0C0M7RJM4FWQ0K9HE"
	otherIntentID      = "int_01K5D09YJ0C0M7RJM4FWQ0K9HF"
)

// TestServiceReadsSellerScopedResources verifies the complete read-only catalog.
func TestServiceReadsSellerScopedResources(t *testing.T) {
	t.Parallel()

	service := newTestResourceService(t)
	principal := integrations.Principal{
		SellerID:     domain.ID(testSellerID),
		CredentialID: domain.ID(testCredentialID),
		Scopes:       []integrations.Scope{integrations.ScopeRead},
	}

	testCases := []struct {
		name     string
		uri      string
		mimeType string
		contains []string
		excludes []string
	}{
		{
			name:     "seller",
			uri:      SellerResourceURI,
			mimeType: JSONMIMEType,
			contains: []string{testSellerID, "Demo Seller", "demo-seller"},
			excludes: []string{"owner-123", "secret-reference"},
		},
		{
			name:     "storefront",
			uri:      StorefrontResourceURI,
			mimeType: JSONMIMEType,
			contains: []string{"Demo Seller", testRouteID},
		},
		{
			name:     "routes",
			uri:      RoutesResourceURI,
			mimeType: JSONMIMEType,
			contains: []string{testRouteID, "/generate"},
		},
		{
			name:     "transaction summary",
			uri:      TransactionSummaryResourceURI,
			mimeType: JSONMIMEType,
			contains: []string{testTransactionID, "PROPOSED", "100"},
			excludes: []string{"buyer-123", "paymentIdentifier", "paymentProofHash"},
		},
		{
			name:     "integration documentation",
			uri:      IntegrationDocumentationResourceURI,
			mimeType: MarkdownMIMEType,
			contains: []string{"AgentPay", "idempotent", "operation-specific scopes"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			document, err := service.Read(t.Context(), principal, testCase.uri)
			if err != nil {
				t.Fatal(err)
			}
			if document.MIMEType != testCase.mimeType {
				t.Fatalf("MIMEType = %q, want %q", document.MIMEType, testCase.mimeType)
			}
			for _, expected := range testCase.contains {
				if !strings.Contains(document.Text, expected) {
					t.Fatalf("resource %q omitted %q: %s", testCase.uri, expected, document.Text)
				}
			}
			for _, forbidden := range testCase.excludes {
				if strings.Contains(document.Text, forbidden) {
					t.Fatalf("resource %q exposed %q: %s", testCase.uri, forbidden, document.Text)
				}
			}
			if document.MIMEType == JSONMIMEType && !json.Valid([]byte(document.Text)) {
				t.Fatalf("resource %q returned invalid JSON", testCase.uri)
			}
		})
	}
}

// TestServiceRejectsUnknownResource verifies the resource boundary is closed.
func TestServiceRejectsUnknownResource(t *testing.T) {
	t.Parallel()

	service := newTestResourceService(t)
	principal := integrations.Principal{SellerID: domain.ID(testSellerID)}
	if _, err := service.Read(
		t.Context(),
		principal,
		"agentpay://other-seller",
	); err == nil {
		t.Fatal("Read() accepted an undeclared resource")
	}
}

// TestServiceFiltersMismatchedRepositoryRows protects the seller boundary.
func TestServiceFiltersMismatchedRepositoryRows(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(
		time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	)
	otherRoute := catalog.PaidRoute{
		RouteID:                domain.ID(otherRouteID),
		SellerID:               domain.ID(otherSellerID),
		Method:                 catalog.RouteMethodPost,
		PathPattern:            "/private",
		Description:            "Other seller route",
		MIMEType:               "application/json",
		Amount:                 domain.MustParseAmount("900"),
		Asset:                  "USDC",
		Network:                "eip155:84532",
		PayTo:                  "0x999",
		UpstreamTimeoutSeconds: 20,
		Enabled:                true,
		CreatedAt:              createdAt,
		UpdatedAt:              createdAt,
		Version:                1,
	}
	otherTransaction, err := transactions.NewTransaction(transactions.TransactionParams{
		TransactionID: domain.ID(otherTransactionID),
		IntentID:      domain.ID(otherIntentID),
		SellerID:      domain.ID(otherSellerID),
		RouteID:       domain.ID(otherRouteID),
		BuyerID:       "other-buyer",
		Amount:        domain.MustParseAmount("900"),
		Asset:         "USDC",
		Network:       "eip155:84532",
		CreatedAt:     createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	baseService := newTestResourceService(t)
	baseService.catalogReader.(*testCatalogReader).routes = append(
		baseService.catalogReader.(*testCatalogReader).routes,
		otherRoute,
	)
	baseService.transactionReader.(*testTransactionReader).transactions = append(
		baseService.transactionReader.(*testTransactionReader).transactions,
		otherTransaction,
	)
	principal := integrations.Principal{SellerID: domain.ID(testSellerID)}

	for _, uri := range []string{
		StorefrontResourceURI,
		RoutesResourceURI,
		TransactionSummaryResourceURI,
	} {
		document, readError := baseService.Read(t.Context(), principal, uri)
		if readError != nil {
			t.Fatal(readError)
		}
		if strings.Contains(document.Text, otherSellerID) ||
			strings.Contains(document.Text, otherRouteID) ||
			strings.Contains(document.Text, otherTransactionID) {
			t.Fatalf("resource %q exposed another seller row: %s", uri, document.Text)
		}
	}
}

// newTestResourceService creates deterministic AUT-003 resource dependencies.
func newTestResourceService(t *testing.T) *Service {
	t.Helper()

	createdAt := domain.NewTimestamp(
		time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	)
	seller := catalog.Seller{
		SellerID:         domain.ID(testSellerID),
		OwnerSubject:     "owner-123",
		Slug:             "demo-seller",
		Name:             "Demo Seller",
		UpstreamBaseURL:  "https://seller.example",
		SigningSecretRef: "secret-reference",
		Status:           catalog.SellerStatusActive,
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
		Version:          1,
	}
	route := catalog.PaidRoute{
		RouteID:                domain.ID(testRouteID),
		SellerID:               domain.ID(testSellerID),
		Method:                 catalog.RouteMethodPost,
		PathPattern:            "/generate",
		Description:            "Generate a report",
		MIMEType:               "application/json",
		Amount:                 domain.MustParseAmount("100"),
		Asset:                  "USDC",
		Network:                "eip155:84532",
		PayTo:                  "0x123",
		UpstreamTimeoutSeconds: 20,
		Enabled:                true,
		CreatedAt:              createdAt,
		UpdatedAt:              createdAt,
		Version:                1,
	}
	transaction, err := transactions.NewTransaction(transactions.TransactionParams{
		TransactionID: domain.ID(testTransactionID),
		IntentID:      domain.ID(testIntentID),
		SellerID:      domain.ID(testSellerID),
		RouteID:       domain.ID(testRouteID),
		BuyerID:       "buyer-123",
		Amount:        domain.MustParseAmount("100"),
		Asset:         "USDC",
		Network:       "eip155:84532",
		CreatedAt:     createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewService(
		&testCatalogReader{seller: seller, routes: []catalog.PaidRoute{route}},
		&testTransactionReader{transactions: []transactions.Transaction{transaction}},
	)
}

type testCatalogReader struct {
	seller catalog.Seller
	routes []catalog.PaidRoute
}

// GetSeller returns the configured seller only for its exact identifier.
func (reader *testCatalogReader) GetSeller(
	_ context.Context,
	sellerID domain.ID,
) (catalog.Seller, error) {
	if sellerID != reader.seller.SellerID {
		return catalog.Seller{}, ErrResourceNotFound
	}
	return reader.seller, nil
}

// ListRoutesBySeller returns only routes for the configured seller.
func (reader *testCatalogReader) ListRoutesBySeller(
	_ context.Context,
	sellerID domain.ID,
) ([]catalog.PaidRoute, error) {
	if sellerID != reader.seller.SellerID {
		return nil, ErrResourceNotFound
	}
	return append([]catalog.PaidRoute(nil), reader.routes...), nil
}

type testTransactionReader struct {
	transactions []transactions.Transaction
}

// ListBySeller returns one bounded seller transaction page.
func (reader *testTransactionReader) ListBySeller(
	_ context.Context,
	_ domain.ID,
	limit int,
	_ string,
) ([]transactions.Transaction, *string, error) {
	page := reader.transactions
	if len(page) > limit {
		page = page[:limit]
	}
	return append([]transactions.Transaction(nil), page...), nil, nil
}
