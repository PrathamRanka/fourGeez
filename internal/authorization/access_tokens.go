package authorization

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const (
	MCPAudience                   = "urn:agentpay:mcp"
	accessTokenType               = "agentpay-access+jwt"
	accessTokenAlgorithm          = "ES256"
	MinimumAccessTokenLifetime    = 2 * time.Minute
	MaximumAccessTokenLifetime    = 5 * time.Minute
	maximumCompactAccessTokenSize = 16 * 1024
)

var (
	ErrInvalidAccessToken       = errors.New("integration access token is invalid")
	ErrAccessTokenExpired       = errors.New("integration access token has expired")
	ErrAccessTokenRevoked       = errors.New("integration access token has been revoked")
	ErrSubscriptionInactive     = errors.New("seller subscription is inactive")
	ErrInsufficientScope        = errors.New("integration access token has insufficient scope")
	ErrAuthorizationUnavailable = errors.New("authorization dependency is unavailable")
)

type AccessTokenConfig struct {
	Issuer   string
	Audience string
	Lifetime time.Duration
}

type AccessTokenRequest struct {
	Audience string               `json:"audience"`
	Scopes   []integrations.Scope `json:"scopes"`
}

type AccessTokenResponse struct {
	AccessToken      string    `json:"accessToken"`
	TokenType        string    `json:"tokenType"`
	ExpiresIn        int64     `json:"expiresIn"`
	Scope            string    `json:"scope"`
	SellerID         domain.ID `json:"sellerId"`
	CredentialID     domain.ID `json:"credentialId"`
	EntitlementEpoch uint64    `json:"entitlementEpoch"`
}

type accessTokenHeader struct {
	Type      string `json:"typ"`
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type accessTokenClaims struct {
	Issuer           string    `json:"iss"`
	Audience         string    `json:"aud"`
	Subject          string    `json:"sub"`
	SellerID         domain.ID `json:"sellerId"`
	CredentialID     domain.ID `json:"credentialId"`
	Scope            string    `json:"scope"`
	EntitlementEpoch uint64    `json:"entitlementEpoch"`
	JWTID            domain.ID `json:"jti"`
	IssuedAt         int64     `json:"iat"`
	ExpiresAt        int64     `json:"exp"`
}

type JSONWebKey struct {
	KeyType   string   `json:"kty"`
	Use       string   `json:"use"`
	KeyOps    []string `json:"key_ops"`
	Algorithm string   `json:"alg"`
	KeyID     string   `json:"kid"`
	Curve     string   `json:"crv"`
	X         string   `json:"x"`
	Y         string   `json:"y"`
}

type JSONWebKeySet struct {
	Keys []JSONWebKey `json:"keys"`
}

type ProjectKeyExchange interface {
	AuthorizeExchange(context.Context, string, []integrations.Scope) (integrations.ExchangeAuthorization, error)
}

type CredentialReader interface {
	GetByID(context.Context, domain.ID) (integrations.Credential, error)
}

type EntitlementReader interface {
	ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error)
}

// CapabilitySigner is implemented by local ephemeral keys and production KMS.
type CapabilitySigner interface {
	CurrentKeyID(context.Context) (string, error)
	Sign(context.Context, string, []byte) ([]byte, error)
}

// CapabilityKeyProvider resolves public verification keys and publishes JWKS.
type CapabilityKeyProvider interface {
	VerificationKey(context.Context, string) (*ecdsa.PublicKey, error)
	JWKS(context.Context) (JSONWebKeySet, error)
}

type AccessTokenService struct {
	config       AccessTokenConfig
	exchange     ProjectKeyExchange
	credentials  CredentialReader
	entitlements EntitlementReader
	signer       CapabilitySigner
	keys         CapabilityKeyProvider
	idGenerator  domain.IDGenerator
	clock        domain.Clock
}

func NewAccessTokenService(
	config AccessTokenConfig,
	exchange ProjectKeyExchange,
	credentials CredentialReader,
	entitlements EntitlementReader,
	signer CapabilitySigner,
	keys CapabilityKeyProvider,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
) *AccessTokenService {
	return &AccessTokenService{
		config: config, exchange: exchange, credentials: credentials, entitlements: entitlements,
		signer: signer, keys: keys, idGenerator: idGenerator, clock: clock,
	}
}

