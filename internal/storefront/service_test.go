package storefront_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/settlement"
	. "github.com/fourgeez/agentpay/internal/storefront"
	"github.com/gowebpki/jcs"
)

func TestServiceSignsOnlyFreshAuthoritativeDiscovery(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.verifyEndpoint(t)

	manifest, err := fixture.service.GetManifest(t.Context(), fixture.seller.Slug)
	if err != nil {
		t.Fatalf("GetManifest() error = %v", err)
	}
	if manifest.Document.Availability != AvailabilityActive || len(manifest.Document.Products) != 1 {
		t.Fatalf("manifest = %#v", manifest)
	}
	if manifest.Document.CanonicalOrigin != "https://store.agentpay.example" ||
		manifest.Document.ExpiresAt != fixture.now.Add(DiscoveryLifetime) ||
		manifest.Document.PublicationRevision != 1 {
		t.Fatalf("manifest metadata = %#v", manifest.Document)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), fixture.seller.UpstreamBaseURL) || strings.Contains(string(encoded), fixture.destination.Address) {
		t.Fatalf("discovery leaked private routing or payout data: %s", encoded)
	}
	verifySignedDocument(t, fixture.keys, manifest.Document, manifest.Signature)

	again, err := fixture.service.GetManifest(t.Context(), fixture.seller.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if again.Document.PublicationRevision != manifest.Document.PublicationRevision {
		t.Fatalf("unchanged revision = %d, want %d", again.Document.PublicationRevision, manifest.Document.PublicationRevision)
	}
}

func TestServiceReturnsSignedTombstoneForKnownInactiveSeller(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.entitlements.response.Assignment.Status = billing.EntitlementStatusCancelled
	fixture.entitlements.response.Assignment.NetworkAccess = billing.NetworkAccessBlocked

	tombstone, err := fixture.service.GetManifest(t.Context(), fixture.seller.Slug)
	if !errors.Is(err, ErrSellerInactive) {
		t.Fatalf("GetManifest() error = %v, want seller inactive", err)
	}
	if tombstone.Tombstone == nil || tombstone.Tombstone.Document.Reason != InactiveReasonCancelled || len(tombstone.Document.Products) != 0 {
		t.Fatalf("tombstone result = %#v", tombstone)
	}
	verifySignedDocument(t, fixture.keys, tombstone.Tombstone.Document, tombstone.Tombstone.Signature)
}

func TestAuthorizeCommerceFailsClosedForEveryFreshPrerequisite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*serviceFixture)
	}{
		{name: "endpoint not verified", mutate: func(*serviceFixture) {}},
		{name: "subscription expired", mutate: func(f *serviceFixture) {
			f.verifyEndpoint(t)
			f.entitlements.response.Assignment.AccessEndsAt = f.now
			f.entitlements.response.Assignment.NetworkAccess = billing.NetworkAccessBlocked
		}},
		{name: "route not published", mutate: func(f *serviceFixture) {
			f.verifyEndpoint(t)
			route := f.route
			expectedVersion := route.Version
			if err := route.Pause(f.now); err != nil {
				t.Fatal(err)
			}
			if err := f.catalog.UpdateRoute(t.Context(), route, expectedVersion); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "destination no longer active", mutate: func(f *serviceFixture) {
			f.verifyEndpoint(t)
			f.destinations.items = nil
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newServiceFixture(t)
			test.mutate(fixture)
			if _, err := fixture.service.AuthorizeCommerce(t.Context(), fixture.route.RouteID); !errors.Is(err, ErrCommerceUnavailable) {
				t.Fatalf("AuthorizeCommerce() error = %v", err)
			}
		})
	}
}

func TestAuthorizeCommerceReturnsVerifiedDestinationAndFrozenQuote(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.verifyEndpoint(t)

	offer, err := fixture.service.AuthorizeCommerce(t.Context(), fixture.route.RouteID)
	if err != nil {
		t.Fatalf("AuthorizeCommerce() error = %v", err)
	}
	if offer.Route.RouteID != fixture.route.RouteID || offer.Destination.DestinationID != fixture.destination.DestinationID ||
		offer.Destination.Address != fixture.route.PayTo {
		t.Fatalf("offer = %#v", offer)
	}
}

