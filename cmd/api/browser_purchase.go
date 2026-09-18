package main

import (
	"context"

	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/storefront"
)

type browserPurchaseProductReader interface {
	GetProduct(context.Context, string, string) (storefront.SignedPublicProductDocument, error)
}

type browserPurchaseProductResolver struct {
	products browserPurchaseProductReader
}

func (resolver browserPurchaseProductResolver) ResolveProduct(ctx context.Context, sellerSlug, productSlug string) (browserpurchase.Product, error) {
	document, err := resolver.products.GetProduct(ctx, sellerSlug, productSlug)
	if err != nil {
		return browserpurchase.Product{}, err
	}
	amount, err := domain.ParseAmount(document.Document.Product.Amount)
	if err != nil {
		return browserpurchase.Product{}, err
	}
	return browserpurchase.Product{
		SellerID: document.Document.Product.SellerID, RouteID: document.Document.Product.RouteID,
		ProductSlug: document.Document.Product.ProductSlug, Amount: amount,
	}, nil
}

var _ browserpurchase.ProductResolver = browserPurchaseProductResolver{}
