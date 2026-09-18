package memory

import (
	"testing"

	"github.com/fourgeez/agentpay/internal/billing"
)

func TestProviderEventRepositoryDeduplicatesAndQuarantinesHashConflicts(t *testing.T) {
	t.Parallel()

	repository := NewProviderEventRepository()
	event := billing.SubscriptionProviderEvent{EventID: "evt_123", Provider: billing.EntitlementProviderStripe, EventType: "invoice.paid", PayloadHash: "hash-one", ProcessingState: billing.ProviderEventStateReceived, ReceivedAt: memoryTime(2026, 9, 18, 10)}
	result, err := repository.Store(t.Context(), event)
	if err != nil || result != billing.ProviderEventStoreResultInserted {
		t.Fatalf("first Store() = (%v, %v)", result, err)
	}
	result, err = repository.Store(t.Context(), event)
	if err != nil || result != billing.ProviderEventStoreResultDuplicate {
		t.Fatalf("duplicate Store() = (%v, %v)", result, err)
	}
	conflict := event
	conflict.PayloadHash = "hash-two"
	result, err = repository.Store(t.Context(), conflict)
	if err != nil || result != billing.ProviderEventStoreResultConflict {
		t.Fatalf("conflicting Store() = (%v, %v)", result, err)
	}
	stored, err := repository.Get(t.Context(), event.EventID)
	if err != nil || stored.ProcessingState != billing.ProviderEventStateQuarantined || stored.ConflictPayloadHash != conflict.PayloadHash {
		t.Fatalf("quarantined event = (%#v, %v)", stored, err)
	}
}
