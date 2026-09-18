package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	localSignedTokenPrefix       = "apls1"
	localSignedTokenParts        = 7
	maximumLocalSignedTokenBytes = 4096
	minimumLocalSigningKeyBytes  = 32
	maximumLocalSigningKeyBytes  = 4096
	maximumLocalTokenLifetime    = 8 * time.Hour
	maximumUnixTimestampDigits   = 10
)

// LocalAdapter verifies signed local tokens and explicitly registered compatibility tokens.
type LocalAdapter struct {
	mutex      sync.RWMutex
	clock      domain.Clock
	signingKey []byte
	tokens     map[string]Claims
	revoked    map[string]struct{}
}

// NewLocalAdapter creates a local verifier with production-shaped claims.
func NewLocalAdapter(clock domain.Clock) *LocalAdapter {
	return &LocalAdapter{
		clock: clock, tokens: make(map[string]Claims), revoked: make(map[string]struct{}),
	}
}

// NewSignedLocalAdapter creates a local-only verifier for HMAC-authenticated seller tokens.
func NewSignedLocalAdapter(signingKey []byte, clock domain.Clock) (*LocalAdapter, error) {
	if clock == nil || len(signingKey) < minimumLocalSigningKeyBytes || len(signingKey) > maximumLocalSigningKeyBytes {
		return nil, errors.New("local identity signing key must contain 32 to 4096 bytes")
	}
	adapter := NewLocalAdapter(clock)
	adapter.signingKey = append([]byte(nil), signingKey...)
	return adapter, nil
}

// Register stores only a digest of a local bearer token.
func (adapter *LocalAdapter) Register(rawToken string, claims Claims) error {
	if adapter == nil || adapter.clock == nil || rawToken == "" {
		return ErrTokenInvalid
	}
	if err := validateClaims(claims, adapter.clock.Now()); err != nil {
		return err
	}
	digest := localTokenDigest(rawToken)
	adapter.mutex.Lock()
	defer adapter.mutex.Unlock()
	adapter.tokens[digest] = claims
	delete(adapter.revoked, digest)
	return nil
}

// Verify resolves a configured local token without retaining its raw value.
func (adapter *LocalAdapter) Verify(_ context.Context, rawToken string) (Claims, error) {
	if adapter == nil || adapter.clock == nil || rawToken == "" || len(rawToken) > maximumLocalSignedTokenBytes {
		return Claims{}, ErrTokenInvalid
	}
	digest := localTokenDigest(rawToken)
	adapter.mutex.RLock()
	claims, exists := adapter.tokens[digest]
	_, revoked := adapter.revoked[digest]
	signingKey := append([]byte(nil), adapter.signingKey...)
	adapter.mutex.RUnlock()
	if revoked {
		return Claims{}, ErrTokenRevoked
	}
	if strings.HasPrefix(rawToken, localSignedTokenPrefix+".") {
		return verifySignedLocalToken(rawToken, signingKey, adapter.clock.Now())
	}
	if !exists {
		return Claims{}, ErrTokenInvalid
	}
	if err := validateClaims(claims, adapter.clock.Now()); err != nil {
		return Claims{}, err
	}
	return claims, nil
}

// Remove revokes a configured local token without exposing it to logs or storage.
func (adapter *LocalAdapter) Remove(rawToken string) {
	if adapter == nil || rawToken == "" {
		return
	}
	digest := localTokenDigest(rawToken)
	adapter.mutex.Lock()
	defer adapter.mutex.Unlock()
	delete(adapter.tokens, digest)
	adapter.revoked[digest] = struct{}{}
}

func localTokenDigest(rawToken string) string {
	digest := sha256.Sum256([]byte("agentpay.local-seller-token.v1\x00" + rawToken))
	return hex.EncodeToString(digest[:])
}

func verifySignedLocalToken(rawToken string, signingKey []byte, now time.Time) (Claims, error) {
	if len(signingKey) < minimumLocalSigningKeyBytes || len(signingKey) > maximumLocalSigningKeyBytes {
		return Claims{}, ErrTokenInvalid
	}
	parts := strings.Split(rawToken, ".")
	if len(parts) != localSignedTokenParts || parts[0] != localSignedTokenPrefix {
		return Claims{}, ErrTokenInvalid
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[6])
	if err != nil || len(providedSignature) != sha256.Size {
		return Claims{}, ErrTokenInvalid
	}
	unsignedToken := strings.Join(parts[:6], ".")
	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write([]byte(unsignedToken))
	if !hmac.Equal(providedSignature, mac.Sum(nil)) {
		return Claims{}, ErrTokenInvalid
	}

	subject, err := decodeLocalIdentityValue(parts[1])
	if err != nil {
		return Claims{}, err
	}
	tokenID, err := decodeLocalIdentityValue(parts[2])
	if err != nil {
		return Claims{}, err
	}
	sessionID, err := decodeLocalIdentityValue(parts[3])
	if err != nil {
		return Claims{}, err
	}
	issuedAtUnix, err := parseLocalUnixTimestamp(parts[4])
	if err != nil {
		return Claims{}, err
	}
	expiresAtUnix, err := parseLocalUnixTimestamp(parts[5])
	if err != nil {
		return Claims{}, err
	}
	claims := Claims{
		Subject: subject, TokenID: tokenID, SessionID: sessionID,
		IssuedAt: time.Unix(issuedAtUnix, 0).UTC(), ExpiresAt: time.Unix(expiresAtUnix, 0).UTC(),
	}
	if err := validateClaims(claims, now); err != nil {
		return Claims{}, err
	}
	if !claims.ExpiresAt.After(claims.IssuedAt) || claims.ExpiresAt.Sub(claims.IssuedAt) > maximumLocalTokenLifetime {
		return Claims{}, ErrTokenInvalid
	}
	return claims, nil
}

func decodeLocalIdentityValue(encoded string) (string, error) {
	if encoded == "" || len(encoded) > base64.RawURLEncoding.EncodedLen(maximumIdentityValueLength) {
		return "", ErrTokenInvalid
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || !utf8.Valid(decoded) {
		return "", ErrTokenInvalid
	}
	value := string(decoded)
	if !validIdentityValue(value) {
		return "", ErrTokenInvalid
	}
	return value, nil
}

func parseLocalUnixTimestamp(value string) (int64, error) {
	if value == "" || len(value) > maximumUnixTimestampDigits || (len(value) > 1 && value[0] == '0') {
		return 0, ErrTokenInvalid
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != value {
		return 0, ErrTokenInvalid
	}
	return parsed, nil
}
