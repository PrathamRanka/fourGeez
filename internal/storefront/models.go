package storefront

import (
	"context"
	"errors"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/settlement"
)

const (
	DiscoverySchemaVersion         = "agentpay.discovery.v1"
	DiscoveryDomainSeparator       = "agentpay.discovery.v1"
	ProductContractSchemaVersion   = "agentpay.product-contract.v1"
	ProductContractDomainSeparator = "agentpay.product-contract.v1"
	DiscoveryLifetime              = 5 * time.Minute
	PlatformManifestSchemaVersion  = "agentpay.platform.v1"
	DirectorySchemaVersion         = "agentpay.directory.v1"
	PlatformStatusDevelopment      = "development_preview"
	DirectoryOrderingLexical       = "lexical"
	BuyerChannelAgent              = "agent"
	BuyerChannelBrowser            = "browser"
	PaymentProtocolX402            = "x402"
	PaymentSchemeExact             = "exact"
	PaymentEnvironmentTestnet      = "testnet"
	SupportedX402Network           = "eip155:84532"
	FulfillmentModeSynchronous     = "synchronous_https"
	AvailabilityActive             = "active"
	AvailabilityInactive           = "inactive"
)

var (
	ErrSellerInactive      = errors.New("seller is inactive")
	ErrCommerceUnavailable = domain.ErrCommerceUnavailable
	ErrPublicationBlocked  = errors.New("publication prerequisites are incomplete")
)

type InactiveReason string

const (
	InactiveReasonSuspended InactiveReason = "suspended"
	InactiveReasonCancelled InactiveReason = "cancelled"
	InactiveReasonClosed    InactiveReason = "closed"
)

type PublicSeller struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type PublicProduct struct {
	SchemaVersion             string             `json:"schemaVersion"`
	SellerID                  domain.ID          `json:"sellerId"`
	RouteID                   domain.ID          `json:"routeId"`
	RouteVersion              uint64             `json:"routeVersion"`
	DisplayName               string             `json:"displayName"`
	ProductSlug               string             `json:"productSlug"`
	Description               string             `json:"description"`
	MIMEType                  string             `json:"mimeType"`
	InputSchema               catalog.JSONSchema `json:"inputSchema"`
	OutputSchema              catalog.JSONSchema `json:"outputSchema"`
	Amount                    string             `json:"amount"`
	Asset                     string             `json:"asset"`
	Network                   string             `json:"network"`
	PaymentProtocol           string             `json:"paymentProtocol"`
	PaymentScheme             string             `json:"paymentScheme"`
	Availability              string             `json:"availability"`
	FulfillmentMode           string             `json:"fulfillmentMode"`
	FulfillmentTimeoutSeconds int                `json:"fulfillmentTimeoutSeconds"`
	UpdatedAt                 domain.Timestamp   `json:"updatedAt"`
	AuthoritativeForPurchase  bool               `json:"authoritativeForPurchase"`
	CanonicalURL              string             `json:"canonicalUrl"`
	PurchaseSessionEndpoint   string             `json:"purchaseSessionEndpoint"`
}

type PlatformDiscoveryCapabilities struct {
	PublicDirectory           bool `json:"publicDirectory"`
	SignedStorefrontManifests bool `json:"signedStorefrontManifests"`
	SignedProductDocuments    bool `json:"signedProductDocuments"`
	LLMSText                  bool `json:"llmsText"`
}

type PlatformPaymentCapability struct {
	Protocol    string `json:"protocol"`
	Environment string `json:"environment"`
	ExactPrice  bool   `json:"exactPrice"`
	Network     string `json:"network"`
}

type PlatformCapabilities struct {
	BuyerChannels           []string                      `json:"buyerChannels"`
	Discovery               PlatformDiscoveryCapabilities `json:"discovery"`
	Payments                []PlatformPaymentCapability   `json:"payments"`
	SellerIntegration       bool                          `json:"sellerIntegration"`
	ExternalBuyerCompatible bool                          `json:"externalBuyerCompatible"`
	AgentPayBuyerRuntime    bool                          `json:"agentPayBuyerRuntime"`
	A2AExecution            bool                          `json:"a2aExecution"`
	Negotiation             bool                          `json:"negotiation"`
	Ranking                 bool                          `json:"ranking"`
}

type AgentPayPlatformManifest struct {
	SchemaVersion     string               `json:"schemaVersion"`
	Name              string               `json:"name"`
	Status            string               `json:"status"`
	CanonicalOrigin   string               `json:"canonicalOrigin"`
	APIOrigin         string               `json:"apiOrigin"`
	DirectoryEndpoint string               `json:"directoryEndpoint"`
	JWKSURI           string               `json:"jwksUri"`
	Capabilities      PlatformCapabilities `json:"capabilities"`
}

type PublicPaymentCapability struct {
	Protocol    string `json:"protocol"`
	Environment string `json:"environment"`
	ExactPrice  bool   `json:"exactPrice"`
	Asset       string `json:"asset"`
	Network     string `json:"network"`
}

type PublicFulfillmentCapability struct {
	Mode           string `json:"mode"`
	OutputMIMEType string `json:"outputMimeType"`
}

type PublicProductCapabilities struct {
	BuyerChannels []string                    `json:"buyerChannels"`
	Payment       PublicPaymentCapability     `json:"payment"`
	Fulfillment   PublicFulfillmentCapability `json:"fulfillment"`
}

type PublicProductDirectoryItem struct {
	Seller       PublicSeller              `json:"seller"`
	Product      PublicProduct             `json:"product"`
	Capabilities PublicProductCapabilities `json:"capabilities"`
}

