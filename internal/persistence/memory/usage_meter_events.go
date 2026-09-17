package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

// UsageMeterEventRepository stores immutable usage facts for local use.
type UsageMeterEventRepository struct {
	mutex  sync.RWMutex
	events map[string]billing.UsageMeterEventSnapshot
}

// NewUsageMeterEventRepository creates an empty usage repository.
func NewUsageMeterEventRepository() *UsageMeterEventRepository {
	return &UsageMeterEventRepository{events: make(map[string]billing.UsageMeterEventSnapshot)}
}

// CreateIfAbsent stores one event per meter and source transaction.
func (repository *UsageMeterEventRepository) CreateIfAbsent(
	_ context.Context,
	event billing.UsageMeterEvent,
) (billing.UsageMeterEvent, bool, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	key := string(event.MeterName()) + "\x00" + event.SourceTransactionID().String()
	if snapshot, exists := repository.events[key]; exists {
		stored, err := billing.RestoreUsageMeterEvent(snapshot)
		return stored, false, err
	}
	repository.events[key] = event.Snapshot()
	return event, true, nil
}

// ListBySellerWindow returns bounded immutable events in chronological order.
func (repository *UsageMeterEventRepository) ListBySellerWindow(
	_ context.Context,
	sellerID domain.ID,
	from domain.Timestamp,
	to domain.Timestamp,
	limit int,
) ([]billing.UsageMeterEvent, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	result := make([]billing.UsageMeterEvent, 0)
	for _, snapshot := range repository.events {
		if snapshot.SellerID != sellerID || snapshot.OccurredAt.Before(from) || !snapshot.OccurredAt.Before(to) {
			continue
		}
		event, err := billing.RestoreUsageMeterEvent(snapshot)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	sort.Slice(result, func(leftIndex int, rightIndex int) bool {
		return result[leftIndex].OccurredAt().Before(result[rightIndex].OccurredAt())
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