func (service *AccessTokenService) Exchange(
	ctx context.Context,
	projectKey string,
	request AccessTokenRequest,
) (AccessTokenResponse, error) {
	if service == nil || service.exchange == nil || service.credentials == nil || service.entitlements == nil ||
		service.signer == nil || service.idGenerator == nil || service.clock == nil {
		return AccessTokenResponse{}, ErrAuthorizationUnavailable
	}
	if request.Audience != service.config.Audience || request.Audience != MCPAudience {
		return AccessTokenResponse{}, domain.NewValidationError("audience", "const", "must be urn:agentpay:mcp")
	}
	scopes, err := validatedAccessScopes(request.Scopes)
	if err != nil {
		return AccessTokenResponse{}, err
	}
	if service.config.Lifetime < MinimumAccessTokenLifetime || service.config.Lifetime > MaximumAccessTokenLifetime {
		return AccessTokenResponse{}, ErrAuthorizationUnavailable
	}
	authorization, err := service.exchange.AuthorizeExchange(ctx, projectKey, scopes)
	if err != nil {
		return AccessTokenResponse{}, err
	}
	now := service.clock.Now().UTC()
	if err := service.authorizeCurrentCapabilityState(
		ctx,
		authorization.SellerID,
		authorization.CredentialID,
		scopes,
		authorization.EntitlementEpoch,
		now,
	); err != nil {
		return AccessTokenResponse{}, err
	}
	jti, err := service.idGenerator.New(domain.CapabilityIDPrefix)
	if err != nil {
		return AccessTokenResponse{}, err
	}
	keyID, err := service.signer.CurrentKeyID(ctx)
	if err != nil {
		return AccessTokenResponse{}, ErrAuthorizationUnavailable
	}
	claims := accessTokenClaims{
		Issuer: service.config.Issuer, Audience: service.config.Audience,
		Subject: authorization.CredentialID.String(), SellerID: authorization.SellerID,
		CredentialID: authorization.CredentialID, Scope: joinScopes(scopes),
		EntitlementEpoch: authorization.EntitlementEpoch, JWTID: jti,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(service.config.Lifetime).Unix(),
	}
	token, err := service.sign(ctx, keyID, claims)
	if err != nil {
		return AccessTokenResponse{}, err
	}
	return AccessTokenResponse{
		AccessToken: token, TokenType: "Bearer", ExpiresIn: int64(service.config.Lifetime / time.Second),
		Scope: claims.Scope, SellerID: claims.SellerID, CredentialID: claims.CredentialID,
		EntitlementEpoch: claims.EntitlementEpoch,
	}, nil
}

func (service *AccessTokenService) AuthorizeAccessToken(ctx context.Context, rawToken string) (integrations.Principal, error) {
	if strings.HasPrefix(rawToken, "apc1.") || strings.HasPrefix(rawToken, "apc2.") || len(rawToken) > maximumCompactAccessTokenSize {
		return integrations.Principal{}, ErrInvalidAccessToken
	}
	if service == nil || service.keys == nil || service.credentials == nil || service.entitlements == nil || service.clock == nil {
		return integrations.Principal{}, ErrInvalidAccessToken
	}
	header, claims, signingInput, signature, err := parseAccessToken(rawToken)
	if err != nil {
		return integrations.Principal{}, ErrInvalidAccessToken
	}
	if header.Type != accessTokenType || header.Algorithm != accessTokenAlgorithm || strings.TrimSpace(header.KeyID) == "" {
		return integrations.Principal{}, ErrInvalidAccessToken
	}
	publicKey, err := service.keys.VerificationKey(ctx, header.KeyID)
	if err != nil || !verifyES256(publicKey, signingInput, signature) {
		return integrations.Principal{}, ErrInvalidAccessToken
	}
	now := service.clock.Now().UTC()
	if err := service.validateClaims(claims, now); err != nil {
		return integrations.Principal{}, err
	}
	scopes, err := parseScopeClaim(claims.Scope)
	if err != nil {
		return integrations.Principal{}, ErrInvalidAccessToken
	}
	if err := service.authorizeCurrentCapabilityState(
		ctx,
		claims.SellerID,
		claims.CredentialID,
		scopes,
		claims.EntitlementEpoch,
		now,
	); err != nil {
		return integrations.Principal{}, err
	}
	return integrations.Principal{SellerID: claims.SellerID, CredentialID: claims.CredentialID, Scopes: scopes}, nil
}

