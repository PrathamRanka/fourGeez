package storefront

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/gowebpki/jcs"
)

type Service struct{ Dependencies }

func NewService(dependencies Dependencies) *Service {
	if dependencies.Clock == nil {
		dependencies.Clock = domain.SystemClock{}
	}
	if dependencies.AuditRecorder == nil {
		dependencies.AuditRecorder = audit.NoopRecorder{}
	}
	dependencies.CanonicalOrigin = strings.TrimRight(dependencies.CanonicalOrigin, "/")
	dependencies.APIOrigin = strings.TrimRight(dependencies.APIOrigin, "/")
	if dependencies.APIOrigin == "" {
		dependencies.APIOrigin = dependencies.CanonicalOrigin
	}
	return &Service{Dependencies: dependencies}
}

func (service *Service) RecordServiceEndpointVerification(ctx context.Context, sellerID domain.ID, actorID string) error {
	seller, err := service.Catalog.GetSeller(ctx, sellerID)
	if err != nil {
		return err
	}
	expectedVersion := seller.Version
	if err := seller.VerifyServiceEndpoint(domain.NewTimestamp(service.Clock.Now())); err != nil {
		return err
	}
	if err := service.Catalog.UpdateSeller(ctx, seller, expectedVersion); err != nil {
		return err
	}
	return service.AuditRecorder.Record(ctx, audit.RecordRequest{SellerID: sellerID, ActorType: audit.ActorTypeSystem, ActorID: actorID, Action: audit.ActionServiceEndpointVerified, TargetType: audit.TargetTypeSeller, TargetID: sellerID.String(), Outcome: audit.OutcomeSucceeded, ChangedFields: []string{"verifiedUpstreamBaseUrl", "verifiedSigningSecretRefHash", "serviceEndpointVerifiedAt"}})
}

func (service *Service) AuthorizePublication(ctx context.Context, sellerID, routeID domain.ID) error {
	if service.PublicationReadiness == nil {
		return ErrPublicationBlocked
	}
	if err := service.PublicationReadiness.AuthorizePublication(ctx, sellerID); err != nil {
		return ErrPublicationBlocked
	}
	_, err := service.authorizeRoute(ctx, sellerID, routeID, false)
	if err != nil {
		return ErrPublicationBlocked
	}
	return nil
}

func (service *Service) AuthorizeCommerce(ctx context.Context, routeID domain.ID) (CommerceOffer, error) {
	route, err := service.Catalog.GetRoute(ctx, routeID)
	if err != nil {
		return CommerceOffer{}, err
	}
	return service.authorizeRoute(ctx, route.SellerID, routeID, true)
}

// AuthorizeIntent returns only the fields an immutable purchase intent may freeze.
func (service *Service) AuthorizeIntent(ctx context.Context, routeID domain.ID) (catalog.PaidRoute, settlement.PaymentDestination, error) {
	offer, err := service.AuthorizeCommerce(ctx, routeID)
	if err != nil {
		return catalog.PaidRoute{}, settlement.PaymentDestination{}, err
	}
	return offer.Route, offer.Destination, nil
}

// AuthorizePaidRoute returns the current offer before an x402 challenge is created.
func (service *Service) AuthorizePaidRoute(ctx context.Context, routeID domain.ID) (catalog.Seller, catalog.PaidRoute, settlement.PaymentDestination, error) {
	offer, err := service.AuthorizeCommerce(ctx, routeID)
	if err != nil {
		return catalog.Seller{}, catalog.PaidRoute{}, settlement.PaymentDestination{}, err
	}
	return offer.Seller, offer.Route, offer.Destination, nil
}

