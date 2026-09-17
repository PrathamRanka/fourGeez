package integrations

import (
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const (
	credentialTokenVersion  = "apc1"
	minimumCredentialSecret = 32
	maximumCredentialLabel  = 80
)

var (
	ErrCredentialInvalid = errors.New("integration credential is invalid")
	ErrCredentialExpired = errors.New("integration credential has expired")
	ErrCredentialRevoked = errors.New("integration credential has been revoked")
	ErrScopeDenied       = errors.New("integration credential scope denied")
)

// SecureTokenGenerator creates URL-safe credential secret material.
type SecureTokenGenerator struct {
	reader io.Reader
}

// NewSecureTokenGenerator uses crypto/rand when no reader is supplied.
func NewSecureTokenGenerator(reader io.Reader) *SecureTokenGenerator {
	if reader == nil {
		reader = cryptorand.Reader
	}
	return &SecureTokenGenerator{reader: reader}
}

// NewToken returns 256 bits of URL-safe random credential material.
func (generator *SecureTokenGenerator) NewToken() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := io.ReadFull(generator.reader, randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

// Service owns integration-credential issuance and authentication rules.
type Service struct {
	repository       Repository
	sellerAuthorizer SellerAuthorizer
	idGenerator      domain.IDGenerator
	tokenGenerator   TokenGenerator
	clock            domain.Clock
}

// NewService creates the integration credential application service.
func NewService(
	repository Repository,
	sellerAuthorizer SellerAuthorizer,
	idGenerator domain.IDGenerator,
	tokenGenerator TokenGenerator,
	clock domain.Clock,
) *Service {
	return &Service{
		repository:       repository,
		sellerAuthorizer: sellerAuthorizer,
		idGenerator:      idGenerator,
		tokenGenerator:   tokenGenerator,
		clock:            clock,
	}
}

// Create issues one seller-scoped credential and returns its token once.
func (service *Service) Create(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	request CreateCredentialRequest,
) (CredentialCreated, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(
		ctx,
		ownerSubject,
		sellerID,
	); err != nil {
		return CredentialCreated{}, err
	}

	credentialID, err := service.idGenerator.New(domain.CredentialIDPrefix)
	if err != nil {
		return CredentialCreated{}, err
	}
	secret, err := service.tokenGenerator.NewToken()
	if err != nil {
		return CredentialCreated{}, err
	}
	if len(secret) < minimumCredentialSecret {
		return CredentialCreated{}, errors.New("credential token generator returned insufficient entropy")
	}

	rawToken := credentialToken(sellerID, credentialID, secret)
	createdAt := domain.NewTimestamp(service.clock.Now())
	credential, err := NewCredential(CredentialParams{
		CredentialID: credentialID,
		SellerID:     sellerID,
		TokenHash:    hashCredentialToken(rawToken),
		Label:        request.Label,
		Scopes:       request.Scopes,
		ExpiresAt:    request.ExpiresAt,
		CreatedAt:    createdAt,
	})
	if err != nil {
		return CredentialCreated{}, err
	}
	if err := service.repository.Create(ctx, credential); err != nil {
		return CredentialCreated{}, err
	}

	return CredentialCreated{
		CredentialView: credentialView(credential),
		Token:          rawToken,
	}, nil
}

// List returns redacted credentials after checking seller ownership.
func (service *Service) List(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
) ([]CredentialView, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(
		ctx,
		ownerSubject,
		sellerID,
	); err != nil {
		return nil, err
	}
	credentials, err := service.repository.ListBySeller(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	views := make([]CredentialView, len(credentials))
	for index, credential := range credentials {
		views[index] = credentialView(credential)
	}
	return views, nil
}

// Revoke immediately disables one credential using optimistic concurrency.
func (service *Service) Revoke(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	credentialID domain.ID,
	expectedVersion uint64,
) (CredentialView, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(
		ctx,
		ownerSubject,
		sellerID,
	); err != nil {
		return CredentialView{}, err
	}
	credential, err := service.repository.Get(ctx, sellerID, credentialID)
	if err != nil {
		return CredentialView{}, err
	}
	if err := credential.Revoke(
		domain.NewTimestamp(service.clock.Now()),
	); err != nil {
		return CredentialView{}, err
	}
	if err := service.repository.Update(
		ctx,
		credential,
		expectedVersion,
	); err != nil {
		return CredentialView{}, err
	}
	return credentialView(credential), nil
}

// Authenticate verifies token binding, lifetime, revocation, and required scope.
func (service *Service) Authenticate(
	ctx context.Context,
	rawToken string,
	requiredScope Scope,
) (Principal, error) {
	sellerID, credentialID, err := parseCredentialToken(rawToken)
	if err != nil {
		return Principal{}, ErrCredentialInvalid
	}
	credential, err := service.repository.Get(ctx, sellerID, credentialID)
	if errors.Is(err, persistence.ErrNotFound) {
		return Principal{}, ErrCredentialInvalid
	}
	if err != nil {
		return Principal{}, err
	}
	if !hmac.Equal(
		[]byte(credential.TokenHash()),
		[]byte(hashCredentialToken(rawToken)),
	) {
		return Principal{}, ErrCredentialInvalid
	}
	if credential.RevokedAt() != nil {
		return Principal{}, ErrCredentialRevoked
	}
	now := service.clock.Now()
	if expiresAt := credential.ExpiresAt(); expiresAt != nil && !now.Before(expiresAt.Time()) {
		return Principal{}, ErrCredentialExpired
	}
	if !credential.HasScope(requiredScope) {
		return Principal{}, ErrScopeDenied
	}

	return Principal{
		SellerID:     sellerID,
		CredentialID: credentialID,
		Scopes:       credential.Scopes(),
	}, nil
}

// NewCredential validates and creates persisted credential metadata.
func NewCredential(params CredentialParams) (Credential, error) {
	validationErrors := validateCredentialParams(params)
	if len(validationErrors) > 0 {
		return Credential{}, validationErrors
	}

	return Credential{
		credentialID: params.CredentialID,
		sellerID:     params.SellerID,
		tokenHash:    params.TokenHash,
		label:        strings.TrimSpace(params.Label),
		scopes:       append([]Scope(nil), params.Scopes...),
		expiresAt:    copyTimestamp(params.ExpiresAt),
		createdAt:    params.CreatedAt,
		updatedAt:    params.CreatedAt,
		version:      1,
	}, nil
}

// Revoke marks the credential unusable without deleting its audit metadata.
func (credential *Credential) Revoke(revokedAt domain.Timestamp) error {
	if credential.revokedAt != nil {
		return ErrCredentialRevoked
	}
	if revokedAt.Before(credential.updatedAt) {
		return domain.NewValidationError(
			"revokedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	revokedAtCopy := revokedAt
	credential.revokedAt = &revokedAtCopy
	credential.updatedAt = revokedAt
	credential.version++
	return nil
}

// validateCredentialParams enforces identity, secret, scope, and time rules.
func validateCredentialParams(params CredentialParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(
		params.CredentialID.String(),
		domain.CredentialIDPrefix,
	); err != nil {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"credentialId",
				"format",
				"must be an integration credential identifier",
			),
		)
	}
	if _, err := domain.ParseID(
		params.SellerID.String(),
		domain.SellerIDPrefix,
	); err != nil {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"sellerId",
				"format",
				"must be a seller identifier",
			),
		)
	}
	label := strings.TrimSpace(params.Label)
	if label == "" || len(label) > maximumCredentialLabel {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"label",
				"length",
				"must contain 1-80 characters",
			),
		)
	}
	decodedHash, hashError := hex.DecodeString(params.TokenHash)
	if hashError != nil || len(decodedHash) != sha256.Size {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"tokenHash",
				"format",
				"must be a SHA-256 digest",
			),
		)
	}
	validationErrors = append(
		validationErrors,
		validateScopes(params.Scopes)...,
	)
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"createdAt",
				"required",
				"is required",
			),
		)
	}
	if params.ExpiresAt != nil &&
		!params.ExpiresAt.Time().After(params.CreatedAt.Time()) {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"expiresAt",
				"chronology",
				"must occur after creation",
			),
		)
	}
	return validationErrors
}

