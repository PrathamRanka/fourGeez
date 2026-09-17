package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// TestWebhookDeliveryRepositoryIsIdempotentAndConditional verifies delivery safety.
func TestWebhookDeliveryRepositoryIsIdempotentAndConditional(t *testing.T) {
	t.Parallel()

	repository := NewWebhookDeliveryRepository()
	delivery := testMemoryWebhookDelivery(t)
	created, inserted, err := repository.CreateIfAbsent(context.Background(), delivery)
	if err != nil || !inserted || created.DeliveryID() != delivery.DeliveryID() {
		t.Fatalf("CreateIfAbsent() = (%#v, %v, %v)", created, inserted, err)
	}
	duplicate, inserted, err := repository.CreateIfAbsent(context.Background(), delivery)
	if err != nil || inserted || duplicate.DeliveryID() != delivery.DeliveryID() {
		t.Fatalf("duplicate CreateIfAbsent() = (%#v, %v, %v)", duplicate, inserted, err)
	}

	expectedVersion := delivery.Version()
	if err := delivery.ScheduleRedelivery(delivery.Snapshot().CreatedAt); err == nil {
		t.Fatal("ScheduleRedelivery() unexpectedly accepted a pending delivery")
	}
	if err := repository.Update(context.Background(), delivery, expectedVersion); err != persistence.ErrConditionFailed {
		t.Fatalf("Update() error = %v, want condition failure", err)
	}
}

// testMemoryWebhookDelivery creates one valid delivery fixture.
func testMemoryWebhookDelivery(t *testing.T) notifications.Delivery {
	t.Helper()
	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	delivery, err := notifications.NewDelivery(notifications.DeliveryParams{
		DeliveryID:     mustMemoryID(t, "whd_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.WebhookDeliveryIDPrefix),
		SellerID:       mustMemoryID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		SubscriptionID: mustMemoryID(t, "whk_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.WebhookSubscriptionIDPrefix),
		Event: notifications.WebhookEvent{
			SchemaVersion: "1",
			EventID:       mustMemoryID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.EvidenceIDPrefix),
			SellerID:      mustMemoryID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
			EventType:     notifications.EventPaymentVerified,
			OccurredAt:    createdAt,
			Payload:       map[string]any{"transactionId": "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7"},
		},
		PayloadHash: strings.Repeat("a", 64),
		CreatedAt:   createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return delivery
}
