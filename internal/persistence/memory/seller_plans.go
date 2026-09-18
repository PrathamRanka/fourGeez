package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

type SellerEntitlementRepository struct {
	mutex           sync.RWMutex
	entitlements    map[domain.ID]billing.SellerEntitlementSnapshot
	reconciliations map[domain.ID]map[string]billing.EntitlementReconciliation
}

type SellerPlanRepository = SellerEntitlementRepository

func NewSellerEntitlementRepository() *SellerEntitlementRepository {
	return &SellerEntitlementRepository{
		entitlements:    make(map[domain.ID]billing.SellerEntitlementSnapshot),
		reconciliations: make(map[domain.ID]map[string]billing.EntitlementReconciliation),
	}
}

func NewSellerPlanRepository() *SellerPlanRepository { return NewSellerEntitlementRepository() }

func (repository *SellerEntitlementRepository) Get(_ context.Context, sellerID domain.ID) (billing.SellerEntitlement, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	snapshot, exists := repository.entitlements[sellerID]
	if !exists {
		return billing.SellerEntitlement{}, persistence.ErrNotFound
	}
	return billing.RestoreSellerEntitlement(snapshot)
}

func (repository *SellerEntitlementRepository) Apply(
	_ context.Context,
	entitlement billing.SellerEntitlement,
	reconciliation billing.EntitlementReconciliation,
	expectedVersion uint64,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.entitlements[entitlement.SellerID()]
	if (!exists && expectedVersion != 0) ||
		(exists && stored.Version != expectedVersion) ||
		entitlement.Version() != expectedVersion+1 ||
		reconciliation.SellerID != entitlement.SellerID() ||
		reconciliation.SourceRevision != entitlement.SourceRevision() ||
		reconciliation.AppliedVersion != entitlement.Version() {
		return persistence.ErrConditionFailed
	}
	if exists && entitlement.SourceRevision() <= stored.SourceRevision {
		return persistence.ErrConditionFailed
	}
	if repository.reconciliations[entitlement.SellerID()] == nil {
		repository.reconciliations[entitlement.SellerID()] = make(map[string]billing.EntitlementReconciliation)
	}
	if _, duplicate := repository.reconciliations[entitlement.SellerID()][reconciliation.SourceRevision]; duplicate {
		return persistence.ErrConditionFailed
	}
	repository.entitlements[entitlement.SellerID()] = entitlement.Snapshot()
	repository.reconciliations[entitlement.SellerID()][reconciliation.SourceRevision] = reconciliation
	return nil
}