func TestServicePublishesPlatformCapabilitiesAndFreshDirectoryResults(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.verifyEndpoint(t)

	platform := fixture.service.GetPlatformManifest()
	if platform.SchemaVersion != PlatformManifestSchemaVersion ||
		platform.DirectoryEndpoint != "https://api.agentpay.example/v1/discovery/products" ||
		platform.Capabilities.Ranking {
		t.Fatalf("platform manifest = %#v", platform)
	}

	page, err := fixture.service.ListPublicProducts(t.Context(), PublicDirectoryRequest{Query: "research report", Limit: 1})
	if err != nil {
		t.Fatalf("ListPublicProducts() error = %v", err)
	}
	if page.SchemaVersion != DirectorySchemaVersion || page.AuthoritativeForPurchase || len(page.Items) != 1 {
		t.Fatalf("directory page = %#v", page)
	}
	if page.NextCursor != "" {
		t.Fatalf("single-result page must not advertise another page: %#v", page)
	}
	item := page.Items[0]
	if item.Seller.Slug != fixture.seller.Slug || item.Product.RouteID != fixture.route.RouteID ||
		item.Capabilities.Payment.Protocol != "x402" || item.Capabilities.Payment.Network != fixture.route.Network {
		t.Fatalf("directory item = %#v", item)
	}

	fixture.entitlements.response.Assignment.Status = billing.EntitlementStatusCancelled
	fixture.entitlements.response.Assignment.NetworkAccess = billing.NetworkAccessBlocked
	page, err = fixture.service.ListPublicProducts(t.Context(), PublicDirectoryRequest{Limit: 12})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("inactive seller directory page = %#v, error = %v", page, err)
	}
}

func TestServiceRejectsInvalidDirectoryQueries(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.verifyEndpoint(t)

	if _, err := fixture.service.ListPublicProducts(t.Context(), PublicDirectoryRequest{Query: "one two three four five", Limit: 12}); err == nil {
		t.Fatal("invalid directory query must fail")
	}
	if _, err := fixture.service.ListPublicProducts(t.Context(), PublicDirectoryRequest{Limit: 25}); err == nil {
		t.Fatal("directory limit above 24 must fail")
	}
}

