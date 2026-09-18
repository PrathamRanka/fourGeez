package identity

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

const (
	maximumAccessTokenBytes = 16 * 1024
	maximumJWKSBytes        = 1 << 20
	maximumJWKSKeys         = 20
	defaultJWKSCacheTTL     = time.Hour
)

// CognitoConfig contains the exact user-pool token contract.
type CognitoConfig struct {
	Issuer     string
	ClientID   string
	HTTPClient *http.Client
	Clock      domain.Clock
}

// CognitoVerifier verifies Cognito access JWTs and caches only public keys.
type CognitoVerifier struct {
	issuer     string
	clientID   string
	jwksURL    string
	httpClient *http.Client
	clock      domain.Clock

	mutex        sync.RWMutex
	keys         map[string]*rsa.PublicKey
	keysExpireAt time.Time
}

type cognitoClaims struct {
	ClientID  string `json:"client_id"`
	TokenUse  string `json:"token_use"`
	OriginJTI string `json:"origin_jti"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	jwt.RegisteredClaims
}

type jwksDocument struct {
	Keys []rsaJWK `json:"keys"`
}

type rsaJWK struct {
	KeyID     string `json:"kid"`
	KeyType   string `json:"kty"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
}

// NewCognitoVerifier creates a strict Cognito access-token verifier.
func NewCognitoVerifier(config CognitoConfig) (*CognitoVerifier, error) {
	issuer := strings.TrimRight(strings.TrimSpace(config.Issuer), "/")
	clientID := strings.TrimSpace(config.ClientID)
	if issuer == "" || clientID == "" || config.Clock == nil {
		return nil, errors.New("Cognito issuer, client ID, and clock are required")
	}
	issuerURL, err := url.Parse(issuer)
	if err != nil || issuerURL.Host == "" || (issuerURL.Scheme != "https" && !isLoopbackHTTP(issuerURL)) {
		return nil, errors.New("Cognito issuer must use HTTPS")
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &CognitoVerifier{
		issuer: issuer, clientID: clientID, jwksURL: issuer + "/.well-known/jwks.json",
		httpClient: httpClient, clock: config.Clock, keys: make(map[string]*rsa.PublicKey),
	}, nil
}

// Verify checks signature, issuer, token use, client, timestamps, and session identity.
func (verifier *CognitoVerifier) Verify(ctx context.Context, rawToken string) (Claims, error) {
	if verifier == nil || len(rawToken) == 0 || len(rawToken) > maximumAccessTokenBytes {
		return Claims{}, ErrTokenInvalid
	}
	claims := &cognitoClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuer(verifier.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(verifier.clock.Now),
	)
	token, err := parser.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		keyID, ok := token.Header["kid"].(string)
		if !ok || !validIdentityValue(keyID) {
			return nil, ErrTokenInvalid
		}
		return verifier.publicKey(ctx, keyID)
	})
	if err != nil || token == nil || !token.Valid {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Claims{}, ErrTokenExpired
		}
		if errors.Is(err, ErrIdentityUnavailable) {
			return Claims{}, err
		}
		return Claims{}, ErrTokenInvalid
	}
	if claims.TokenUse != "access" || claims.ClientID != verifier.clientID ||
		claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return Claims{}, ErrTokenInvalid
	}
	sessionID := claims.OriginJTI
	if sessionID == "" {
		sessionID = claims.ID
	}
	normalized := Claims{
		Subject: claims.Subject, Email: claims.Email, Username: claims.Username,
		TokenID: claims.ID, SessionID: sessionID, IssuedAt: claims.IssuedAt.Time, ExpiresAt: claims.ExpiresAt.Time,
	}
	if err := validateClaims(normalized, verifier.clock.Now()); err != nil {
		return Claims{}, err
	}
	return normalized, nil
}

func (verifier *CognitoVerifier) publicKey(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	verifier.mutex.RLock()
	key := verifier.keys[keyID]
	fresh := verifier.clock.Now().Before(verifier.keysExpireAt)
	verifier.mutex.RUnlock()
	if key != nil && fresh {
		return key, nil
	}
	if err := verifier.refreshKeys(ctx); err != nil {
		return nil, err
	}
	verifier.mutex.RLock()
	key = verifier.keys[keyID]
	verifier.mutex.RUnlock()
	if key == nil {
		return nil, ErrTokenInvalid
	}
	return key, nil
}

func (verifier *CognitoVerifier) refreshKeys(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, verifier.jwksURL, nil)
	if err != nil {
		return ErrIdentityUnavailable
	}
	response, err := verifier.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: fetch Cognito JWKS", ErrIdentityUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: Cognito JWKS status %d", ErrIdentityUnavailable, response.StatusCode)
	}
	limited := io.LimitReader(response.Body, maximumJWKSBytes+1)
	encoded, err := io.ReadAll(limited)
	if err != nil || len(encoded) > maximumJWKSBytes {
		return fmt.Errorf("%w: read Cognito JWKS", ErrIdentityUnavailable)
	}
	var document jwksDocument
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil || len(document.Keys) == 0 || len(document.Keys) > maximumJWKSKeys {
		return fmt.Errorf("%w: decode Cognito JWKS: %v", ErrIdentityUnavailable, err)
	}
	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, jwk := range document.Keys {
		key, err := parseRSAJWK(jwk)
		if err != nil {
			return fmt.Errorf("%w: invalid Cognito JWK", ErrIdentityUnavailable)
		}
		if _, duplicate := keys[jwk.KeyID]; duplicate {
			return fmt.Errorf("%w: duplicate Cognito JWK", ErrIdentityUnavailable)
		}
		keys[jwk.KeyID] = key
	}
	verifier.mutex.Lock()
	verifier.keys = keys
	verifier.keysExpireAt = verifier.clock.Now().Add(defaultJWKSCacheTTL)
	verifier.mutex.Unlock()
	return nil
}

func parseRSAJWK(jwk rsaJWK) (*rsa.PublicKey, error) {
	if !validIdentityValue(jwk.KeyID) || jwk.KeyType != "RSA" || jwk.Algorithm != "RS256" || jwk.Use != "sig" {
		return nil, ErrTokenInvalid
	}
	modulusBytes, err := base64.RawURLEncoding.DecodeString(jwk.Modulus)
	if err != nil || len(modulusBytes) < 256 {
		return nil, ErrTokenInvalid
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(jwk.Exponent)
	if err != nil || len(exponentBytes) == 0 || len(exponentBytes) > 4 {
		return nil, ErrTokenInvalid
	}
	exponent := new(big.Int).SetBytes(exponentBytes)
	if !exponent.IsInt64() || exponent.Int64() < 3 {
		return nil, ErrTokenInvalid
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulusBytes), E: int(exponent.Int64())}, nil
}

func isLoopbackHTTP(candidate *url.URL) bool {
	if candidate.Scheme != "http" {
		return false
	}
	host := candidate.Hostname()
	return host == "localhost" || (net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
}
