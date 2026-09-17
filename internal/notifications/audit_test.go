package notifications

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestServiceAuditsWebhookConfiguration verifies subscription creation history.
func TestServiceAuditsWebhookConfiguration(t *testing.T) {
	t.Parallel()

	sellerID := mustNotificationID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	}
	recorder := &notificationAuditRecorder{}
	service := NewService(
		newNotificationRepository(),
		notificationSellerAuthorizer{sellerID: sellerID},
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("w", 128))),
		fixedWebhookSecretGenerator{secret: strings.Repeat("s", 43)},
		&notificationSecretStore{secrets: make(map[string][]byte)},
		clock,
		recorder,
	)

	_, err := service.Create(
		t.Context(),
		"seller-user",
		sellerID,
		CreateSubscriptionRequest{
			EndpointURL: "https://seller.example/webhooks/agentpay",
			EventTypes: []EventType{
				EventPaymentVerified,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(recorder.requests) != 1 ||
		recorder.requests[0].Action != audit.ActionWebhookSubscriptionCreated {
		t.Fatalf("audit requests = %#v", recorder.requests)
	}
}

type notificationAuditRecorder struct {
	requests []audit.RecordRequest
}

// Record captures one webhook configuration audit request.
func (recorder *notificationAuditRecorder) Record(
	_ context.Context,
	request audit.RecordRequest,
) error {
	recorder.requests = append(recorder.requests, request)
	return nil
}
