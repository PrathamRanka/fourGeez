package evidence

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

// TestRecorderAppendsSafePaymentAndDeliveryEvidence verifies the PAY-009 chain.
func TestRecorderAppendsSafePaymentAndDeliveryEvidence(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	repository := &recordingEvidenceRepository{}
	signer, err := NewLocalHMACSigner(
		"local-evidence-key",
		[]byte(strings.Repeat("e", 32)),
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder := NewRecorder(
		repository,
		domain.NewULIDGenerator(
			clock,
			strings.NewReader(strings.Repeat("r", 256)),
		),
		signer,
		clock,
	)
	transactionID := mustEvidenceID(
		t,
		"txn_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.TransactionIDPrefix,
	)
	proofHash, err := intents.ParseSHA256Digest(strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	responseHash, err := intents.ParseSHA256Digest(strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}

	if err := recorder.RecordPaymentChallenge(
		t.Context(),
		transactionID,
		PaymentChallengeFacts{
			Amount:  "10000",
			Asset:   "test-usdc",
			Network: "test-network",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := recorder.RecordPaymentVerification(
		t.Context(),
		transactionID,
		PaymentVerificationFacts{
			PaymentIdentifier: "x402_identifier",
			PaymentProofHash:  proofHash,
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := recorder.RecordProxyForwarding(
		t.Context(),
		transactionID,
		ProxyForwardingFacts{
			SellerID: "sel_123",
			RouteID:  "rte_123",
			Method:   "POST",
			Path:     "/research",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := recorder.RecordDelivery(
		t.Context(),
		transactionID,
		DeliveryFacts{
			Succeeded:     true,
			StatusCode:    200,
			ResponseHash:  responseHash,
			ContentType:   "application/json",
			ContentLength: 11,
		},
	); err != nil {
		t.Fatal(err)
	}

	events, err := repository.ListByTransaction(t.Context(), transactionID)
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := []EventType{
		EventPaymentChallenged,
		EventPaymentVerified,
		EventProxyForwarded,
		EventDeliverySucceeded,
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("event count = %d", len(events))
	}
	for index, event := range events {
		if event.Sequence != uint64(index+1) || event.EventType != wantTypes[index] {
			t.Fatalf("event %d = %#v", index, event)
		}
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"raw-payment-signature",
		"approval-token",
		"seller-secret",
		`{"private":"response body"}`,
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("evidence contains forbidden value %q", forbidden)
		}
	}
	if err := VerifyChain(t.Context(), events, signer); err != nil {
		t.Fatalf("VerifyChain() error = %v", err)
	}
}

type recordingEvidenceRepository struct {
	mutex  sync.Mutex
	events []Event
}

// Append stores one sequential event for recorder tests.
func (repository *recordingEvidenceRepository) Append(
	_ context.Context,
	event Event,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.events = append(repository.events, event)
	return nil
}

// ListByTransaction returns the recorded chain.
func (repository *recordingEvidenceRepository) ListByTransaction(
	_ context.Context,
	_ domain.ID,
) ([]Event, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	return append([]Event(nil), repository.events...), nil
}
