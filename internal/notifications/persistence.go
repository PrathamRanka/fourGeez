package notifications

import "github.com/fourgeez/agentpay/internal/domain"

// SubscriptionSnapshot is the complete persisted subscription representation.
type SubscriptionSnapshot struct {
	SubscriptionID domain.ID          `json:"subscriptionId"`
	SellerID       domain.ID          `json:"sellerId"`
	EndpointURL    string             `json:"endpointUrl"`
	EventTypes     []EventType        `json:"eventTypes"`
	SecretRef      string             `json:"secretRef"`
	Status         SubscriptionStatus `json:"status"`
	CreatedAt      domain.Timestamp   `json:"createdAt"`
	UpdatedAt      domain.Timestamp   `json:"updatedAt"`
	Version        uint64             `json:"version"`
}

// Snapshot returns an isolated persistence representation.
func (subscription Subscription) Snapshot() SubscriptionSnapshot {
	return SubscriptionSnapshot{
		SubscriptionID: subscription.subscriptionID,
		SellerID:       subscription.sellerID,
		EndpointURL:    subscription.endpointURL,
		EventTypes:     subscription.EventTypes(),
		SecretRef:      subscription.secretRef,
		Status:         subscription.status,
		CreatedAt:      subscription.createdAt,
		UpdatedAt:      subscription.updatedAt,
		Version:        subscription.version,
	}
}

// RestoreSubscription recreates a subscription from trusted storage.
func RestoreSubscription(snapshot SubscriptionSnapshot) (Subscription, error) {
	subscription, err := NewSubscription(SubscriptionParams{
		SubscriptionID: snapshot.SubscriptionID,
		SellerID:       snapshot.SellerID,
		EndpointURL:    snapshot.EndpointURL,
		EventTypes:     snapshot.EventTypes,
		SecretRef:      snapshot.SecretRef,
		CreatedAt:      snapshot.CreatedAt,
	})
	if err != nil {
		return Subscription{}, err
	}
	if snapshot.Status != SubscriptionStatusActive &&
		snapshot.Status != SubscriptionStatusDisabled {
		return Subscription{}, domain.NewValidationError(
			"status",
			"persistence",
			"stored subscription status is invalid",
		)
	}
	if snapshot.Version == 0 || snapshot.UpdatedAt.Time().IsZero() {
		return Subscription{}, domain.NewValidationError(
			"version",
			"persistence",
			"stored subscription metadata is invalid",
		)
	}
	subscription.status = snapshot.Status
	subscription.updatedAt = snapshot.UpdatedAt
	subscription.version = snapshot.Version
	return subscription, nil
}
