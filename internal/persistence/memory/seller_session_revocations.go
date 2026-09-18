package memory

import (
	"context"
	"sync"
	"time"
)

// SellerSessionRevocationRepository stores local hashed session revocations.
type SellerSessionRevocationRepository struct {
	mutex       sync.Mutex
	revocations map[string]time.Time
}

// NewSellerSessionRevocationRepository creates an empty local revocation store.
func NewSellerSessionRevocationRepository() *SellerSessionRevocationRepository {
	return &SellerSessionRevocationRepository{revocations: make(map[string]time.Time)}
}

// IsRevoked reports whether the digest remains revoked at the supplied time.
func (repository *SellerSessionRevocationRepository) IsRevoked(_ context.Context, digest string, now time.Time) (bool, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	expiresAt, exists := repository.revocations[digest]
	if exists && !expiresAt.After(now) {
		delete(repository.revocations, digest)
		return false, nil
	}
	return exists, nil
}

// Revoke stores the longest known token-family expiry for one digest.
func (repository *SellerSessionRevocationRepository) Revoke(_ context.Context, digest string, expiresAt time.Time) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if current, exists := repository.revocations[digest]; !exists || expiresAt.After(current) {
		repository.revocations[digest] = expiresAt
	}
	return nil
}