func (service *Service) authorizeRoute(ctx context.Context, sellerID, routeID domain.ID, requirePublished bool) (CommerceOffer, error) {
	seller, err := service.Catalog.GetSeller(ctx, sellerID)
	if err != nil {
		return CommerceOffer{}, err
	}
	route, err := service.Catalog.GetRoute(ctx, routeID)
	if err != nil {
		return CommerceOffer{}, err
	}
	if route.SellerID != sellerID || seller.Status != catalog.SellerStatusActive || !seller.HasCurrentServiceEndpointVerification() {
		return CommerceOffer{}, ErrCommerceUnavailable
	}
	if requirePublished && (route.LifecycleStatus != catalog.RouteLifecyclePublished || !route.Enabled) {
		return CommerceOffer{}, ErrCommerceUnavailable
	}
	if route.LifecycleStatus == catalog.RouteLifecycleArchived || route.LifecycleStatus == catalog.RouteLifecycleEmergencyDisabled {
		return CommerceOffer{}, ErrCommerceUnavailable
	}
	if service.PublicationReadiness == nil || service.PublicationReadiness.AuthorizePublication(ctx, sellerID) != nil {
		return CommerceOffer{}, ErrCommerceUnavailable
	}
	entitlement, err := service.Entitlements.ResolveSellerPlan(ctx, sellerID)
	if err != nil || entitlement.Assignment.Status != billing.EntitlementStatusActive || entitlement.Assignment.NetworkAccess != billing.NetworkAccessEnabled || !service.Clock.Now().Before(entitlement.Assignment.AccessEndsAt.Time()) {
		return CommerceOffer{}, ErrCommerceUnavailable
	}
	destinations, err := service.Destinations.ListBySeller(ctx, sellerID)
	if err != nil {
		return CommerceOffer{}, err
	}
	for _, destination := range destinations {
		if destination.Status == settlement.PaymentDestinationStatusActive && destination.VerifiedAt != nil && destination.Asset == route.Asset && destination.Network == route.Network && destination.Address == route.PayTo {
			return CommerceOffer{Seller: seller, Route: route, Destination: destination}, nil
		}
	}
	return CommerceOffer{}, ErrCommerceUnavailable
}

func (service *Service) GetManifest(ctx context.Context, slug string) (SignedStorefrontManifest, error) {
	seller, err := service.Catalog.ResolveSellerBySlug(ctx, slug)
	if err != nil {
		return SignedStorefrontManifest{}, err
	}
	products, entitlement, active, stateErr := service.activeProducts(ctx, seller)
	if stateErr != nil {
		return SignedStorefrontManifest{}, stateErr
	}
	if !active {
		tombstone, signErr := service.signTombstone(ctx, seller, inactiveReason(entitlement))
		if signErr != nil {
			return SignedStorefrontManifest{}, signErr
		}
		return SignedStorefrontManifest{Tombstone: &tombstone}, ErrSellerInactive
	}
	revision, err := service.resolveRevision(ctx, seller, entitlement, products)
	if err != nil {
		return SignedStorefrontManifest{}, err
	}
	now := domain.NewTimestamp(service.Clock.Now())
	document := StorefrontManifest{SchemaVersion: DiscoverySchemaVersion, SellerID: seller.SellerID, Seller: PublicSeller{Name: seller.Name, Slug: seller.Slug}, Availability: AvailabilityActive, PublicationRevision: revision, IssuedAt: now, ExpiresAt: now.Add(DiscoveryLifetime), CanonicalOrigin: service.CanonicalOrigin, Products: products}
	signature, err := service.sign(ctx, document)
	return SignedStorefrontManifest{Document: document, Signature: signature}, err
}

func (service *Service) GetProduct(ctx context.Context, sellerSlug, productSlug string) (SignedPublicProductDocument, error) {
	manifest, err := service.GetManifest(ctx, sellerSlug)
	if err != nil {
		return SignedPublicProductDocument{}, err
	}
	for _, product := range manifest.Document.Products {
		if product.ProductSlug == productSlug {
			document := PublicProductDocument{SchemaVersion: DiscoverySchemaVersion, SellerID: manifest.Document.SellerID, SellerSlug: sellerSlug, PublicationRevision: manifest.Document.PublicationRevision, IssuedAt: manifest.Document.IssuedAt, ExpiresAt: manifest.Document.ExpiresAt, CanonicalOrigin: service.CanonicalOrigin, Product: product}
			signature, signErr := service.sign(ctx, document)
			return SignedPublicProductDocument{Document: document, Signature: signature}, signErr
		}
	}
	return SignedPublicProductDocument{}, persistence.ErrNotFound
}

