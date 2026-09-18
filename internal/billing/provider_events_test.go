package billing

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/stripe/stripe-go/v86/webhook"
)

func TestStripeEventIngestionIsAuthenticatedIdempotentAndReplaySafe(t *testing.T) {
	t.Parallel()

	repository := newProviderEventRepository()
	verifier := &stubStripeEventVerifier{event: VerifiedStripeEvent{
		EventID: "evt_123", EventType: "invoice.paid", Livemode: true,
		AccountID: "acct_123", APIVersion: "2026-08-27.basil", ProviderCreatedAt: entitlementTime(2026, time.September, 18, 9),
		CustomerID: "cus_123", SubscriptionID: "sub_123", InvoiceID: "in_123",
	}}
	service := NewStripeProviderEventService(StripeProviderEventConfig{
		ExpectedLivemode: true, ExpectedAccountID: "acct_123", ExpectedAPIVersion: "2026-08-27.basil",
	}, verifier, repository, nil, nil, domain.FixedClock{Value: entitlementTime(2026, time.September, 18, 10).Time()})

	first, err := service.Ingest(t.Context(), []byte(`{"id":"evt_123","first":true}`), "stripe-signature")
	if err != nil || first.Duplicate {
		t.Fatalf("first Ingest() = (%#v, %v)", first, err)
	}
	stored, err := repository.Get(t.Context(), "evt_123")
	if err != nil {
		t.Fatal(err)
	}
	if stored.PayloadHash == "" || stored.ProcessingState != ProviderEventStateReceived || stored.AttemptCount != 0 {
		t.Fatalf("stored event = %#v", stored)
	}
	second, err := service.Ingest(t.Context(), []byte(`{"id":"evt_123","first":true}`), "stripe-signature")
	if err != nil || !second.Duplicate {
		t.Fatalf("duplicate Ingest() = (%#v, %v)", second, err)
	}
	_, err = service.Ingest(t.Context(), []byte(`{"id":"evt_123","first":false}`), "stripe-signature")
	if !errors.Is(err, ErrProviderEventPayloadConflict) {
		t.Fatalf("conflicting Ingest() error = %v", err)
	}
	quarantined, _ := repository.Get(t.Context(), "evt_123")
	if quarantined.ProcessingState != ProviderEventStateQuarantined || quarantined.ConflictPayloadHash == "" {
		t.Fatalf("quarantined event = %#v", quarantined)
	}
}

func TestStripeEventIngestionRejectsEnvironmentAccountAndAPIVersionMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event VerifiedStripeEvent
	}{
		{name: "livemode", event: VerifiedStripeEvent{EventID: "evt_1", Livemode: false, AccountID: "acct_123", APIVersion: "2026-08-27.basil"}},
		{name: "account", event: VerifiedStripeEvent{EventID: "evt_1", Livemode: true, AccountID: "acct_other", APIVersion: "2026-08-27.basil"}},
		{name: "api version", event: VerifiedStripeEvent{EventID: "evt_1", Livemode: true, AccountID: "acct_123", APIVersion: "2025-01-01"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := NewStripeProviderEventService(StripeProviderEventConfig{ExpectedLivemode: true, ExpectedAccountID: "acct_123", ExpectedAPIVersion: "2026-08-27.basil"}, &stubStripeEventVerifier{event: test.event}, newProviderEventRepository(), nil, nil, domain.SystemClock{})
			if _, err := service.Ingest(t.Context(), []byte(`{"id":"evt_1"}`), "signature"); !errors.Is(err, ErrProviderEventEnvironmentMismatch) {
				t.Fatalf("Ingest() error = %v", err)
			}
		})
	}
}

func TestStripeWebhookVerifierValidatesExactRawBody(t *testing.T) {
	t.Parallel()

	secret := "whsec_test"
	payload := []byte(`{"id":"evt_123","object":"event","api_version":"2026-08-27.basil","created":1789718400,"livemode":false,"type":"invoice.paid","data":{"object":{"id":"in_123","customer":"cus_123","subscription":"sub_123"}}}`)
	timestamp := time.Now().UTC()
	signature := hex.EncodeToString(webhook.ComputeSignature(timestamp, payload, secret))
	header := fmt.Sprintf("t=%d,v1=%s", timestamp.Unix(), signature)
	verifier := NewStripeWebhookVerifier(secret, time.Minute)
	event, err := verifier.Verify(payload, header)
	if err != nil {
		t.Fatal(err)
	}
	if event.EventID != "evt_123" || event.InvoiceID != "in_123" || event.CustomerID != "cus_123" || event.SubscriptionID != "sub_123" {
		t.Fatalf("verified event = %#v", event)
	}
	if _, err := verifier.Verify(append(payload, ' '), header); err == nil {
		t.Fatal("modified raw body passed signature verification")
	}
}

