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
	"github.com/fourgeez/agentpay/internal/persistence"
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
	quota := &notificationQuotaEnforcer{}
	service.SetQuotaEnforcer(quota)

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
	if quota.subscriptionCount != 0 {
		t.Fatalf("subscription count = %d, want 0", quota.subscriptionCount)
	}
}

// TestServiceDisablesReplacedWebhookSubscription verifies overlap-based secret rotation.
func TestServiceDisablesReplacedWebhookSubscription(t *testing.T) {
	t.Parallel()

	sellerID := mustNotificationID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	repository := newNotificationRepository()
	clock := domain.FixedClock{Value: time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)}
	recorder := &notificationAuditRecorder{}
	service := NewService(
		repository,
		notificationSellerAuthorizer{sellerID: sellerID},
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("w", 256))),
		&sequenceWebhookSecretGenerator{secrets: []string{strings.Repeat("s", 43), strings.Repeat("r", 43)}},
		&notificationSecretStore{secrets: make(map[string][]byte)},
		clock,
		recorder,
	)
	predecessor, err := service.Create(t.Context(), "seller-user", sellerID, CreateSubscriptionRequest{
		EndpointURL: "https://seller.example/webhooks/agentpay",
		EventTypes:  []EventType{EventPaymentVerified},
	})
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := service.Create(t.Context(), "seller-user", sellerID, CreateSubscriptionRequest{
		EndpointURL: "https://seller.example/webhooks/agentpay-rotated",
		EventTypes:  []EventType{EventPaymentVerified},
	})
	if err != nil {
		t.Fatal(err)
	}
	if replacement.SigningSecret == predecessor.SigningSecret {
		t.Fatal("replacement reused the predecessor signing secret")
	}

	disabled, err := service.Disable(
		t.Context(),
		"seller-user",
		sellerID,
		predecessor.SubscriptionID,
		DisableSubscriptionRequest{ExpectedVersion: predecessor.Version},
	)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Status != SubscriptionStatusDisabled || disabled.Version != predecessor.Version+1 {
		t.Fatalf("disabled subscription = %#v", disabled)
	}
	if _, err := service.Disable(
		t.Context(),
		"seller-user",
		sellerID,
		predecessor.SubscriptionID,
		DisableSubscriptionRequest{ExpectedVersion: predecessor.Version},
	); err == nil {
		t.Fatal("Disable() stale version error = nil")
	}
	if got := recorder.requests[len(recorder.requests)-1]; got.Action != audit.ActionWebhookSubscriptionDisabled ||
		len(got.ChangedFields) != 1 || got.ChangedFields[0] != "status" {
		t.Fatalf("disable audit = %#v", got)
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

// Update replaces one subscription when its stored version matches.
func (repository *notificationRepository) Update(
	_ context.Context,
	subscription Subscription,
	expectedVersion uint64,
) error {
	stored, exists := repository.subscriptions[subscription.SubscriptionID()]
	if !exists || stored.Version() != expectedVersion {
		return persistence.ErrConditionFailed
	}
	repository.subscriptions[subscription.SubscriptionID()] = subscription
	return nil
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

type sequenceWebhookSecretGenerator struct {
	secrets []string
}

func (generator *sequenceWebhookSecretGenerator) NewSecret() (string, error) {
	secret := generator.secrets[0]
	generator.secrets = generator.secrets[1:]
	return secret, nil
}

// NewSecret returns deterministic test-only webhook secret material.
func (generator fixedWebhookSecretGenerator) NewSecret() (string, error) {
	return generator.secret, nil
}

type notificationSecretStore struct {
	secrets map[string][]byte
}

type notificationQuotaEnforcer struct {
	subscriptionCount uint64
	deliverySources   []string
}

// ConsumeAPIRequest permits notification API requests.
func (enforcer *notificationQuotaEnforcer) ConsumeAPIRequest(context.Context, domain.ID) error {
	return nil
}

// ConsumeMCPOperation permits notification MCP operations.
func (enforcer *notificationQuotaEnforcer) ConsumeMCPOperation(context.Context, domain.ID) error {
	return nil
}

// ConsumeWebhookDelivery records a logical webhook source.
func (enforcer *notificationQuotaEnforcer) ConsumeWebhookDelivery(
	_ context.Context,
	_ domain.ID,
	source string,
) error {
	enforcer.deliverySources = append(enforcer.deliverySources, source)
	return nil
}

// AllowPublishedRoute permits notification route publication.
func (enforcer *notificationQuotaEnforcer) AllowPublishedRoute(context.Context, domain.ID, uint64) error {
	return nil
}

// AllowWebhookSubscription records the current subscription count.
func (enforcer *notificationQuotaEnforcer) AllowWebhookSubscription(
	_ context.Context,
	_ domain.ID,
	count uint64,
) error {
	enforcer.subscriptionCount = count
	return nil
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
