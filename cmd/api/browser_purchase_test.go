package main

import (
	"context"
	"testing"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/storefront"
)

func TestBrowserPurchaseProductResolverUsesAuthoritativeStorefrontProduct(t *testing.T) {
	t.Parallel()
	resolver := browserPurchaseProductResolver{products: browserPurchaseProductReaderStub{
		document: storefront.SignedPublicProductDocument{Document: storefront.PublicProductDocument{
			SellerID: "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
			Product: storefront.PublicProduct{
				SellerID: "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", RouteID: "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				ProductSlug: "market-snapshot", Amount: "2500000",
			},
		}},
	}}

	product, err := resolver.ResolveProduct(t.Context(), "demo-seller", "market-snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if product.SellerID != domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7") || product.RouteID != domain.ID("rte_01K5D09YJ0C0M7RJM4FWQ0K9H7") || product.Amount.String() != "2500000" {
		t.Fatalf("resolved product = %#v", product)
	}
}

type browserPurchaseProductReaderStub struct {
	document storefront.SignedPublicProductDocument
}

func (stub browserPurchaseProductReaderStub) GetProduct(context.Context, string, string) (storefront.SignedPublicProductDocument, error) {
	return stub.document, nil
}