func (service *Service) GetLLMSText(ctx context.Context, slug string) (string, error) {
	manifest, err := service.GetManifest(ctx, slug)
	if err != nil {
		return "", err
	}
	var output strings.Builder
	output.WriteString("# " + manifest.Document.Seller.Name + "\n\nCanonical manifest: " + service.CanonicalOrigin + "/store/" + slug + "/manifest.json\n\nProducts:\n")
	for _, product := range manifest.Document.Products {
		output.WriteString("- " + product.DisplayName + ": " + product.CanonicalURL + " (" + product.Amount + " " + product.Asset + " on " + product.Network + ")\n")
	}
	return output.String(), nil
}

func (service *Service) activeProducts(ctx context.Context, seller catalog.Seller) ([]PublicProduct, billing.SellerPlanResponse, bool, error) {
	entitlement, err := service.Entitlements.ResolveSellerPlan(ctx, seller.SellerID)
	if err != nil {
		if errors.Is(err, billing.ErrSellerEntitlementNotFound) || errors.Is(err, persistence.ErrNotFound) {
			return nil, entitlement, false, nil
		}
		return nil, entitlement, false, err
	}
	if entitlement.Assignment.Status != billing.EntitlementStatusActive || entitlement.Assignment.NetworkAccess != billing.NetworkAccessEnabled || !service.Clock.Now().Before(entitlement.Assignment.AccessEndsAt.Time()) || seller.Status != catalog.SellerStatusActive || !seller.HasCurrentServiceEndpointVerification() || service.PublicationReadiness == nil || service.PublicationReadiness.AuthorizePublication(ctx, seller.SellerID) != nil {
		return nil, entitlement, false, nil
	}
	routes, err := service.Catalog.ListRoutesBySeller(ctx, seller.SellerID)
	if err != nil {
		return nil, entitlement, false, err
	}
	destinations, err := service.Destinations.ListBySeller(ctx, seller.SellerID)
	if err != nil {
		return nil, entitlement, false, err
	}
	products := make([]PublicProduct, 0, len(routes))
	for _, route := range routes {
		if route.LifecycleStatus != catalog.RouteLifecyclePublished || !route.Enabled || matchingDestination(route, destinations) == nil {
			continue
		}
		products = append(products, service.publicProduct(seller, route))
	}
	sort.Slice(products, func(i, j int) bool { return products[i].ProductSlug < products[j].ProductSlug })
	return products, entitlement, len(products) > 0, nil
}

func matchingDestination(route catalog.PaidRoute, destinations []settlement.PaymentDestination) *settlement.PaymentDestination {
	for index := range destinations {
		destination := &destinations[index]
		if destination.Status == settlement.PaymentDestinationStatusActive && destination.VerifiedAt != nil && destination.Asset == route.Asset && destination.Network == route.Network && destination.Address == route.PayTo {
			return destination
		}
	}
	return nil
}

func (service *Service) publicProduct(seller catalog.Seller, route catalog.PaidRoute) PublicProduct {
	base := service.CanonicalOrigin + "/store/" + seller.Slug + "/products/" + route.ProductSlug
	return PublicProduct{SellerID: seller.SellerID, RouteID: route.RouteID, DisplayName: route.DisplayName, ProductSlug: route.ProductSlug, Description: route.Description, MIMEType: route.MIMEType, Amount: route.Amount.String(), Asset: route.Asset, Network: route.Network, Availability: AvailabilityActive, CanonicalURL: base, PurchaseSessionEndpoint: service.APIOrigin + "/v1/storefronts/" + seller.Slug + "/products/" + route.ProductSlug + "/purchase-sessions"}
}

