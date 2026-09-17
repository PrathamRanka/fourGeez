package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// WebhookSubscriptionRepository stores webhook subscriptions for local use.
type WebhookSubscriptionRepository struct {
	mutex         sync.RWMutex
	subscriptions map[domain.ID]notifications.SubscriptionSnapshot
}

// NewWebhookSubscriptionRepository creates an empty subscription repository.
func NewWebhookSubscriptionRepository() *WebhookSubscriptionRepository {
	return &WebhookSubscriptionRepository{
		subscriptions: make(map[domain.ID]notifications.SubscriptionSnapshot),
	}
}

// Create stores one subscription without replacing an existing identifier.
func (repository *WebhookSubscriptionRepository) Create(
	_ context.Context,
	subscription notifications.Subscription,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.subscriptions[subscription.SubscriptionID()]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.subscriptions[subscription.SubscriptionID()] = subscription.Snapshot()
	return nil
}

// Get returns one subscription only to its owning seller.
func (repository *WebhookSubscriptionRepository) Get(
	_ context.Context,
	sellerID domain.ID,
	subscriptionID domain.ID,
) (notifications.Subscription, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	snapshot, exists := repository.subscriptions[subscriptionID]
	if !exists || snapshot.SellerID != sellerID {
		return notifications.Subscription{}, notifications.ErrSubscriptionNotFound
	}
	return notifications.RestoreSubscription(snapshot)
}

// ListBySeller returns deterministic seller-scoped subscription metadata.
func (repository *WebhookSubscriptionRepository) ListBySeller(
	_ context.Context,
	sellerID domain.ID,
) ([]notifications.Subscription, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	result := make([]notifications.Subscription, 0)
	for _, snapshot := range repository.subscriptions {
		if snapshot.SellerID != sellerID {
			continue
		}
		subscription, err := notifications.RestoreSubscription(snapshot)
		if err != nil {
			return nil, err
		}
		result = append(result, subscription)
	}
	sort.Slice(result, func(leftIndex int, rightIndex int) bool {
		return result[leftIndex].SubscriptionID().String() <
			result[rightIndex].SubscriptionID().String()
	})
	return result, nil
}
