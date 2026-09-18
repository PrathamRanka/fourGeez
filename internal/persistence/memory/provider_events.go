package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

type ProviderEventRepository struct {
	mutex  sync.RWMutex
	events map[string]billing.SubscriptionProviderEvent
}

func NewProviderEventRepository() *ProviderEventRepository {
	return &ProviderEventRepository{events: make(map[string]billing.SubscriptionProviderEvent)}
}

func (repository *ProviderEventRepository) Store(
	_ context.Context,
	event billing.SubscriptionProviderEvent,
) (billing.ProviderEventStoreResult, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.events[event.EventID]
	if !exists {
		repository.events[event.EventID] = event
		return billing.ProviderEventStoreResultInserted, nil
	}
	if stored.PayloadHash == event.PayloadHash {
		return billing.ProviderEventStoreResultDuplicate, nil
	}
	stored.ProcessingState = billing.ProviderEventStateQuarantined
	stored.ConflictPayloadHash = event.PayloadHash
	repository.events[event.EventID] = stored
	return billing.ProviderEventStoreResultConflict, nil
}

func (repository *ProviderEventRepository) Get(
	_ context.Context,
	eventID string,
) (billing.SubscriptionProviderEvent, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	event, exists := repository.events[eventID]
	if !exists {
		return billing.SubscriptionProviderEvent{}, billing.ErrProviderEventNotFound
	}
	return event, nil
}

func (repository *ProviderEventRepository) MarkProcessing(
	_ context.Context,
	eventID string,
	_ domain.Timestamp,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	event, exists := repository.events[eventID]
	if !exists {
		return billing.ErrProviderEventNotFound
	}
	if event.ProcessingState == billing.ProviderEventStateQuarantined {
		return billing.ErrProviderEventQuarantined
	}
	if event.ProcessingState != billing.ProviderEventStateReceived && event.ProcessingState != billing.ProviderEventStateFailed {
		return billing.ErrProviderEventAlreadyProcessing
	}
	event.ProcessingState = billing.ProviderEventStateProcessing
	event.AttemptCount++
	repository.events[eventID] = event
	return nil
}

func (repository *ProviderEventRepository) MarkFailed(
	_ context.Context,
	eventID string,
	failedAt domain.Timestamp,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	event, exists := repository.events[eventID]
	if !exists {
		return billing.ErrProviderEventNotFound
	}
	if event.ProcessingState == billing.ProviderEventStateQuarantined {
		return billing.ErrProviderEventQuarantined
	}
	event.ProcessingState = billing.ProviderEventStateFailed
	event.ProcessedAt = &failedAt
	repository.events[eventID] = event
	return nil
}

func (repository *ProviderEventRepository) MarkApplied(
	_ context.Context,
	eventID string,
	sellerID domain.ID,
	entitlementVersion uint64,
	appliedAt domain.Timestamp,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	event, exists := repository.events[eventID]
	if !exists {
		return billing.ErrProviderEventNotFound
	}
	if event.ProcessingState == billing.ProviderEventStateQuarantined {
		return billing.ErrProviderEventQuarantined
	}
	event.SellerID = sellerID
	event.ProcessingState = billing.ProviderEventStateApplied
	event.AppliedEntitlementVersion = entitlementVersion
	event.ProcessedAt = &appliedAt
	repository.events[eventID] = event
	return nil
}
