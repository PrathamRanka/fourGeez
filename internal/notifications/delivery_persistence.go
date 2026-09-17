package notifications

import "github.com/fourgeez/agentpay/internal/domain"

// DeliverySnapshot is the complete persisted webhook delivery representation.
type DeliverySnapshot struct {
	DeliveryID         domain.ID            `json:"deliveryId"`
	SellerID           domain.ID            `json:"sellerId"`
	SubscriptionID     domain.ID            `json:"subscriptionId"`
	Event              WebhookEvent         `json:"event"`
	PayloadHash        string               `json:"payloadHash"`
	Status             DeliveryStatus       `json:"status"`
	AttemptCount       uint32               `json:"attemptCount"`
	NextAttemptAt      *domain.Timestamp    `json:"nextAttemptAt"`
	LastAttemptAt      *domain.Timestamp    `json:"lastAttemptAt"`
	DeliveredAt        *domain.Timestamp    `json:"deliveredAt"`
	ResponseStatusCode *int                 `json:"responseStatusCode"`
	ResponseBodyHash   *string              `json:"responseBodyHash"`
	ErrorCode          *DeliveryFailureCode `json:"errorCode"`
	CreatedAt          domain.Timestamp     `json:"createdAt"`
	UpdatedAt          domain.Timestamp     `json:"updatedAt"`
	Version            uint64               `json:"version"`
}

// Snapshot returns an isolated persistence representation.
func (delivery Delivery) Snapshot() DeliverySnapshot {
	return DeliverySnapshot{
		DeliveryID:         delivery.deliveryID,
		SellerID:           delivery.sellerID,
		SubscriptionID:     delivery.subscriptionID,
		Event:              delivery.event,
		PayloadHash:        delivery.payloadHash,
		Status:             delivery.status,
		AttemptCount:       delivery.attemptCount,
		NextAttemptAt:      copyTimestampPointer(delivery.nextAttemptAt),
		LastAttemptAt:      copyTimestampPointer(delivery.lastAttemptAt),
		DeliveredAt:        copyTimestampPointer(delivery.deliveredAt),
		ResponseStatusCode: copyIntPointer(delivery.responseStatusCode),
		ResponseBodyHash:   copyStringPointer(delivery.responseBodyHash),
		ErrorCode:          copyFailureCodePointer(delivery.errorCode),
		CreatedAt:          delivery.createdAt,
		UpdatedAt:          delivery.updatedAt,
		Version:            delivery.version,
	}
}

// RestoreDelivery recreates a webhook delivery from trusted storage.
func RestoreDelivery(snapshot DeliverySnapshot) (Delivery, error) {
	delivery, err := NewDelivery(DeliveryParams{
		DeliveryID:     snapshot.DeliveryID,
		SellerID:       snapshot.SellerID,
		SubscriptionID: snapshot.SubscriptionID,
		Event:          snapshot.Event,
		PayloadHash:    snapshot.PayloadHash,
		CreatedAt:      snapshot.CreatedAt,
	})
	if err != nil {
		return Delivery{}, err
	}
	if !validDeliveryStatus(snapshot.Status) ||
		snapshot.AttemptCount > maximumAutomaticDeliveryAttempts && snapshot.Status != DeliveryStatusPending ||
		snapshot.Version == 0 ||
		snapshot.UpdatedAt.Time().IsZero() {
		return Delivery{}, domain.NewValidationError(
			"delivery",
			"persistence",
			"stored webhook delivery is invalid",
		)
	}
	delivery.status = snapshot.Status
	delivery.attemptCount = snapshot.AttemptCount
	delivery.nextAttemptAt = copyTimestampPointer(snapshot.NextAttemptAt)
	delivery.lastAttemptAt = copyTimestampPointer(snapshot.LastAttemptAt)
	delivery.deliveredAt = copyTimestampPointer(snapshot.DeliveredAt)
	delivery.responseStatusCode = copyIntPointer(snapshot.ResponseStatusCode)
	delivery.responseBodyHash = copyStringPointer(snapshot.ResponseBodyHash)
	delivery.errorCode = copyFailureCodePointer(snapshot.ErrorCode)
	delivery.updatedAt = snapshot.UpdatedAt
	delivery.version = snapshot.Version
	return delivery, nil
}

// deliveryView redacts event payload while exposing delivery history metadata.
func deliveryView(delivery Delivery) DeliveryView {
	snapshot := delivery.Snapshot()
	return DeliveryView{
		DeliveryID:         snapshot.DeliveryID,
		SellerID:           snapshot.SellerID,
		SubscriptionID:     snapshot.SubscriptionID,
		EventID:            snapshot.Event.EventID,
		EventType:          snapshot.Event.EventType,
		PayloadHash:        snapshot.PayloadHash,
		Status:             snapshot.Status,
		AttemptCount:       snapshot.AttemptCount,
		NextAttemptAt:      snapshot.NextAttemptAt,
		LastAttemptAt:      snapshot.LastAttemptAt,
		DeliveredAt:        snapshot.DeliveredAt,
		ResponseStatusCode: snapshot.ResponseStatusCode,
		ResponseBodyHash:   snapshot.ResponseBodyHash,
		ErrorCode:          snapshot.ErrorCode,
		CreatedAt:          snapshot.CreatedAt,
		UpdatedAt:          snapshot.UpdatedAt,
		Version:            snapshot.Version,
	}
}

// validDeliveryStatus reports whether a persisted status is supported.
func validDeliveryStatus(status DeliveryStatus) bool {
	switch status {
	case DeliveryStatusPending,
		DeliveryStatusRetryScheduled,
		DeliveryStatusDelivered,
		DeliveryStatusDeadLetter:
		return true
	default:
		return false
	}
}

// copyIntPointer copies optional HTTP status metadata.
func copyIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	return intPointer(*value)
}

// copyStringPointer copies optional hash metadata.
func copyStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// copyFailureCodePointer copies an optional failure code.
func copyFailureCodePointer(value *DeliveryFailureCode) *DeliveryFailureCode {
	if value == nil {
		return nil
	}
	return failureCodePointer(*value)
}
