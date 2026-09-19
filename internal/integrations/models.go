package integrations

import (
	"context"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	IntegrationVerificationSchemaVersion = "agentpay.sandbox.v2"

	IntegrationVerificationCheckEndpointReachability = "endpoint_reachability"
	IntegrationVerificationCheckSignedExchange       = "signed_exchange"
	IntegrationVerificationCheckSchemaContract       = "schema_contract"
	IntegrationVerificationCheckFulfillmentReadiness = "fulfillment_readiness"
	IntegrationVerificationCheckPaymentGating        = "payment_gating"
	IntegrationVerificationCheckReplayIdempotency    = "replay_idempotency"
)

// IntegrationVerificationCheck records one bounded seller integration requirement.
type IntegrationVerificationCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// IntegrationVerificationResult is the latest route-version-bound sandbox outcome.
type IntegrationVerificationResult struct {
	SchemaVersion string                         `json:"schemaVersion"`
	SellerID      domain.ID                      `json:"sellerId"`
	RouteID       domain.ID                      `json:"routeId"`
	RouteVersion  uint64                         `json:"routeVersion"`
	CompletedAt   domain.Timestamp               `json:"completedAt"`
	Valid         bool                           `json:"valid"`
	Checks        []IntegrationVerificationCheck `json:"checks"`
}

// Scope identifies one capability granted to a seller integration.
type Scope string

const (
	ScopeRead      Scope = "read"
	ScopeConfigure Scope = "configure"
	ScopePublish   Scope = "publish"
	ScopeValidate  Scope = "validate"
	ScopeRotate    Scope = "rotate"
)

// CredentialParams contains validated credential construction values.
type CredentialParams struct {
	CredentialID           domain.ID
	SellerID               domain.ID
	TokenHash              string
	Label                  string
	Scopes                 []Scope
	EntitlementEpoch       uint64
	ExpiresAt              *domain.Timestamp
	LastUsedAt             *domain.Timestamp
	ReplacedByCredentialID *domain.ID
	CreatedAt              domain.Timestamp
	UpdatedAt              domain.Timestamp
	RevokedAt              *domain.Timestamp
	Version                uint64
}

// Credential stores only the digest and metadata for one integration token.
type Credential struct {
	credentialID           domain.ID
	sellerID               domain.ID
	tokenHash              string
	label                  string
	scopes                 []Scope
	entitlementEpoch       uint64
	expiresAt              *domain.Timestamp
	revokedAt              *domain.Timestamp
	lastUsedAt             *domain.Timestamp
	replacedByCredentialID *domain.ID
	createdAt              domain.Timestamp
	updatedAt              domain.Timestamp
	version                uint64
}

// CreateCredentialRequest contains seller-approved credential configuration.
type CreateCredentialRequest struct {
	Label     string            `json:"label"`
	Scopes    []Scope           `json:"scopes"`
	ExpiresAt *domain.Timestamp `json:"expiresAt"`
}

// RevokeCredentialRequest provides optimistic concurrency for revocation.
type RevokeCredentialRequest struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
}

type RotateCredentialRequest struct {
	ExpectedVersion uint64            `json:"expectedVersion"`
	Label           string            `json:"label,omitempty"`
	Scopes          []Scope           `json:"scopes,omitempty"`
	ExpiresAt       *domain.Timestamp `json:"expiresAt,omitempty"`
}

// RotationIdempotency carries the caller key and exact request bytes used to
// bind a credential-rotation replay record.
type RotationIdempotency struct {
	Key         string
	RequestBody []byte
}

// CredentialView is the redacted public credential representation.
type CredentialView struct {
	CredentialID           domain.ID         `json:"credentialId"`
	SellerID               domain.ID         `json:"sellerId"`
	Label                  string            `json:"label"`
	Scopes                 []Scope           `json:"scopes"`
	ExpiresAt              *domain.Timestamp `json:"expiresAt"`
	RevokedAt              *domain.Timestamp `json:"revokedAt"`
	LastUsedAt             *domain.Timestamp `json:"lastUsedAt"`
	ReplacedByCredentialID *domain.ID        `json:"replacedByCredentialId"`
	CreatedAt              domain.Timestamp  `json:"createdAt"`
	UpdatedAt              domain.Timestamp  `json:"updatedAt"`
	Version                uint64            `json:"version"`
}

type CredentialRotation struct {
	Predecessor           CredentialView   `json:"predecessor"`
	Successor             CredentialView   `json:"successor"`
	Token                 string           `json:"token"`
	SecretReplayExpiresAt domain.Timestamp `json:"secretReplayExpiresAt"`
}

// RotationReplay stores only protected response material for an exact retry.
type RotationReplay struct {
	Scope                 string
	Key                   domain.IdempotencyKey
	RequestHash           string
	ProtectedResponse     []byte
	SuccessorCredentialID domain.ID
	CreatedAt             domain.Timestamp
	ExpiresAt             domain.Timestamp
}

