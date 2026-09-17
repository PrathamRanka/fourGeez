package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// WebhookDeliveryRepository stores webhook deliveries for local use.
type WebhookDeliveryRepository struct {
	mutex      sync.RWMutex
	deliveries map[domain.ID]notifications.DeliverySnapshot
	identities map[string]domain.ID
}

// NewWebhookDeliveryRepository creates an empty delivery repository.
func NewWebhookDeliveryRepository() *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{
		deliveries: make(map[domain.ID]notifications.DeliverySnapshot),
		identities: make(map[string]domain.ID),
	}
}

// CreateIfAbsent stores at most one delivery for a subscription and event.
func (repository *WebhookDeliveryRepository) CreateIfAbsent(
	_ context.Context,
	delivery notifications.Delivery,
) (notifications.Delivery, bool, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	identity := memoryWebhookDeliveryIdentity(
		delivery.SubscriptionID(),
		delivery.Event().EventID,
	)
	if deliveryID, exists := repository.identities[identity]; exists {
		stored, err := notifications.RestoreDelivery(repository.deliveries[deliveryID])
		return stored, false, err
	}
	if _, exists := repository.deliveries[delivery.DeliveryID()]; exists {
		return notifications.Delivery{}, false, persistence.ErrAlreadyExists
	}
	repository.deliveries[delivery.DeliveryID()] = delivery.Snapshot()
	repository.identities[identity] = delivery.DeliveryID()
	return delivery, true, nil
}

// Get returns one delivery only to its owning seller.
func (repository *WebhookDeliveryRepository) Get(
	_ context.Context,
	sellerID domain.ID,
	deliveryID domain.ID,
) (notifications.Delivery, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	snapshot, exists := repository.deliveries[deliveryID]
	if !exists || snapshot.SellerID != sellerID {
		return notifications.Delivery{}, notifications.ErrDeliveryNotFound
	}
	return notifications.RestoreDelivery(snapshot)
}

// Update replaces a delivery only when its stored version matches.
func (repository *WebhookDeliveryRepository) Update(
	_ context.Context,
	delivery notifications.Delivery,
	expectedVersion uint64,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.deliveries[delivery.DeliveryID()]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Version != expectedVersion || delivery.Version() != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	repository.deliveries[delivery.DeliveryID()] = delivery.Snapshot()
	return nil
}

// ListBySeller returns a newest-first bounded delivery page.
func (repository *WebhookDeliveryRepository) ListBySeller(
	_ context.Context,
	sellerID domain.ID,
	limit int,
	cursor string,
) ([]notifications.Delivery, *string, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	matching := make([]notifications.Delivery, 0)
	for _, snapshot := range repository.deliveries {
		if snapshot.SellerID != sellerID {
			continue
		}
		delivery, err := notifications.RestoreDelivery(snapshot)
		if err != nil {
			return nil, nil, err
		}
		matching = append(matching, delivery)
	}
	sort.Slice(matching, func(leftIndex int, rightIndex int) bool {
		left := matching[leftIndex].Snapshot()
		right := matching[rightIndex].Snapshot()
		if left.CreatedAt.String() == right.CreatedAt.String() {
			return left.DeliveryID.String() > right.DeliveryID.String()
		}
		return left.CreatedAt.Time().After(right.CreatedAt.Time())
	})
	start, err := memoryDeliveryPageStart(matching, cursor)
	if err != nil {
		return nil, nil, err
	}
	end := start + limit
	if end > len(matching) {
		end = len(matching)
	}
	page := append([]notifications.Delivery(nil), matching[start:end]...)
	var nextCursor *string
	if end < len(matching) && len(page) > 0 {
		value := page[len(page)-1].DeliveryID().String()
		nextCursor = &value
	}
	return page, nextCursor, nil
}

// memoryDeliveryPageStart resolves a cursor only within the matching seller page.
func memoryDeliveryPageStart(
	deliveries []notifications.Delivery,
	cursor string,
) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	cursorID, err := domain.ParseID(cursor, domain.WebhookDeliveryIDPrefix)
	if err != nil {
		return 0, domain.NewValidationError("cursor", "format", "must identify a webhook delivery")
	}
	for index, delivery := range deliveries {
		if delivery.DeliveryID() == cursorID {
			return index + 1, nil
		}
	}
	return 0, domain.NewValidationError("cursor", "scope", "does not belong to this seller")
}

// memoryWebhookDeliveryIdentity returns the subscription-event uniqueness key.
func memoryWebhookDeliveryIdentity(subscriptionID domain.ID, eventID domain.ID) string {
	return subscriptionID.String() + "\x00" + eventID.String()
}
