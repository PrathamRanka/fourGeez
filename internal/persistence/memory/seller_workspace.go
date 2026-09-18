package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/sellerworkspace"
)

type SellerWorkspaceRepository struct {
	mutex  sync.RWMutex
	states map[domain.ID]sellerworkspace.WorkspaceState
}

func NewSellerWorkspaceRepository() *SellerWorkspaceRepository {
	return &SellerWorkspaceRepository{states: make(map[domain.ID]sellerworkspace.WorkspaceState)}
}

func (repository *SellerWorkspaceRepository) Get(_ context.Context, sellerID domain.ID) (sellerworkspace.WorkspaceState, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	state, exists := repository.states[sellerID]
	if !exists {
		return sellerworkspace.WorkspaceState{}, persistence.ErrNotFound
	}
	return cloneSellerWorkspaceState(state), nil
}

func (repository *SellerWorkspaceRepository) Put(_ context.Context, state sellerworkspace.WorkspaceState, expectedVersion uint64) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.states[state.SellerID]
	if !exists {
		if expectedVersion != 0 || state.Version != 1 {
			return persistence.ErrConditionFailed
		}
	} else if stored.Version != expectedVersion || state.Version != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	repository.states[state.SellerID] = cloneSellerWorkspaceState(state)
	return nil
}

func cloneSellerWorkspaceState(state sellerworkspace.WorkspaceState) sellerworkspace.WorkspaceState {
	if state.ConnectorVerifiedAt != nil {
		value := *state.ConnectorVerifiedAt
		state.ConnectorVerifiedAt = &value
	}
	if state.SandboxPurchaseTransactionID != nil {
		value := *state.SandboxPurchaseTransactionID
		state.SandboxPurchaseTransactionID = &value
	}
	if state.StorefrontPreviewedAt != nil {
		value := *state.StorefrontPreviewedAt
		state.StorefrontPreviewedAt = &value
	}
	return state
}
