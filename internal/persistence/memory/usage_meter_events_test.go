package memory

import (
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestUsageMeterEventRepositoryDeduplicatesSource verifies immutable usage claims.
func TestUsageMeterEventRepositoryDeduplicatesSource(t *testing.T) {
	t.Parallel()
	repository := NewUsageMeterEventRepository()
	event := testMemoryUsageMeterEvent(t)
	first, created, err := repository.CreateIfAbsent(t.Context(), event)
	if err != nil || !created {
		t.Fatalf("first CreateIfAbsent() = (%#v, %v, %v)", first, created, err)
	}
	second, created, err := repository.CreateIfAbsent(t.Context(), event)
	if err != nil || created || second.MeterEventID() != first.MeterEventID() {
		t.Fatalf("second CreateIfAbsent() = (%#v, %v, %v)", second, created, err)
	}
}

// testMemoryUsageMeterEvent creates one valid immutable usage fixture.
func testMemoryUsageMeterEvent(t *testing.T) billing.UsageMeterEvent {
	t.Helper()
	event, err := billing.NewUsageMeterEvent(billing.UsageMeterEventParams{
		MeterEventID:        mustMemoryID(t, "mtr_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.UsageMeterEventIDPrefix),
		SellerID:            mustMemoryID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		MeterName:           billing.MeterSuccessfulTransaction,
		Quantity:            1,
		SourceTransactionID: mustMemoryID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		PlanID:              billing.PlanStarter,
		PlanVersion:         1,
		OccurredAt:          domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return event
}
