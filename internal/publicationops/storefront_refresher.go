package publicationops

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/storefront"
)

type SellerReader interface {
	GetSeller(context.Context, domain.ID) (catalog.Seller, error)
}

type ManifestRefresher interface {
	GetManifest(context.Context, string) (storefront.SignedStorefrontManifest, error)
}

type StorefrontRefresher struct {
	sellers   SellerReader
	manifests ManifestRefresher
}

func NewStorefrontRefresher(sellers SellerReader, manifests ManifestRefresher) *StorefrontRefresher {
	return &StorefrontRefresher{sellers: sellers, manifests: manifests}
}

func (refresher *StorefrontRefresher) RefreshSellerPublication(ctx context.Context, rawSellerID string) error {
	sellerID, err := domain.ParseID(rawSellerID, domain.SellerIDPrefix)
	if err != nil {
		return ErrInvalidEvent
	}
	if refresher.sellers == nil || refresher.manifests == nil {
		return errors.New("storefront refresh dependencies are unavailable")
	}
	seller, err := refresher.sellers.GetSeller(ctx, sellerID)
	if err != nil {
		return err
	}
	_, err = refresher.manifests.GetManifest(ctx, seller.Slug)
	if errors.Is(err, storefront.ErrSellerInactive) {
		return nil
	}
	return err
}
