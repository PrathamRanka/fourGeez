package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/storefront"
)

type StorefrontPublicationRepository struct {
	mutex  sync.RWMutex
	states map[domain.ID]storefront.PublicationState
}

func NewStorefrontPublicationRepository() *StorefrontPublicationRepository {
	return &StorefrontPublicationRepository{states: make(map[domain.ID]storefront.PublicationState)}
}

func (repository *StorefrontPublicationRepository) Get(_ context.Context, sellerID domain.ID) (storefront.PublicationState, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	state, exists := repository.states[sellerID]
	if !exists {
		return storefront.PublicationState{}, persistence.ErrNotFound
	}
	return state, nil
}

func (repository *StorefrontPublicationRepository) Put(_ context.Context, state storefront.PublicationState, expectedVersion uint64) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.states[state.SellerID]
	if !exists {
		if expectedVersion != 0 || state.Version != 1 {
			return persistence.ErrConditionFailed
		}
	} else if stored.Version != expectedVersion || state.Version != expectedVersion+1 || state.PublicationRevision <= stored.PublicationRevision {
		return persistence.ErrConditionFailed
	}
	repository.states[state.SellerID] = state
	return nil
}