type PublicProductDirectoryPage struct {
	SchemaVersion            string                       `json:"schemaVersion"`
	Query                    string                       `json:"query,omitempty"`
	Ordering                 string                       `json:"ordering"`
	AuthoritativeForPurchase bool                         `json:"authoritativeForPurchase"`
	Items                    []PublicProductDirectoryItem `json:"items"`
	NextCursor               string                       `json:"nextCursor,omitempty"`
}

type PublicDirectoryRequest struct {
	Query   string
	Asset   string
	Network string
	Limit   int
	Cursor  string
}

type StorefrontManifest struct {
	SchemaVersion       string           `json:"schemaVersion"`
	SellerID            domain.ID        `json:"sellerId"`
	Seller              PublicSeller     `json:"seller"`
	Availability        string           `json:"availability"`
	PublicationRevision uint64           `json:"publicationRevision"`
	IssuedAt            domain.Timestamp `json:"issuedAt"`
	ExpiresAt           domain.Timestamp `json:"expiresAt"`
	CanonicalOrigin     string           `json:"canonicalOrigin"`
	Products            []PublicProduct  `json:"products"`
}

type PublicProductDocument struct {
	SchemaVersion       string           `json:"schemaVersion"`
	SellerID            domain.ID        `json:"sellerId"`
	SellerSlug          string           `json:"sellerSlug"`
	PublicationRevision uint64           `json:"publicationRevision"`
	IssuedAt            domain.Timestamp `json:"issuedAt"`
	ExpiresAt           domain.Timestamp `json:"expiresAt"`
	CanonicalOrigin     string           `json:"canonicalOrigin"`
	Product             PublicProduct    `json:"product"`
}

type StorefrontTombstone struct {
	SchemaVersion       string           `json:"schemaVersion"`
	SellerID            domain.ID        `json:"sellerId"`
	SellerSlug          string           `json:"sellerSlug"`
	Availability        string           `json:"availability"`
	Reason              InactiveReason   `json:"reason"`
	PublicationRevision uint64           `json:"publicationRevision"`
	IssuedAt            domain.Timestamp `json:"issuedAt"`
	ExpiresAt           domain.Timestamp `json:"expiresAt"`
	CanonicalOrigin     string           `json:"canonicalOrigin"`
}

type DiscoverySignature struct {
	Algorithm        string `json:"alg"`
	KeyID            string `json:"kid"`
	Canonicalization string `json:"canonicalization"`
	DomainSeparator  string `json:"domainSeparator"`
	Value            string `json:"value"`
}

type SignedStorefrontManifest struct {
	Document  StorefrontManifest         `json:"document"`
	Signature DiscoverySignature         `json:"signature"`
	Tombstone *SignedStorefrontTombstone `json:"-"`
}

type SignedStorefrontTombstone struct {
	Document  StorefrontTombstone `json:"document"`
	Signature DiscoverySignature  `json:"signature"`
}

type SignedPublicProductDocument struct {
	Document  PublicProductDocument `json:"document"`
	Signature DiscoverySignature    `json:"signature"`
}

type CommerceOffer struct {
	Seller      catalog.Seller
	Route       catalog.PaidRoute
	Destination settlement.PaymentDestination
}

type PublicationState struct {
	SellerID            domain.ID        `json:"sellerId"`
	Fingerprint         string           `json:"fingerprint"`
	PublicationRevision uint64           `json:"publicationRevision"`
	Catalog             PublishedCatalog `json:"catalog"`
	UpdatedAt           domain.Timestamp `json:"updatedAt"`
	Version             uint64           `json:"version"`
}

// PublishedCatalog is the durable public snapshot shared by every discovery surface.
type PublishedCatalog struct {
	SellerID            domain.ID               `json:"sellerId"`
	Seller              PublicSeller            `json:"seller"`
	Availability        string                  `json:"availability"`
	InactiveReason      InactiveReason          `json:"inactiveReason,omitempty"`
	PublicationRevision uint64                  `json:"publicationRevision"`
	Routes              []PublishedRouteVersion `json:"routes"`
}

type PublishedRouteVersion struct {
	RouteID      domain.ID `json:"routeId"`
	RouteVersion uint64    `json:"routeVersion"`
}

type PublicationRepository interface {
	Get(context.Context, domain.ID) (PublicationState, error)
	Put(context.Context, PublicationState, uint64) error
}

type CatalogRepository interface {
	GetSeller(context.Context, domain.ID) (catalog.Seller, error)
	ResolveSellerBySlug(context.Context, string) (catalog.Seller, error)
	UpdateSeller(context.Context, catalog.Seller, uint64) error
	GetRoute(context.Context, domain.ID) (catalog.PaidRoute, error)
	ListRoutesBySeller(context.Context, domain.ID) ([]catalog.PaidRoute, error)
}

type DirectoryRepository interface {
	catalog.PublicDirectoryRepository
}

type DestinationRepository interface {
	ListBySeller(context.Context, domain.ID) ([]settlement.PaymentDestination, error)
}

type EntitlementReader interface {
	ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error)
}

type PublicationReadiness interface {
	AuthorizePublication(context.Context, domain.ID) error
}

type Dependencies struct {
	Catalog              CatalogRepository
	Directory            DirectoryRepository
	Destinations         DestinationRepository
	Entitlements         EntitlementReader
	PublicationReadiness PublicationReadiness
	Publications         PublicationRepository
	Signer               authorization.CapabilitySigner
	Clock                domain.Clock
	CanonicalOrigin      string
	APIOrigin            string
	AuditRecorder        audit.Recorder
}