func (service *AccessTokenService) authorizeCurrentCapabilityState(
	ctx context.Context,
	sellerID domain.ID,
	credentialID domain.ID,
	scopes []integrations.Scope,
	entitlementEpoch uint64,
	now time.Time,
) error {
	credential, err := service.credentials.GetByID(ctx, credentialID)
	if errors.Is(err, persistence.ErrNotFound) {
		return ErrAccessTokenRevoked
	}
	if err != nil {
		return ErrAuthorizationUnavailable
	}
	if credential.CredentialID() != credentialID || credential.SellerID() != sellerID ||
		credential.RevokedAt() != nil || credential.EntitlementEpoch() != entitlementEpoch {
		return ErrAccessTokenRevoked
	}
	if expiresAt := credential.ExpiresAt(); expiresAt != nil && !now.Before(expiresAt.Time()) {
		return ErrAccessTokenRevoked
	}
	if !scopesSubset(scopes, credential.Scopes()) {
		return ErrAccessTokenRevoked
	}
	entitlement, err := service.entitlements.ResolveSellerPlan(ctx, sellerID)
	if errors.Is(err, billing.ErrSellerEntitlementNotFound) || errors.Is(err, persistence.ErrNotFound) {
		return ErrSubscriptionInactive
	}
	if err != nil {
		return ErrAuthorizationUnavailable
	}
	if entitlement.Assignment.SellerID != sellerID {
		return ErrAccessTokenRevoked
	}
	if entitlement.Assignment.Status != billing.EntitlementStatusActive || !now.Before(entitlement.Assignment.AccessEndsAt.Time()) {
		return ErrSubscriptionInactive
	}
	if entitlement.Assignment.EntitlementEpoch != entitlementEpoch {
		return ErrAccessTokenRevoked
	}
	return nil
}

func (service *AccessTokenService) JWKS(ctx context.Context) (JSONWebKeySet, error) {
	if service == nil || service.keys == nil {
		return JSONWebKeySet{}, ErrAuthorizationUnavailable
	}
	return service.keys.JWKS(ctx)
}