func (service *Service) signTombstone(ctx context.Context, seller catalog.Seller, reason InactiveReason) (SignedStorefrontTombstone, error) {
	entitlement, _ := service.Entitlements.ResolveSellerPlan(ctx, seller.SellerID)
	revision, err := service.resolveRevision(ctx, seller, entitlement, nil)
	if err != nil {
		return SignedStorefrontTombstone{}, err
	}
	now := domain.NewTimestamp(service.Clock.Now())
	document := StorefrontTombstone{SchemaVersion: DiscoverySchemaVersion, SellerID: seller.SellerID, SellerSlug: seller.Slug, Availability: AvailabilityInactive, Reason: reason, PublicationRevision: revision, IssuedAt: now, ExpiresAt: now.Add(DiscoveryLifetime), CanonicalOrigin: service.CanonicalOrigin}
	signature, err := service.sign(ctx, document)
	return SignedStorefrontTombstone{Document: document, Signature: signature}, err
}

func inactiveReason(entitlement billing.SellerPlanResponse) InactiveReason {
	switch entitlement.Assignment.Status {
	case billing.EntitlementStatusCancelled:
		return InactiveReasonCancelled
	case billing.EntitlementStatusClosed:
		return InactiveReasonClosed
	default:
		return InactiveReasonSuspended
	}
}

func (service *Service) resolveRevision(ctx context.Context, seller catalog.Seller, entitlement billing.SellerPlanResponse, products []PublicProduct) (uint64, error) {
	fingerprintBytes, err := json.Marshal(struct {
		Seller      catalog.Seller                `json:"seller"`
		Entitlement billing.SellerEntitlementView `json:"entitlement"`
		Products    []PublicProduct               `json:"products"`
	}{seller, entitlement.Assignment, products})
	if err != nil {
		return 0, err
	}
	digest := sha256.Sum256(append([]byte("agentpay.publication-state.v1\x00"), fingerprintBytes...))
	fingerprint := hex.EncodeToString(digest[:])
	for attempt := 0; attempt < 3; attempt++ {
		state, getErr := service.Publications.Get(ctx, seller.SellerID)
		if errors.Is(getErr, persistence.ErrNotFound) {
			state = PublicationState{SellerID: seller.SellerID, Fingerprint: fingerprint, PublicationRevision: 1, UpdatedAt: domain.NewTimestamp(service.Clock.Now()), Version: 1}
			if putErr := service.Publications.Put(ctx, state, 0); putErr == nil {
				return 1, nil
			} else if !errors.Is(putErr, persistence.ErrConditionFailed) {
				return 0, putErr
			}
			continue
		}
		if getErr != nil {
			return 0, getErr
		}
		if state.Fingerprint == fingerprint {
			return state.PublicationRevision, nil
		}
		expected := state.Version
		state.Fingerprint = fingerprint
		state.PublicationRevision++
		state.Version++
		state.UpdatedAt = domain.NewTimestamp(service.Clock.Now())
		if putErr := service.Publications.Put(ctx, state, expected); putErr == nil {
			return state.PublicationRevision, nil
		} else if !errors.Is(putErr, persistence.ErrConditionFailed) {
			return 0, putErr
		}
	}
	return 0, persistence.ErrConditionFailed
}

func (service *Service) sign(ctx context.Context, document any) (DiscoverySignature, error) {
	encoded, err := json.Marshal(document)
	if err != nil {
		return DiscoverySignature{}, err
	}
	canonical, err := jcs.Transform(encoded)
	if err != nil {
		return DiscoverySignature{}, err
	}
	keyID, err := service.Signer.CurrentKeyID(ctx)
	if err != nil {
		return DiscoverySignature{}, err
	}
	payload := append(append([]byte(DiscoveryDomainSeparator), 0), canonical...)
	raw, err := service.Signer.Sign(ctx, keyID, payload)
	if err != nil {
		return DiscoverySignature{}, err
	}
	return DiscoverySignature{Algorithm: "ES256", KeyID: keyID, Canonicalization: "RFC8785", DomainSeparator: DiscoveryDomainSeparator, Value: base64.RawURLEncoding.EncodeToString(raw)}, nil
}
