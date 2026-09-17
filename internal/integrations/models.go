package integrations

import (
	"context"

	"github.com/fourgeez/agentpay/internal/domain"
)

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
	CredentialID domain.ID
	SellerID     domain.ID
	TokenHash    string
	Label        string
	Scopes       []Scope
	ExpiresAt    *domain.Timestamp
	CreatedAt    domain.Timestamp
}

// Credential stores only the digest and metadata for one integration token.
type Credential struct {
	credentialID domain.ID
	sellerID     domain.ID
	tokenHash    string
	label        string
	scopes       []Scope
	expiresAt    *domain.Timestamp
	revokedAt    *domain.Timestamp
	createdAt    domain.Timestamp
	updatedAt    domain.Timestamp
	version      uint64
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

// CredentialView is the redacted public credential representation.
type CredentialView struct {
	CredentialID domain.ID         `json:"credentialId"`
	SellerID     domain.ID         `json:"sellerId"`
	Label        string            `json:"label"`
	Scopes       []Scope           `json:"scopes"`
	ExpiresAt    *domain.Timestamp `json:"expiresAt"`
	RevokedAt    *domain.Timestamp `json:"revokedAt"`
	CreatedAt    domain.Timestamp  `json:"createdAt"`
	UpdatedAt    domain.Timestamp  `json:"updatedAt"`
	Version      uint64            `json:"version"`
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
	ListBySeller(context.Context, domain.ID) ([]Credential, error)
	Update(context.Context, Credential, uint64) error
}

// SellerAuthorizer verifies that a user owns the requested seller.
type SellerAuthorizer interface {
	AuthorizeSeller(context.Context, string, domain.ID) error
}

// TokenGenerator creates unpredictable credential secret material.
type TokenGenerator interface {
	NewToken() (string, error)
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

// ExpiresAt returns the optional expiration timestamp.
func (credential Credential) ExpiresAt() *domain.Timestamp {
	return copyTimestamp(credential.expiresAt)
}

// RevokedAt returns the optional revocation timestamp.
func (credential Credential) RevokedAt() *domain.Timestamp {
	return copyTimestamp(credential.revokedAt)
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