// validateScopes rejects missing, duplicate, and unsupported capabilities.
func validateScopes(scopes []Scope) domain.ValidationErrors {
	if len(scopes) == 0 {
		return domain.ValidationErrors{
			domain.NewValidationError(
				"scopes",
				"required",
				"must contain at least one scope",
			),
		}

	}

	seen := make(map[Scope]struct{}, len(scopes))
	var validationErrors domain.ValidationErrors
	for _, scope := range scopes {
		if !supportedScope(scope) {
			validationErrors = append(
				validationErrors,
				domain.NewValidationError(
					"scopes",
					"supported",
					fmt.Sprintf("unsupported scope %q", scope),
				),
			)
			continue
		}
		if _, exists := seen[scope]; exists {
			validationErrors = append(
				validationErrors,
				domain.NewValidationError(
					"scopes",
					"unique",
					"must not contain duplicate scopes",
				),
			)
		}
		seen[scope] = struct{}{}
	}
	return validationErrors
}

// supportedScope reports whether a scope belongs to the fixed capability set.
func supportedScope(scope Scope) bool {
	switch scope {
	case ScopeRead, ScopeConfigure, ScopePublish, ScopeValidate, ScopeRotate:
		return true
	default:
		return false
	}
}

// credentialToken creates the versioned point-addressable credential value.
func credentialToken(
	sellerID domain.ID,
	credentialID domain.ID,
	secret string,
) string {
	return strings.Join(
		[]string{
			credentialTokenVersion,
			sellerID.String(),
			credentialID.String(),
			secret,
		},
		".",
	)
}

// parseCredentialToken validates the non-secret routing segments.
func parseCredentialToken(rawToken string) (domain.ID, domain.ID, error) {
	segments := strings.Split(rawToken, ".")
	if len(segments) != 4 ||
		segments[0] != credentialTokenVersion ||
		len(segments[3]) < minimumCredentialSecret {
		return "", "", ErrCredentialInvalid
	}
	sellerID, err := domain.ParseID(segments[1], domain.SellerIDPrefix)
	if err != nil {
		return "", "", ErrCredentialInvalid
	}
	credentialID, err := domain.ParseID(
		segments[2],
		domain.CredentialIDPrefix,
	)
	if err != nil {
		return "", "", ErrCredentialInvalid
	}
	return sellerID, credentialID, nil
}

// hashCredentialToken creates the only token representation stored at rest.
func hashCredentialToken(rawToken string) string {
	digest := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(digest[:])
}

// credentialView removes the token hash from service responses.
func credentialView(credential Credential) CredentialView {
	return CredentialView{
		CredentialID: credential.CredentialID(),
		SellerID:     credential.SellerID(),
		Label:        credential.Label(),
		Scopes:       credential.Scopes(),
		ExpiresAt:    credential.ExpiresAt(),
		RevokedAt:    credential.RevokedAt(),
		CreatedAt:    credential.CreatedAt(),
		UpdatedAt:    credential.UpdatedAt(),
		Version:      credential.Version(),
	}
}
