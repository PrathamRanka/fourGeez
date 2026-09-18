package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/fourgeez/agentpay/internal/domain"
)

// LocalAdapter verifies opaque development tokens against in-process state.
type LocalAdapter struct {
	mutex   sync.RWMutex
	clock   domain.Clock
	tokens  map[string]Claims
	revoked map[string]struct{}
}

// NewLocalAdapter creates a local verifier with production-shaped claims.
func NewLocalAdapter(clock domain.Clock) *LocalAdapter {
	return &LocalAdapter{
		clock: clock, tokens: make(map[string]Claims), revoked: make(map[string]struct{}),
	}
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
	if adapter == nil || adapter.clock == nil || rawToken == "" {
		return Claims{}, ErrTokenInvalid
	}
	digest := localTokenDigest(rawToken)
	adapter.mutex.RLock()
	claims, exists := adapter.tokens[digest]
	_, revoked := adapter.revoked[digest]
	adapter.mutex.RUnlock()
	if revoked {
		return Claims{}, ErrTokenRevoked
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