func TestStripeEventReconciliationFetchesCurrentProviderStateAndMarksApplied(t *testing.T) {
	t.Parallel()

	now := entitlementTime(2026, time.September, 18, 10)
	sellerID := mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	eventRepository := newProviderEventRepository()
	eventRepository.events["evt_old"] = SubscriptionProviderEvent{EventID: "evt_old", EventType: "customer.subscription.updated", PayloadHash: "hash", Livemode: true, AccountID: "acct_123", APIVersion: "2026-08-27.basil", ProcessingState: ProviderEventStateReceived, ReceivedAt: now}
	entitlementRepository := newBillingRepository()
	entitlementService := NewService(entitlementRepository, billingSellerAuthorizer{sellerID: sellerID}, domain.FixedClock{Value: now.Time()})
	reader := &stubStripeProviderStateReader{candidate: EntitlementCandidate{
		SellerID: sellerID, PlanID: PlanScale, PlanVersion: 1, Status: EntitlementStatusActive,
		BillingPeriodStart: entitlementTime(2026, time.September, 1, 0), BillingPeriodEnd: entitlementTime(2026, time.October, 1, 0), AccessEndsAt: entitlementTime(2026, time.October, 1, 0),
		Source: EntitlementSourceBillingProvider, Provider: EntitlementProviderStripe,
		ProviderCustomerID: "cus_current", ProviderSubscriptionID: "sub_current", ProviderPriceID: "price_current",
	}}
	service := NewStripeProviderEventService(StripeProviderEventConfig{ExpectedLivemode: true, ExpectedAccountID: "acct_123", ExpectedAPIVersion: "2026-08-27.basil"}, nil, eventRepository, reader, entitlementService, domain.FixedClock{Value: now.Time()})

	response, err := service.Reconcile(t.Context(), "evt_old")
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 || response.Assignment.PlanID != PlanScale || response.Assignment.SourceRevision != "00000000000000000001" {
		t.Fatalf("reconciliation response = %#v, reader calls = %d", response, reader.calls)
	}
	applied, _ := eventRepository.Get(t.Context(), "evt_old")
	if applied.ProcessingState != ProviderEventStateApplied || applied.AppliedEntitlementVersion != 1 || applied.SellerID != sellerID {
		t.Fatalf("applied event = %#v", applied)
	}
}

func TestStripeEventReconciliationFailureRemainsRetryable(t *testing.T) {
	t.Parallel()

	now := entitlementTime(2026, time.September, 18, 10)
	sellerID := mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	events := newProviderEventRepository()
	events.events["evt_retry"] = SubscriptionProviderEvent{EventID: "evt_retry", EventType: "invoice.paid", PayloadHash: "hash", ProcessingState: ProviderEventStateReceived, ReceivedAt: now}
	entitlements := newBillingRepository()
	reader := &stubStripeProviderStateReader{err: errors.New("Stripe unavailable")}
	service := NewStripeProviderEventService(StripeProviderEventConfig{}, nil, events, reader, NewService(entitlements, billingSellerAuthorizer{sellerID: sellerID}, domain.FixedClock{Value: now.Time()}), domain.FixedClock{Value: now.Time()})
	if _, err := service.Reconcile(t.Context(), "evt_retry"); err == nil {
		t.Fatal("Reconcile() succeeded during provider failure")
	}
	failed, _ := events.Get(t.Context(), "evt_retry")
	if failed.ProcessingState != ProviderEventStateFailed || failed.AttemptCount != 1 {
		t.Fatalf("failed event = %#v", failed)
	}
	reader.err = nil
	reader.candidate = EntitlementCandidate{
		SellerID: sellerID, PlanID: PlanStarter, PlanVersion: 1, Status: EntitlementStatusActive,
		BillingPeriodStart: entitlementTime(2026, time.September, 1, 0), BillingPeriodEnd: entitlementTime(2026, time.October, 1, 0), AccessEndsAt: entitlementTime(2026, time.October, 1, 0),
		ProviderCustomerID: "cus_123", ProviderSubscriptionID: "sub_123", ProviderPriceID: "price_123",
	}
	if _, err := service.Reconcile(t.Context(), "evt_retry"); err != nil {
		t.Fatal(err)
	}
}