func (service *AccessTokenService) sign(ctx context.Context, keyID string, claims accessTokenClaims) (string, error) {
	headerBytes, err := json.Marshal(accessTokenHeader{Type: accessTokenType, Algorithm: accessTokenAlgorithm, KeyID: keyID})
	if err != nil {
		return "", err
	}
	claimBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimBytes)
	signingInput := encodedHeader + "." + encodedClaims
	signature, err := service.signer.Sign(ctx, keyID, []byte(signingInput))
	if err != nil {
		return "", ErrAuthorizationUnavailable
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (service *AccessTokenService) validateClaims(claims accessTokenClaims, now time.Time) error {
	if _, err := domain.ParseID(claims.SellerID.String(), domain.SellerIDPrefix); err != nil {
		return ErrInvalidAccessToken
	}
	if _, err := domain.ParseID(claims.CredentialID.String(), domain.CredentialIDPrefix); err != nil {
		return ErrInvalidAccessToken
	}
	if _, err := domain.ParseID(claims.JWTID.String(), domain.CapabilityIDPrefix); err != nil {
		return ErrInvalidAccessToken
	}
	if claims.Issuer != service.config.Issuer || claims.Audience != service.config.Audience ||
		claims.Subject != claims.CredentialID.String() ||
		claims.EntitlementEpoch == 0 || claims.IssuedAt <= 0 || claims.ExpiresAt <= claims.IssuedAt {
		return ErrInvalidAccessToken
	}
	lifetime := time.Duration(claims.ExpiresAt-claims.IssuedAt) * time.Second
	if lifetime < MinimumAccessTokenLifetime || lifetime > MaximumAccessTokenLifetime || claims.IssuedAt > now.Unix() {
		return ErrInvalidAccessToken
	}
	if now.Unix() >= claims.ExpiresAt {
		return ErrAccessTokenExpired
	}
	if _, err := parseScopeClaim(claims.Scope); err != nil {
		return ErrInvalidAccessToken
	}
	return nil
}

func parseAccessToken(rawToken string) (accessTokenHeader, accessTokenClaims, []byte, []byte, error) {
	segments := strings.Split(rawToken, ".")
	if len(segments) != 3 || segments[0] == "" || segments[1] == "" || segments[2] == "" {
		return accessTokenHeader{}, accessTokenClaims{}, nil, nil, ErrInvalidAccessToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(segments[0])
	if err != nil {
		return accessTokenHeader{}, accessTokenClaims{}, nil, nil, err
	}
	claimBytes, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		return accessTokenHeader{}, accessTokenClaims{}, nil, nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(segments[2])
	if err != nil {
		return accessTokenHeader{}, accessTokenClaims{}, nil, nil, err
	}
	var header accessTokenHeader
	if err := decodeStrictJSON(headerBytes, &header); err != nil {
		return accessTokenHeader{}, accessTokenClaims{}, nil, nil, err
	}
	var claims accessTokenClaims
	if err := decodeStrictJSON(claimBytes, &claims); err != nil {
		return accessTokenHeader{}, accessTokenClaims{}, nil, nil, err
	}
	return header, claims, []byte(segments[0] + "." + segments[1]), signature, nil
}

func decodeStrictJSON(encoded []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if decoder.More() {
		return ErrInvalidAccessToken
	}
	return nil
}

func validatedAccessScopes(scopes []integrations.Scope) ([]integrations.Scope, error) {
	if len(scopes) == 0 {
		return nil, domain.NewValidationError("scopes", "minimum", "must contain at least one MCP scope")
	}
	seen := make(map[integrations.Scope]struct{}, len(scopes))
	validated := append([]integrations.Scope(nil), scopes...)
	for _, scope := range validated {
		if scope != integrations.ScopeRead && scope != integrations.ScopeConfigure && scope != integrations.ScopePublish && scope != integrations.ScopeValidate {
			return nil, domain.NewValidationError("scopes", "enum", "contains an unsupported MCP scope")
		}
		if _, exists := seen[scope]; exists {
			return nil, domain.NewValidationError("scopes", "unique", "must not contain duplicate scopes")
		}
		seen[scope] = struct{}{}
	}
	sort.Slice(validated, func(left, right int) bool { return validated[left] < validated[right] })
	return validated, nil
}

func parseScopeClaim(scopeClaim string) ([]integrations.Scope, error) {
	parts := strings.Split(scopeClaim, " ")
	scopes := make([]integrations.Scope, len(parts))
	for index, part := range parts {
		scopes[index] = integrations.Scope(part)
	}
	validated, err := validatedAccessScopes(scopes)
	if err != nil || joinScopes(validated) != scopeClaim {
		return nil, ErrInvalidAccessToken
	}
	return validated, nil
}

func joinScopes(scopes []integrations.Scope) string {
	parts := make([]string, len(scopes))
	for index, scope := range scopes {
		parts[index] = string(scope)
	}
	return strings.Join(parts, " ")
}

func scopesSubset(requested, granted []integrations.Scope) bool {
	for _, requestedScope := range requested {
		found := false
		for _, grantedScope := range granted {
			if requestedScope == grantedScope {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func verifyES256(publicKey *ecdsa.PublicKey, signingInput, signature []byte) bool {
	if publicKey == nil || len(signature) != 64 {
		return false
	}
	digest := sha256.Sum256(signingInput)
	return ecdsa.Verify(publicKey, digest[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:]))
}

type localSigningKey struct {
	key          *ecdsa.PrivateKey
	keyID        string
	publishUntil time.Time
}

// LocalES256KeyRing is an ephemeral development signer with one overlapping key.
type LocalES256KeyRing struct {
	mutex   sync.RWMutex
	clock   domain.Clock
	current localSigningKey
	retired []localSigningKey
}

func NewLocalES256KeyRing(clock domain.Clock) (*LocalES256KeyRing, error) {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	key, err := newLocalSigningKey()
	if err != nil {
		return nil, err
	}
	return &LocalES256KeyRing{clock: clock, current: key}, nil
}

func (ring *LocalES256KeyRing) Rotate() error {
	key, err := newLocalSigningKey()
	if err != nil {
		return err
	}
	ring.mutex.Lock()
	defer ring.mutex.Unlock()
	previous := ring.current
	previous.publishUntil = ring.clock.Now().UTC().Add(MaximumAccessTokenLifetime)
	ring.retired = append([]localSigningKey{previous}, ring.activeRetiredLocked()...)
	ring.current = key
	return nil
}

func (ring *LocalES256KeyRing) CurrentKeyID(context.Context) (string, error) {
	ring.mutex.RLock()
	defer ring.mutex.RUnlock()
	return ring.current.keyID, nil
}

func (ring *LocalES256KeyRing) Sign(_ context.Context, keyID string, signingInput []byte) ([]byte, error) {
	ring.mutex.RLock()
	defer ring.mutex.RUnlock()
	if ring.current.keyID != keyID {
		return nil, ErrAuthorizationUnavailable
	}
	digest := sha256.Sum256(signingInput)
	r, s, err := ecdsa.Sign(cryptorand.Reader, ring.current.key, digest[:])
	if err != nil {
		return nil, err
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return signature, nil
}

func (ring *LocalES256KeyRing) VerificationKey(_ context.Context, keyID string) (*ecdsa.PublicKey, error) {
	ring.mutex.Lock()
	defer ring.mutex.Unlock()
	if ring.current.keyID == keyID {
		return &ring.current.key.PublicKey, nil
	}
	ring.retired = ring.activeRetiredLocked()
	for _, key := range ring.retired {
		if key.keyID == keyID {
			return &key.key.PublicKey, nil
		}
	}
	return nil, ErrInvalidAccessToken
}

func (ring *LocalES256KeyRing) JWKS(context.Context) (JSONWebKeySet, error) {
	ring.mutex.Lock()
	defer ring.mutex.Unlock()
	ring.retired = ring.activeRetiredLocked()
	keys := []JSONWebKey{publicJWK(ring.current)}
	for _, key := range ring.retired {
		keys = append(keys, publicJWK(key))
	}
	return JSONWebKeySet{Keys: keys}, nil
}

func (ring *LocalES256KeyRing) activeRetiredLocked() []localSigningKey {
	now := ring.clock.Now().UTC()
	active := make([]localSigningKey, 0, len(ring.retired))
	for _, key := range ring.retired {
		if now.Before(key.publishUntil) {
			active = append(active, key)
		}
	}
	return active
}

func newLocalSigningKey() (localSigningKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	if err != nil {
		return localSigningKey{}, err
	}
	identifier := make([]byte, 16)
	if _, err := cryptorand.Read(identifier); err != nil {
		return localSigningKey{}, err
	}
	return localSigningKey{key: key, keyID: "local-" + base64.RawURLEncoding.EncodeToString(identifier)}, nil
}

func publicJWK(key localSigningKey) JSONWebKey {
	x := make([]byte, 32)
	y := make([]byte, 32)
	key.key.PublicKey.X.FillBytes(x)
	key.key.PublicKey.Y.FillBytes(y)
	return JSONWebKey{
		KeyType: "EC", Use: "sig", KeyOps: []string{"verify"}, Algorithm: accessTokenAlgorithm,
		KeyID: key.keyID, Curve: "P-256", X: base64.RawURLEncoding.EncodeToString(x), Y: base64.RawURLEncoding.EncodeToString(y),
	}
}

var _ CapabilitySigner = (*LocalES256KeyRing)(nil)
var _ CapabilityKeyProvider = (*LocalES256KeyRing)(nil)