// CredentialCreated returns raw token material exactly once at creation.
type CredentialCreated struct {
	CredentialView
	Token string `json:"token"`
}

// Principal is the authenticated seller integration identity.
type Principal struct {
	SellerID     domain.ID
	CredentialID domain.ID
	Scopes       []Scope
}

type ExchangeAuthorization struct {
	Principal
	EntitlementEpoch uint64
}

// HasScope reports whether the credential grants one exact capability.
func (principal Principal) HasScope(required Scope) bool {
	for _, scope := range principal.Scopes {
		if scope == required {
			return true
		}
	}
	return false
}

// Repository persists seller-scoped integration credentials.
type Repository interface {
	Create(context.Context, Credential) error
	Get(context.Context, domain.ID, domain.ID) (Credential, error)
	GetByID(context.Context, domain.ID) (Credential, error)
	ListBySeller(context.Context, domain.ID) ([]Credential, error)
	Update(context.Context, Credential, uint64) error
	Rotate(context.Context, Credential, Credential, uint64, RotationReplay) error
	LoadRotationReplay(context.Context, string, domain.IdempotencyKey) (RotationReplay, bool, error)
}

// SellerAuthorizer verifies that a user owns the requested seller.
type SellerAuthorizer interface {
	AuthorizeSeller(context.Context, string, domain.ID) error
}

// CredentialIssuanceAuthorizer verifies all authoritative onboarding
// prerequisites after the normal seller ownership check succeeds.
type CredentialIssuanceAuthorizer interface {
	AuthorizeCredentialIssuance(context.Context, string, domain.ID) error
}

// TokenGenerator creates unpredictable credential secret material.
type TokenGenerator interface {
	NewToken() (string, error)
}

type CredentialPepperProvider interface {
	CredentialPepper(context.Context) ([]byte, error)
}

type CredentialDigester interface {
	Digest(context.Context, string) (string, error)
}

// RotationReplayProtector keeps one-time rotation responses protected at rest.
// Production implementations use a cloud KMS envelope-encryption boundary.
type RotationReplayProtector interface {
	Seal(context.Context, []byte) ([]byte, error)
	Open(context.Context, []byte) ([]byte, error)
}

type EntitlementResolver interface {
	ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error)
}

type ExchangeRateLimiter interface {
	AllowProjectKeyExchange(context.Context, domain.ID) error
}

type ExchangeQuotaEnforcer interface {
	ConsumeAPIRequest(context.Context, domain.ID) error
}

// CredentialID returns the public credential identifier.
func (credential Credential) CredentialID() domain.ID {
	return credential.credentialID
}

// SellerID returns the credential's fixed seller scope.
func (credential Credential) SellerID() domain.ID {
	return credential.sellerID
}

// TokenHash returns the one-way digest used for authentication.
func (credential Credential) TokenHash() string {
	return credential.tokenHash
}

// Label returns the seller-visible installation label.
func (credential Credential) Label() string {
	return credential.label
}

// Scopes returns a copy of the granted scopes.
func (credential Credential) Scopes() []Scope {
	return append([]Scope(nil), credential.scopes...)
}

func (credential Credential) EntitlementEpoch() uint64 { return credential.entitlementEpoch }

// ExpiresAt returns the optional expiration timestamp.
func (credential Credential) ExpiresAt() *domain.Timestamp {
	return copyTimestamp(credential.expiresAt)
}

// RevokedAt returns the optional revocation timestamp.
func (credential Credential) RevokedAt() *domain.Timestamp {
	return copyTimestamp(credential.revokedAt)
}

func (credential Credential) LastUsedAt() *domain.Timestamp {
	return copyTimestamp(credential.lastUsedAt)
}

func (credential Credential) ReplacedByCredentialID() *domain.ID {
	if credential.replacedByCredentialID == nil {
		return nil
	}
	copy := *credential.replacedByCredentialID
	return &copy
}

// CreatedAt returns the creation timestamp.
func (credential Credential) CreatedAt() domain.Timestamp {
	return credential.createdAt
}

// UpdatedAt returns the latest mutation timestamp.
func (credential Credential) UpdatedAt() domain.Timestamp {
	return credential.updatedAt
}

// Version returns the optimistic-concurrency version.
func (credential Credential) Version() uint64 {
	return credential.version
}

// HasScope reports whether the credential grants the requested capability.
func (credential Credential) HasScope(required Scope) bool {
	for _, scope := range credential.scopes {
		if scope == required {
			return true
		}
	}
	return false
}

// copyTimestamp prevents callers from mutating optional timestamp pointers.
func copyTimestamp(timestamp *domain.Timestamp) *domain.Timestamp {
	if timestamp == nil {
		return nil
	}
	copy := *timestamp
	return &copy
}
