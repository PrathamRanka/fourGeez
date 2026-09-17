package evidence

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

func TestAppendAndVerifyEvidenceChain(t *testing.T) {
	t.Parallel()

	signer := hmacTestSigner{keyID: "test-key-v1", key: []byte("test evidence signing key")}
	transactionID := mustEvidenceID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))

	first, err := Append(context.Background(), EventParams{
		EventID:       mustEvidenceID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.EvidenceIDPrefix),
		TransactionID: transactionID,
		Sequence:      1,
		EventType:     EventIntentCreated,
		ActorType:     ActorBuyer,
		ActorID:       "agent_demo_123",
		Payload:       map[string]any{"amount": "35000000", "routeId": "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7"},
		CreatedAt:     createdAt,
	}, nil, signer)
	if err != nil {
		t.Fatalf("Append(first) error = %v", err)
	}
	second, err := Append(context.Background(), EventParams{
		EventID:       mustEvidenceID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.EvidenceIDPrefix),
		TransactionID: transactionID,
		Sequence:      2,
		EventType:     EventPaymentVerified,
		ActorType:     ActorSystem,
		Payload:       map[string]any{"paymentProofHash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		CreatedAt:     createdAt.Add(time.Second),
	}, &first, signer)
	if err != nil {
		t.Fatalf("Append(second) error = %v", err)
	}

	if second.PreviousEventHash == nil || *second.PreviousEventHash != first.EventHash {
		t.Fatal("second event was not linked to the first event hash")
	}
	if err := VerifyChain(context.Background(), []Event{first, second}, signer); err != nil {
		t.Fatalf("VerifyChain() error = %v", err)
	}
}

func TestVerifyChainRejectsTampering(t *testing.T) {
	t.Parallel()

	signer := hmacTestSigner{keyID: "test-key-v1", key: []byte("test evidence signing key")}
	chain := newEvidenceChain(t, signer)

	tests := []struct {
		name   string
		mutate func([]Event) []Event
	}{
		{name: "payload", mutate: func(events []Event) []Event { events[0].Payload["amount"] = "1"; return events }},
		{name: "order", mutate: func(events []Event) []Event { events[0], events[1] = events[1], events[0]; return events }},
		{name: "previous hash", mutate: func(events []Event) []Event {
			digest := mustEvidenceDigest(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
			events[1].PreviousEventHash = &digest
			return events
		}},
		{name: "event hash", mutate: func(events []Event) []Event {
			events[1].EventHash = mustEvidenceDigest(t, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
			return events
		}},
		{name: "signature", mutate: func(events []Event) []Event { events[1].KMSSignature = "invalid"; return events }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mutated := test.mutate(cloneEvents(t, chain))
			if err := VerifyChain(context.Background(), mutated, signer); !errors.Is(err, ErrChainInvalid) {
				t.Fatalf("VerifyChain() error = %v, want ErrChainInvalid", err)
			}
		})
	}
}

func TestAppendRejectsInvalidSequenceAndVocabulary(t *testing.T) {
	t.Parallel()

	signer := hmacTestSigner{keyID: "test-key-v1", key: []byte("key")}
	params := validEventParams(t)
	params.Sequence = 2
	if _, err := Append(context.Background(), params, nil, signer); err == nil {
		t.Fatal("Append() accepted a first event whose sequence was not one")
	}

	params = validEventParams(t)
	params.EventType = "payment.proof.raw"
	if _, err := Append(context.Background(), params, nil, signer); err == nil {
		t.Fatal("Append() accepted an undocumented event type")
	}
}

func newEvidenceChain(t *testing.T, signer Signer) []Event {
	t.Helper()
	params := validEventParams(t)
	first, err := Append(context.Background(), params, nil, signer)
	if err != nil {
		t.Fatalf("Append(first) error = %v", err)
	}
	params.EventID = mustEvidenceID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.EvidenceIDPrefix)
	params.Sequence = 2
	params.EventType = EventDeliverySucceeded
	params.ActorType = ActorSeller
	params.ActorID = "seller-demo"
	params.Payload = map[string]any{"upstreamStatus": 200}
	params.CreatedAt = params.CreatedAt.Add(time.Second)
	second, err := Append(context.Background(), params, &first, signer)
	if err != nil {
		t.Fatalf("Append(second) error = %v", err)
	}
	return []Event{first, second}
}

func validEventParams(t *testing.T) EventParams {
	t.Helper()
	return EventParams{
		EventID:       mustEvidenceID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.EvidenceIDPrefix),
		TransactionID: mustEvidenceID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		Sequence:      1,
		EventType:     EventIntentCreated,
		ActorType:     ActorBuyer,
		ActorID:       "agent_demo_123",
		Payload:       map[string]any{"amount": "35000000"},
		CreatedAt:     domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)),
	}
}

func cloneEvents(t *testing.T, events []Event) []Event {
	t.Helper()
	cloned := make([]Event, len(events))
	for index, event := range events {
		cloned[index] = event
		cloned[index].Payload = make(map[string]any, len(event.Payload))
		for key, value := range event.Payload {
			cloned[index].Payload[key] = value
		}
		if event.PreviousEventHash != nil {
			digest := *event.PreviousEventHash
			cloned[index].PreviousEventHash = &digest
		}
	}
	return cloned
}

type hmacTestSigner struct {
	keyID string
	key   []byte
}

func (signer hmacTestSigner) Sign(_ context.Context, digest []byte) (Signature, error) {
	mac := hmac.New(sha256.New, signer.key)
	_, _ = mac.Write(digest)
	return Signature{KeyID: signer.keyID, Value: base64.StdEncoding.EncodeToString(mac.Sum(nil))}, nil
}

func (signer hmacTestSigner) Verify(_ context.Context, keyID string, digest []byte, signature string) (bool, error) {
	if keyID != signer.keyID {
		return false, nil
	}
	want, err := signer.Sign(context.Background(), digest)
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(want.Value), []byte(signature)), nil
}

func mustEvidenceID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatalf("ParseID(%q) error = %v", raw, err)
	}
	return identifier
}

func mustEvidenceDigest(t *testing.T, raw string) intents.SHA256Digest {
	t.Helper()
	digest, err := intents.ParseSHA256Digest(raw)
	if err != nil {
		t.Fatalf("ParseSHA256Digest() error = %v", err)
	}
	return digest
}
