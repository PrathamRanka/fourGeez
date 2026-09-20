package publicationops

import (
	"context"
	"errors"
	"testing"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/storefront"
)

func TestStorefrontRefresherRegeneratesActiveAndInactiveSignedViews(t *testing.T) {
	t.Parallel()
	sellerID := domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	for _, test := range []struct {
		name        string
		manifestErr error
	}{
		{name: "active"},
		{name: "inactive tombstone", manifestErr: storefront.ErrSellerInactive},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest := &recordingManifestRefresher{err: test.manifestErr}
			refresher := NewStorefrontRefresher(staticSellerReader{seller: catalog.Seller{SellerID: sellerID, Slug: "demo-store"}}, manifest)
			if err := refresher.RefreshSellerPublication(t.Context(), sellerID.String()); err != nil {
				t.Fatal(err)
			}
			if manifest.slug != "demo-store" {
				t.Fatalf("manifest slug = %q", manifest.slug)
			}
		})
	}
}

func TestStorefrontRefresherFailsClosedWhenSellerCannotBeLoaded(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("database unavailable")
	refresher := NewStorefrontRefresher(staticSellerReader{err: wantErr}, &recordingManifestRefresher{})
	if err := refresher.RefreshSellerPublication(t.Context(), "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"); !errors.Is(err, wantErr) {
		t.Fatalf("RefreshSellerPublication() error = %v", err)
	}
}

type staticSellerReader struct {
	seller catalog.Seller
	err    error
}

func (reader staticSellerReader) GetSeller(context.Context, domain.ID) (catalog.Seller, error) {
	return reader.seller, reader.err
}

type recordingManifestRefresher struct {
	slug string
	err  error
}

func (refresher *recordingManifestRefresher) GetManifest(_ context.Context, slug string) (storefront.SignedStorefrontManifest, error) {
	refresher.slug = slug
	return storefront.SignedStorefrontManifest{}, refresher.err
}
