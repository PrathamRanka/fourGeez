package notifications

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

// TestDeliveryServiceQueuesEachSubscriptionOnce verifies idempotent event fan-out.
func TestDeliveryServiceQueuesEachSubscriptionOnce(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	sellerID := mustNotificationID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	subscription, err := NewSubscription(validSubscriptionParams(t))
	if err != nil {
		t.Fatal(err)
	}
	subscriptions := newNotificationRepository()
	subscriptions.subscriptions[subscription.SubscriptionID()] = subscription
	deliveries := newDeliveryRepository()
	service := NewDeliveryService(
		deliveries,
		subscriptions,
		notificationSellerAuthorizer{sellerID: sellerID},
		domain.NewULIDGenerator(
			domain.FixedClock{Value: createdAt.Time()},
			strings.NewReader(strings.Repeat("d", 128)),
		),
		&deliverySigner{},
		&deliverySender{},
		domain.FixedClock{Value: createdAt.Time()},
	)
	quota := &notificationQuotaEnforcer{}
	service.SetQuotaEnforcer(quota)
	event := validDeliveryParams(t, createdAt).Event

	first, err := service.Enqueue(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Enqueue(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 ||
		first[0].DeliveryID != second[0].DeliveryID ||
		len(deliveries.deliveries) != 1 {
		t.Fatalf("enqueue results = (%#v, %#v), stored = %d", first, second, len(deliveries.deliveries))
	}
	if len(quota.deliverySources) != 2 ||
		quota.deliverySources[0] != subscription.SubscriptionID().String()+":"+event.EventID.String() {
		t.Fatalf("delivery quota sources = %#v", quota.deliverySources)
	}
}

// TestDeliveryServiceAttemptsAndPersistsRetry verifies signed retryable delivery.
func TestDeliveryServiceAttemptsAndPersistsRetry(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	params := validDeliveryParams(t, createdAt)
	delivery, err := NewDelivery(params)
	if err != nil {
		t.Fatal(err)
	}
	subscription, err := NewSubscription(validSubscriptionParams(t))
	if err != nil {
		t.Fatal(err)
	}
	deliveries := newDeliveryRepository()
	deliveries.deliveries[delivery.DeliveryID()] = delivery
	deliveries.identities[deliveryIdentity(delivery.SubscriptionID(), delivery.Event().EventID)] = delivery.DeliveryID()
	subscriptions := newNotificationRepository()
	subscriptions.subscriptions[subscription.SubscriptionID()] = subscription
	sender := &deliverySender{
		result: WebhookSendResult{
			StatusCode:       http.StatusTooManyRequests,
			ResponseBodyHash: strings.Repeat("d", 64),
		},
	}
	service := NewDeliveryService(
		deliveries,
		subscriptions,
		notificationSellerAuthorizer{sellerID: params.SellerID},
		domain.NewULIDGenerator(nil, nil),
		&deliverySigner{},
		sender,
		domain.FixedClock{Value: createdAt.Time()},
	)

	view, err := service.Attempt(
		context.Background(),
		params.SellerID,
		params.DeliveryID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 || view.Status != DeliveryStatusRetryScheduled || view.AttemptCount != 1 {
		t.Fatalf("attempt result = %#v, sender calls = %d", view, sender.calls)
	}
}

// TestDeliveryServiceDoesNotSendBeforeRetryIsDue verifies side effects follow policy time.
func TestDeliveryServiceDoesNotSendBeforeRetryIsDue(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	params := validDeliveryParams(t, createdAt)
	delivery, err := NewDelivery(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := delivery.RecordFailure(
		createdAt,
		DeliveryFailureUnavailable,
		0,
		"",
	); err != nil {
		t.Fatal(err)
	}
	subscription, err := NewSubscription(validSubscriptionParams(t))
	if err != nil {
		t.Fatal(err)
	}
	deliveries := newDeliveryRepository()
	deliveries.deliveries[delivery.DeliveryID()] = delivery
	subscriptions := newNotificationRepository()
	subscriptions.subscriptions[subscription.SubscriptionID()] = subscription
	sender := &deliverySender{}
	service := NewDeliveryService(
		deliveries,
		subscriptions,
		notificationSellerAuthorizer{sellerID: params.SellerID},
		domain.NewULIDGenerator(nil, nil),
		&deliverySigner{},
		sender,
		domain.FixedClock{Value: createdAt.Add(30 * time.Second).Time()},
	)

	if _, err := service.Attempt(
		context.Background(),
		params.SellerID,
		params.DeliveryID,
	); !errors.Is(err, ErrDeliveryConflict) {
		t.Fatalf("Attempt() error = %v, want state conflict", err)
	}
	if sender.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", sender.calls)
	}
}

// TestDeliveryServiceRedeliversOnlySellerOwnedDeadLetters verifies authorization and state.
func TestDeliveryServiceRedeliversOnlySellerOwnedDeadLetters(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	params := validDeliveryParams(t, createdAt)
	delivery, err := NewDelivery(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := delivery.RecordFailure(
		createdAt,
		DeliveryFailurePermanentResponse,
		http.StatusBadRequest,
		strings.Repeat("e", 64),
	); err != nil {
		t.Fatal(err)
	}
	deliveries := newDeliveryRepository()
	deliveries.deliveries[delivery.DeliveryID()] = delivery
	service := NewDeliveryService(
		deliveries,
		newNotificationRepository(),
		notificationSellerAuthorizer{sellerID: params.SellerID},
		domain.NewULIDGenerator(nil, nil),
		&deliverySigner{},
		&deliverySender{},
		domain.FixedClock{Value: createdAt.Add(time.Hour).Time()},
	)

	view, err := service.Redeliver(
		context.Background(),
		"seller-user",
		params.SellerID,
		params.DeliveryID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if view.EventID != params.Event.EventID || view.Status != DeliveryStatusPending {
		t.Fatalf("redelivery view = %#v", view)
	}
	if _, err := service.Redeliver(
		context.Background(),
		"seller-user",
		params.SellerID,
		params.DeliveryID,
	); !errors.Is(err, ErrDeliveryConflict) {
		t.Fatalf("second Redeliver() error = %v", err)
	}
}

// TestDeliveryRecordsBoundedRetryAndDeadLetterTransitions verifies EVT-002 policy.
func TestDeliveryRecordsBoundedRetryAndDeadLetterTransitions(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	delivery, err := NewDelivery(validDeliveryParams(t, createdAt))
	if err != nil {
		t.Fatal(err)
	}

	expectedDelays := []time.Duration{
		time.Minute,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
	}
	attemptedAt := createdAt
	for _, expectedDelay := range expectedDelays {
		if err := delivery.RecordFailure(
			attemptedAt,
			DeliveryFailureRetryableResponse,
			http.StatusServiceUnavailable,
			strings.Repeat("a", 64),
		); err != nil {
			t.Fatal(err)
		}
		if delivery.Status() != DeliveryStatusRetryScheduled {
			t.Fatalf("status = %q, want retry_scheduled", delivery.Status())
		}
		if got := delivery.NextAttemptAt().Time().Sub(attemptedAt.Time()); got != expectedDelay {
			t.Fatalf("retry delay = %s, want %s", got, expectedDelay)
		}
		attemptedAt = *delivery.NextAttemptAt()
	}

	if err := delivery.RecordFailure(
		attemptedAt,
		DeliveryFailureTimeout,
		0,
		"",
	); err != nil {
		t.Fatal(err)
	}
	if delivery.Status() != DeliveryStatusDeadLetter || delivery.NextAttemptAt() != nil {
		t.Fatalf("terminal delivery = %#v", delivery.Snapshot())
	}
}

// TestDeliveryRedeliveryPreservesIdentity verifies replay-safe seller redelivery.
func TestDeliveryRedeliveryPreservesIdentity(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	delivery, err := NewDelivery(validDeliveryParams(t, createdAt))
	if err != nil {
		t.Fatal(err)
	}
	if err := delivery.RecordFailure(
		createdAt,
		DeliveryFailurePermanentResponse,
		http.StatusBadRequest,
		strings.Repeat("b", 64),
	); err != nil {
		t.Fatal(err)
	}
	originalDeliveryID := delivery.DeliveryID()
	originalEventID := delivery.Event().EventID

	redeliveryAt := createdAt.Add(time.Hour)
	if err := delivery.ScheduleRedelivery(redeliveryAt); err != nil {
		t.Fatal(err)
	}
	if delivery.DeliveryID() != originalDeliveryID || delivery.Event().EventID != originalEventID {
		t.Fatal("redelivery changed delivery or event identity")
	}
	if delivery.Status() != DeliveryStatusPending || delivery.AttemptCount() != 1 {
		t.Fatalf("redelivery snapshot = %#v", delivery.Snapshot())
	}
}

// TestWebhookSenderRejectsUnsafeDestinations verifies delivery-time SSRF controls.
func TestWebhookSenderRejectsUnsafeDestinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		addresses []net.IP
	}{
		{name: "loopback", addresses: []net.IP{net.ParseIP("127.0.0.1")}},
		{name: "private", addresses: []net.IP{net.ParseIP("10.0.0.1")}},
		{name: "link local", addresses: []net.IP{net.ParseIP("169.254.169.254")}},
		{name: "mixed answers", addresses: []net.IP{net.ParseIP("93.184.216.34"), net.ParseIP("10.0.0.1")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sender := NewWebhookSenderWithClient(
				&deliveryResolver{addresses: test.addresses},
				&deliveryHTTPClient{},
			)
			_, err := sender.Send(
				context.Background(),
				"https://seller.example/webhooks/agentpay",
				SignedEvent{Body: []byte(`{"schemaVersion":"1"}`)},
			)
			if err != ErrWebhookForbiddenTarget {
				t.Fatalf("Send() error = %v, want %v", err, ErrWebhookForbiddenTarget)
			}
		})
	}
}

// validDeliveryParams returns a complete delivery fixture.
func validDeliveryParams(t *testing.T, createdAt domain.Timestamp) DeliveryParams {
	t.Helper()
	return DeliveryParams{
		DeliveryID:     mustNotificationID(t, "whd_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.WebhookDeliveryIDPrefix),
		SellerID:       mustNotificationID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		SubscriptionID: mustNotificationID(t, "whk_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.WebhookSubscriptionIDPrefix),
		Event: WebhookEvent{
			SchemaVersion: "1",
			EventID:       mustNotificationID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.EvidenceIDPrefix),
			SellerID:      mustNotificationID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
			EventType:     EventPaymentVerified,
			OccurredAt:    createdAt,
			Payload:       map[string]any{"transactionId": "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7"},
		},
		PayloadHash: strings.Repeat("c", 64),
		CreatedAt:   createdAt,
	}
}

type deliveryResolver struct {
	addresses []net.IP
}

// LookupIP returns deterministic addresses for delivery security tests.
func (resolver *deliveryResolver) LookupIP(
	_ context.Context,
	_ string,
	_ string,
) ([]net.IP, error) {
	return resolver.addresses, nil
}

type deliveryHTTPClient struct{}

// Do returns a successful empty webhook response.
func (client *deliveryHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusNoContent,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

type deliveryRepository struct {
	deliveries map[domain.ID]Delivery
	identities map[string]domain.ID
}

// newDeliveryRepository creates an empty delivery repository fixture.
func newDeliveryRepository() *deliveryRepository {
	return &deliveryRepository{
		deliveries: make(map[domain.ID]Delivery),
		identities: make(map[string]domain.ID),
	}
}

// CreateIfAbsent stores at most one delivery for one subscription event.
func (repository *deliveryRepository) CreateIfAbsent(
	_ context.Context,
	delivery Delivery,
) (Delivery, bool, error) {
	identity := deliveryIdentity(delivery.SubscriptionID(), delivery.Event().EventID)
	if deliveryID, exists := repository.identities[identity]; exists {
		return repository.deliveries[deliveryID], false, nil
	}
	repository.deliveries[delivery.DeliveryID()] = delivery
	repository.identities[identity] = delivery.DeliveryID()
	return delivery, true, nil
}

// Get returns one seller-owned delivery fixture.
func (repository *deliveryRepository) Get(
	_ context.Context,
	sellerID domain.ID,
	deliveryID domain.ID,
) (Delivery, error) {
	delivery, exists := repository.deliveries[deliveryID]
	if !exists || delivery.SellerID() != sellerID {
		return Delivery{}, ErrDeliveryNotFound
	}
	return delivery, nil
}

// Update stores one optimistic delivery transition.
func (repository *deliveryRepository) Update(
	_ context.Context,
	delivery Delivery,
	expectedVersion uint64,
) error {
	stored, exists := repository.deliveries[delivery.DeliveryID()]
	if !exists {
		return ErrDeliveryNotFound
	}
	if stored.Version() != expectedVersion || delivery.Version() != expectedVersion+1 {
		return ErrDeliveryConflict
	}
	repository.deliveries[delivery.DeliveryID()] = delivery
	return nil
}

// ListBySeller returns a bounded seller delivery fixture page.
func (repository *deliveryRepository) ListBySeller(
	_ context.Context,
	sellerID domain.ID,
	_ int,
	_ string,
) ([]Delivery, *string, error) {
	result := make([]Delivery, 0)
	for _, delivery := range repository.deliveries {
		if delivery.SellerID() == sellerID {
			result = append(result, delivery)
		}
	}
	return result, nil, nil
}

type deliverySigner struct{}

// Sign returns deterministic signed event bytes for service tests.
func (signer *deliverySigner) Sign(
	_ context.Context,
	_ string,
	event WebhookEvent,
) (SignedEvent, error) {
	return SignedEvent{
		Body: []byte(`{"schemaVersion":"1"}`),
		Headers: SignatureHeaders{
			EventID:   event.EventID.String(),
			Timestamp: event.OccurredAt.String(),
			Signature: "signature",
		},
	}, nil
}

type deliverySender struct {
	result WebhookSendResult
	err    error
	calls  int
}

// Send records one webhook attempt and returns its configured result.
func (sender *deliverySender) Send(
	_ context.Context,
	_ string,
	_ SignedEvent,
) (WebhookSendResult, error) {
	sender.calls++
	return sender.result, sender.err
}