func TestStripeEventReconciliationDoesNotReapplyAfterCommitWindow(t *testing.T) {
	t.Parallel()

	now := entitlementTime(2026, time.September, 18, 10)
	sellerID := mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7")
	events := newProviderEventRepository()
	events.events["evt_committed"] = SubscriptionProviderEvent{EventID: "evt_committed", EventType: "invoice.paid", PayloadHash: "hash", ProcessingState: ProviderEventStateFailed, ReceivedAt: now}
	entitlements := newBillingRepository()
	service := NewService(entitlements, billingSellerAuthorizer{sellerID: sellerID}, domain.FixedClock{Value: now.Time()})
	seeded, err := service.ReconcileEntitlement(t.Context(), EntitlementCandidate{
		SellerID: sellerID, PlanID: PlanStarter, PlanVersion: 1, Status: EntitlementStatusActive,
		BillingPeriodStart: entitlementTime(2026, time.September, 1, 0), BillingPeriodEnd: entitlementTime(2026, time.October, 1, 0), AccessEndsAt: entitlementTime(2026, time.October, 1, 0),
		ProviderCustomerID: "cus_123", ProviderSubscriptionID: "sub_123", ProviderPriceID: "price_123",
		Source: EntitlementSourceBillingProvider, Provider: EntitlementProviderStripe, LastProviderEventID: "evt_committed",
	})
	if err != nil {
		t.Fatal(err)
	}
	reader := &stubStripeProviderStateReader{candidate: candidateFromEntitlement(entitlements.assignments[sellerID])}
	providerService := NewStripeProviderEventService(StripeProviderEventConfig{}, nil, events, reader, service, domain.FixedClock{Value: now.Time()})
	response, err := providerService.Reconcile(t.Context(), "evt_committed")
	if err != nil {
		t.Fatal(err)
	}
	if response.Assignment.Version != seeded.Assignment.Version || entitlements.applyCalls != 1 {
		t.Fatalf("event reapplied: response version=%d apply calls=%d", response.Assignment.Version, entitlements.applyCalls)
	}
}

type stubStripeEventVerifier struct {
	event VerifiedStripeEvent
	err   error
}

func (verifier *stubStripeEventVerifier) Verify([]byte, string) (VerifiedStripeEvent, error) {
	return verifier.event, verifier.err
}

type stubStripeProviderStateReader struct {
	candidate EntitlementCandidate
	calls     int
	err       error
}

func (reader *stubStripeProviderStateReader) CurrentEntitlement(context.Context, SubscriptionProviderEvent) (EntitlementCandidate, error) {
	reader.calls++
	return reader.candidate, reader.err
}

type providerEventRepository struct {
	events map[string]SubscriptionProviderEvent
}

func newProviderEventRepository() *providerEventRepository {
	return &providerEventRepository{events: make(map[string]SubscriptionProviderEvent)}
}

func (repository *providerEventRepository) Store(_ context.Context, event SubscriptionProviderEvent) (ProviderEventStoreResult, error) {
	stored, exists := repository.events[event.EventID]
	if !exists {
		repository.events[event.EventID] = event
		return ProviderEventStoreResultInserted, nil
	}
	if stored.PayloadHash == event.PayloadHash {
		return ProviderEventStoreResultDuplicate, nil
	}
	stored.ProcessingState = ProviderEventStateQuarantined
	stored.ConflictPayloadHash = event.PayloadHash
	repository.events[event.EventID] = stored
	return ProviderEventStoreResultConflict, nil
}

func (repository *providerEventRepository) Get(_ context.Context, eventID string) (SubscriptionProviderEvent, error) {
	event, exists := repository.events[eventID]
	if !exists {
		return SubscriptionProviderEvent{}, ErrProviderEventNotFound
	}
	return event, nil
}

func (repository *providerEventRepository) MarkProcessing(_ context.Context, eventID string, _ domain.Timestamp) error {
	event, exists := repository.events[eventID]
	if !exists {
		return ErrProviderEventNotFound
	}
	if event.ProcessingState != ProviderEventStateReceived && event.ProcessingState != ProviderEventStateFailed {
		return ErrProviderEventAlreadyProcessing
	}
	event.ProcessingState = ProviderEventStateProcessing
	event.AttemptCount++
	repository.events[eventID] = event
	return nil
}

func (repository *providerEventRepository) MarkFailed(_ context.Context, eventID string, failedAt domain.Timestamp) error {
	event, exists := repository.events[eventID]
	if !exists {
		return ErrProviderEventNotFound
	}
	event.ProcessingState = ProviderEventStateFailed
	event.ProcessedAt = &failedAt
	repository.events[eventID] = event
	return nil
}

func (repository *providerEventRepository) MarkApplied(_ context.Context, eventID string, sellerID domain.ID, entitlementVersion uint64, appliedAt domain.Timestamp) error {
	event, exists := repository.events[eventID]
	if !exists {
		return ErrProviderEventNotFound
	}
	event.SellerID = sellerID
	event.ProcessingState = ProviderEventStateApplied
	event.AppliedEntitlementVersion = entitlementVersion
	event.ProcessedAt = &appliedAt
	repository.events[eventID] = event
	return nil
}
