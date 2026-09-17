package notifications

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestServiceCreatesSellerWebhookSubscription verifies EVT-001 subscription creation.
func TestServiceCreatesSellerWebhookSubscription(t *testing.T) {
	t.Parallel()

	sellerID := mustNotificationID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	repository := newNotificationRepository()
	secretStore := &notificationSecretStore{secrets: make(map[string][]byte)}
	clock := domain.FixedClock{Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)}
	service := NewService(
		repository,
		notificationSellerAuthorizer{sellerID: sellerID},
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("w", 128))),
		fixedWebhookSecretGenerator{secret: strings.Repeat("s", 43)},
		secretStore,
		clock,
		audit.NoopRecorder{},
	)

	created, err := service.Create(
		context.Background(),
		"seller-user",
		sellerID,
		CreateSubscriptionRequest{
			EndpointURL: "https://seller.example/webhooks/agentpay",
			EventTypes: []EventType{
				EventFulfillmentSucceeded,
				EventPaymentVerified,
				EventPaymentVerified,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if created.SigningSecret == "" || created.Status != SubscriptionStatusActive {
		t.Fatalf("created subscription = %#v", created)
	}
	stored, err := repository.Get(context.Background(), sellerID, created.SubscriptionID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SecretRef() == "" || strings.Contains(stored.SecretRef(), created.SigningSecret) {
		t.Fatalf("stored secret reference = %q", stored.SecretRef())
	}
	if len(stored.EventTypes()) != 2 {
		t.Fatalf("stored event types = %#v", stored.EventTypes())
	}
}

// TestNewSubscriptionRejectsUnsafeConfiguration verifies URL and event allowlists.
func TestNewSubscriptionRejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()

	valid := validSubscriptionParams(t)
	tests := []struct {
		name   string
		mutate func(*SubscriptionParams)
	}{
		{name: "http endpoint", mutate: func(params *SubscriptionParams) { params.EndpointURL = "http://seller.example/hook" }},
		{name: "loopback endpoint", mutate: func(params *SubscriptionParams) { params.EndpointURL = "https://127.0.0.1/hook" }},
		{name: "unknown event", mutate: func(params *SubscriptionParams) { params.EventTypes = []EventType{"unknown"} }},
		{name: "missing secret reference", mutate: func(params *SubscriptionParams) { params.SecretRef = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := valid
			test.mutate(&params)
			if _, err := NewSubscription(params); err == nil {
				t.Fatal("NewSubscription() error = nil")
			}
		})
	}
}

// TestHMACEventSignerSignsCanonicalEnvelope verifies signed webhook contracts.
func TestHMACEventSignerSignsCanonicalEnvelope(t *testing.T) {
	t.Parallel()

	secret := []byte(strings.Repeat("s", 32))
	store := &notificationSecretStore{
		secrets: map[string][]byte{"secret/webhook": secret},
	}
	clock := domain.FixedClock{Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)}
	signer := NewHMACEventSigner(store, clock)
	event := WebhookEvent{
		SchemaVersion: "1",
		EventID:       mustNotificationID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.EvidenceIDPrefix),
		SellerID:      mustNotificationID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		EventType:     EventPaymentVerified,
		OccurredAt:    domain.NewTimestamp(clock.Now()),
		Payload: map[string]any{
			"transactionId": "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		},
	}

	signed, err := signer.Sign(context.Background(), "secret/webhook", event)
	if err != nil {
		t.Fatal(err)
	}
	if signed.Headers.Signature == "" || signed.Headers.EventID != event.EventID.String() {
		t.Fatalf("signed event = %#v", signed)
	}
	if !VerifyWebhookSignature(secret, signed.Body, signed.Headers) {
		t.Fatal("VerifyWebhookSignature() rejected a valid event")
	}
	tampered := append([]byte(nil), signed.Body...)
	tampered[len(tampered)-2] ^= 1
	if VerifyWebhookSignature(secret, tampered, signed.Headers) {
		t.Fatal("VerifyWebhookSignature() accepted a modified body")
	}
	decoded, err := base64.StdEncoding.DecodeString(signed.Headers.Signature)
	if err != nil || len(decoded) == 0 || !hmac.Equal(decoded, decoded) {
		t.Fatal("signature is not valid base64 HMAC material")
	}
}

// validSubscriptionParams returns one complete subscription fixture.
func validSubscriptionParams(t *testing.T) SubscriptionParams {
	t.Helper()
	return SubscriptionParams{
		SubscriptionID: mustNotificationID(t, "whk_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.WebhookSubscriptionIDPrefix),
		SellerID:       mustNotificationID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		EndpointURL:    "https://seller.example/webhooks/agentpay",
		EventTypes:     []EventType{EventPaymentVerified},
		SecretRef:      "secret/webhook",
		CreatedAt:      domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)),
	}
}

type notificationRepository struct {
	subscriptions map[domain.ID]Subscription
}

// newNotificationRepository creates an empty subscription repository fixture.
func newNotificationRepository() *notificationRepository {
	return &notificationRepository{subscriptions: make(map[domain.ID]Subscription)}
}

// Create stores one subscription fixture.
func (repository *notificationRepository) Create(
	_ context.Context,
	subscription Subscription,
) error {
	repository.subscriptions[subscription.SubscriptionID()] = subscription
	return nil
}

// Get returns one seller-owned subscription fixture.
func (repository *notificationRepository) Get(
	_ context.Context,
	sellerID domain.ID,
	subscriptionID domain.ID,
) (Subscription, error) {
	subscription, exists := repository.subscriptions[subscriptionID]
	if !exists || subscription.SellerID() != sellerID {
		return Subscription{}, ErrSubscriptionNotFound
	}
	return subscription, nil
}

// ListBySeller returns subscription fixtures owned by one seller.
func (repository *notificationRepository) ListBySeller(
	_ context.Context,
	sellerID domain.ID,
) ([]Subscription, error) {
	result := make([]Subscription, 0)
	for _, subscription := range repository.subscriptions {
		if subscription.SellerID() == sellerID {
			result = append(result, subscription)
		}
	}
	return result, nil
}

type notificationSellerAuthorizer struct {
	sellerID domain.ID
}

// AuthorizeSeller verifies the expected seller fixture.
func (authorizer notificationSellerAuthorizer) AuthorizeSeller(
	_ context.Context,
	ownerSubject string,
	sellerID domain.ID,
) error {
	if ownerSubject != "seller-user" || sellerID != authorizer.sellerID {
		return ErrSubscriptionNotFound
	}
	return nil
}

type fixedWebhookSecretGenerator struct {
	secret string
}

// NewSecret returns deterministic test-only webhook secret material.
func (generator fixedWebhookSecretGenerator) NewSecret() (string, error) {
	return generator.secret, nil
}

type notificationSecretStore struct {
	secrets map[string][]byte
}

// PutSecret stores one secret under its opaque reference.
func (store *notificationSecretStore) PutSecret(
	_ context.Context,
	reference string,
	secret []byte,
) error {
	store.secrets[reference] = append([]byte(nil), secret...)
	return nil
}

// GetSecret returns a copied secret for signing tests.
func (store *notificationSecretStore) GetSecret(
	_ context.Context,
	reference string,
) ([]byte, error) {
	return append([]byte(nil), store.secrets[reference]...), nil
}

// mustNotificationID parses one stable notification fixture identifier.
func mustNotificationID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
