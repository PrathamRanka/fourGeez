package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// SellerPlanRepository stores seller plan assignments for local use.
type SellerPlanRepository struct {
	mutex       sync.RWMutex
	assignments map[domain.ID]billing.SellerPlanSnapshot
}

// NewSellerPlanRepository creates an empty seller plan repository.
func NewSellerPlanRepository() *SellerPlanRepository {
	return &SellerPlanRepository{
		assignments: make(map[domain.ID]billing.SellerPlanSnapshot),
	}
}

// Get returns one seller plan assignment.
func (repository *SellerPlanRepository) Get(
	_ context.Context,
	sellerID domain.ID,
) (billing.SellerPlan, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	snapshot, exists := repository.assignments[sellerID]
	if !exists {
		return billing.SellerPlan{}, billing.ErrSellerPlanNotFound
	}
	return billing.RestoreSellerPlan(snapshot)
}

// CreateIfAbsent stores one default assignment without replacing existing state.
func (repository *SellerPlanRepository) CreateIfAbsent(
	_ context.Context,
	assignment billing.SellerPlan,
) (billing.SellerPlan, bool, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if snapshot, exists := repository.assignments[assignment.SellerID()]; exists {
		stored, err := billing.RestoreSellerPlan(snapshot)
		return stored, false, err
	}
	repository.assignments[assignment.SellerID()] = assignment.Snapshot()
	return assignment, true, nil
}

// Update replaces an assignment only when its stored version matches.
func (repository *SellerPlanRepository) Update(
	_ context.Context,
	assignment billing.SellerPlan,
	expectedVersion uint64,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.assignments[assignment.SellerID()]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Version != expectedVersion || assignment.Version() != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	repository.assignments[assignment.SellerID()] = assignment.Snapshot()
	return nil
}