func TestServiceBindsDirectoryCursorToAllFilters(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.verifyEndpoint(t)
	secondRouteID := mustID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9HA", domain.RouteIDPrefix)
	secondRoute, err := catalog.NewDraftPaidRoute(catalog.PaidRouteParams{
		RouteID: secondRouteID, SellerID: fixture.seller.SellerID, DisplayName: "Weather Report", ProductSlug: "weather-report",
		Method: catalog.RouteMethodPost, PathPattern: "/weather", Description: "A bounded forecast", MIMEType: "application/json",
		Amount: domain.MustParseAmount("20000000"), Asset: fixture.route.Asset, Network: fixture.route.Network,
		PayTo: fixture.route.PayTo, UpstreamTimeoutSeconds: 20, CreatedAt: fixture.now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := secondRoute.Publish(fixture.now); err != nil {
		t.Fatal(err)
	}
	if err := fixture.catalog.CreateRoute(t.Context(), secondRoute); err != nil {
		t.Fatal(err)
	}

	page, err := fixture.service.ListPublicProducts(t.Context(), PublicDirectoryRequest{Asset: fixture.route.Asset, Limit: 1})
	if err != nil || page.NextCursor == "" {
		t.Fatalf("first page = %#v, error = %v", page, err)
	}
	if _, err := fixture.service.ListPublicProducts(t.Context(), PublicDirectoryRequest{Asset: "EURC", Limit: 1, Cursor: page.NextCursor}); err == nil {
		t.Fatal("cursor reused with a different asset filter must fail")
	}
}

func TestServiceDropsStaleDirectoryProjectionAfterRouteVersionChanges(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.verifyEndpoint(t)
	projection, ok := catalog.NewPublicDirectoryProjection(fixture.route)
	if !ok {
		t.Fatal("published route projection missing")
	}
	updatedRoute := fixture.route
	if err := updatedRoute.ChangePrice(domain.MustParseAmount("36000000"), fixture.now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	fixture.service.Directory = staticDirectoryRepository{projection: projection, route: updatedRoute}

	page, err := fixture.service.ListPublicProducts(t.Context(), PublicDirectoryRequest{Limit: 12})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("stale route-version candidate page = %#v, error = %v", page, err)
	}
}

func TestServiceFreshlyRevalidatesEveryDirectoryEligibilityPrerequisite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*testing.T, *serviceFixture)
	}{
		{name: "endpoint verification", mutate: func(_ *testing.T, _ *serviceFixture) {}},
		{name: "seller status", mutate: func(t *testing.T, fixture *serviceFixture) {
			fixture.verifyEndpoint(t)
			seller, err := fixture.catalog.GetSeller(t.Context(), fixture.seller.SellerID)
			if err != nil {
				t.Fatal(err)
			}
			expectedVersion := seller.Version
			if err := seller.Suspend(fixture.now.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			if err := fixture.catalog.UpdateSeller(t.Context(), seller, expectedVersion); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "entitlement", mutate: func(t *testing.T, fixture *serviceFixture) {
			fixture.verifyEndpoint(t)
			fixture.entitlements.response.Assignment.AccessEndsAt = fixture.now
		}},
		{name: "publication readiness", mutate: func(t *testing.T, fixture *serviceFixture) {
			fixture.verifyEndpoint(t)
			fixture.service.PublicationReadiness = denyPublication{}
		}},
		{name: "payment destination", mutate: func(t *testing.T, fixture *serviceFixture) {
			fixture.verifyEndpoint(t)
			fixture.destinations.items = nil
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newServiceFixture(t)
			test.mutate(t, fixture)
			page, err := fixture.service.ListPublicProducts(t.Context(), PublicDirectoryRequest{Limit: 12})
			if err != nil || len(page.Items) != 0 {
				t.Fatalf("ineligible directory page = %#v, error = %v", page, err)
			}
		})
	}
}

type serviceFixture struct {
	now          domain.Timestamp
	seller       catalog.Seller
	route        catalog.PaidRoute
	destination  settlement.PaymentDestination
	catalog      *memory.CatalogRepository
	destinations *destinationReader
	entitlements *entitlementReader
	keys         *authorization.LocalES256KeyRing
	service      *Service
}

func newServiceFixture(t *testing.T) *serviceFixture {
	t.Helper()
	now := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	sellerID := mustID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	routeID := mustID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.RouteIDPrefix)
	destinationID := mustID(t, "dst_01K5D09YJ0C0M7RJM4FWQ0K9H9", domain.PaymentDestinationIDPrefix)
	seller, err := catalog.NewSeller(catalog.SellerParams{SellerID: sellerID, OwnerSubject: "seller-subject", Slug: "acme-research", Name: "Acme Research", UpstreamBaseURL: "https://seller.example/api", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := seller.Activate("secret/seller/acme", now); err != nil {
		t.Fatal(err)
	}
	route, err := catalog.NewDraftPaidRoute(catalog.PaidRouteParams{RouteID: routeID, SellerID: sellerID, DisplayName: "Research Report", ProductSlug: "research-report", Method: catalog.RouteMethodPost, PathPattern: "/research", Description: "A bounded report", MIMEType: "application/json", Amount: domain.MustParseAmount("35000000"), Asset: "USDC", Network: "eip155:84532", PayTo: "0x1111111111111111111111111111111111111111", UpstreamTimeoutSeconds: 20, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := route.Publish(now); err != nil {
		t.Fatal(err)
	}
	repository := memory.NewCatalogRepository()
	if err := repository.CreateSeller(context.Background(), seller); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateRoute(context.Background(), route); err != nil {
		t.Fatal(err)
	}
	verifiedAt := now
	destination := settlement.PaymentDestination{DestinationID: destinationID, SellerID: sellerID, Asset: route.Asset, Network: route.Network, Address: route.PayTo, Status: settlement.PaymentDestinationStatusActive, VerifiedAt: &verifiedAt, CreatedAt: now, UpdatedAt: now, Version: 2}
	destinations := &destinationReader{items: []settlement.PaymentDestination{destination}}
	entitlements := &entitlementReader{response: billing.SellerPlanResponse{Assignment: billing.SellerEntitlementView{SellerID: sellerID, Status: billing.EntitlementStatusActive, AccessEndsAt: now.Add(time.Hour), EntitlementEpoch: 3, NetworkAccess: billing.NetworkAccessEnabled, Version: 1}}}
	keys, err := authorization.NewLocalES256KeyRing(domain.FixedClock{Value: now.Time()})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(Dependencies{Catalog: repository, Directory: repository, Destinations: destinations, Entitlements: entitlements, PublicationReadiness: allowPublication{}, Publications: memory.NewStorefrontPublicationRepository(), Signer: keys, Clock: domain.FixedClock{Value: now.Time()}, CanonicalOrigin: "https://store.agentpay.example", APIOrigin: "https://api.agentpay.example", AuditRecorder: audit.NoopRecorder{}})
	return &serviceFixture{now: now, seller: seller, route: route, destination: destination, catalog: repository, destinations: destinations, entitlements: entitlements, keys: keys, service: service}
}

func (fixture *serviceFixture) verifyEndpoint(t *testing.T) {
	t.Helper()
	if err := fixture.service.RecordServiceEndpointVerification(t.Context(), fixture.seller.SellerID, "sandbox"); err != nil {
		t.Fatal(err)
	}
}

type destinationReader struct {
	items []settlement.PaymentDestination
}

func (reader *destinationReader) ListBySeller(context.Context, domain.ID) ([]settlement.PaymentDestination, error) {
	return append([]settlement.PaymentDestination(nil), reader.items...), nil
}

type entitlementReader struct {
	response billing.SellerPlanResponse
	err      error
}

func (reader *entitlementReader) ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error) {
	return reader.response, reader.err
}

type allowPublication struct{}

func (allowPublication) AuthorizePublication(context.Context, domain.ID) error { return nil }

type denyPublication struct{}

func (denyPublication) AuthorizePublication(context.Context, domain.ID) error {
	return ErrPublicationBlocked
}

type staticDirectoryRepository struct {
	projection catalog.PublicDirectoryProjection
	route      catalog.PaidRoute
}

func (repository staticDirectoryRepository) ListPublicDirectoryCandidates(context.Context, catalog.PublicDirectoryQuery) (catalog.PublicDirectoryCandidatePage, error) {
	return catalog.PublicDirectoryCandidatePage{
		Items: []catalog.PublicDirectoryCandidate{{Projection: repository.projection, SortKey: "PRODUCT#stale"}}, Exhausted: true,
	}, nil
}

func (repository staticDirectoryRepository) GetPublicDirectoryRoute(context.Context, domain.ID, domain.ID) (catalog.PaidRoute, error) {
	return repository.route, nil
}

func mustID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	id, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func verifySignedDocument(t *testing.T, keys *authorization.LocalES256KeyRing, document any, signature DiscoverySignature) {
	t.Helper()
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := jcs.Transform(encoded)
	if err != nil {
		t.Fatal(err)
	}
	payload := append(append([]byte(DiscoveryDomainSeparator), 0), canonical...)
	rawSignature, err := base64.RawURLEncoding.DecodeString(signature.Value)
	if err != nil || len(rawSignature) != 64 {
		t.Fatalf("signature = %q err=%v", signature.Value, err)
	}
	key, err := keys.VerificationKey(context.Background(), signature.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	if !ecdsa.Verify(key, digest[:], new(big.Int).SetBytes(rawSignature[:32]), new(big.Int).SetBytes(rawSignature[32:])) {
		t.Fatal("discovery signature did not verify")
	}
}
