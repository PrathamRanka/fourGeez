package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

type ConfirmationGrantRepository struct {
	mutex    sync.RWMutex
	grants   map[domain.ID]authorization.ConfirmationGrant
	bindings map[string]domain.ID
}

func NewConfirmationGrantRepository() *ConfirmationGrantRepository {
	return &ConfirmationGrantRepository{
		grants: make(map[domain.ID]authorization.ConfirmationGrant), bindings: make(map[string]domain.ID),
	}
}

func (repository *ConfirmationGrantRepository) CreateReplacing(_ context.Context, grant authorization.ConfirmationGrant) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.grants[grant.ConfirmationGrantID]; exists {
		return persistence.ErrAlreadyExists
	}
	if previousID, exists := repository.bindings[grant.BindingHash]; exists {
		previous := repository.grants[previousID]
		if previous.ConsumedAt == nil && previous.RevokedAt == nil {
			revokedAt := grant.IssuedAt
			previous.RevokedAt = &revokedAt
			previous.Version++
			repository.grants[previousID] = previous
		}
	}
	repository.grants[grant.ConfirmationGrantID] = cloneConfirmationGrant(grant)
	repository.bindings[grant.BindingHash] = grant.ConfirmationGrantID
	return nil
}

func (repository *ConfirmationGrantRepository) Get(_ context.Context, grantID domain.ID) (authorization.ConfirmationGrant, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	grant, exists := repository.grants[grantID]
	if !exists {
		return authorization.ConfirmationGrant{}, persistence.ErrNotFound
	}
	return cloneConfirmationGrant(grant), nil
}

func (repository *ConfirmationGrantRepository) Consume(_ context.Context, grant authorization.ConfirmationGrant, expectedVersion uint64) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.grants[grant.ConfirmationGrantID]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Version != expectedVersion || grant.Version != expectedVersion+1 ||
		stored.ConsumedAt != nil || stored.RevokedAt != nil ||
		repository.bindings[stored.BindingHash] != stored.ConfirmationGrantID {
		return persistence.ErrConditionFailed
	}
	repository.grants[grant.ConfirmationGrantID] = cloneConfirmationGrant(grant)
	return nil
}

func cloneConfirmationGrant(grant authorization.ConfirmationGrant) authorization.ConfirmationGrant {
	if grant.ConsumedAt != nil {
		consumedAt := *grant.ConsumedAt
		grant.ConsumedAt = &consumedAt
	}
	if grant.RevokedAt != nil {
		revokedAt := *grant.RevokedAt
		grant.RevokedAt = &revokedAt
	}
	return grant
}

var _ authorization.ConfirmationGrantRepository = (*ConfirmationGrantRepository)(nil)
