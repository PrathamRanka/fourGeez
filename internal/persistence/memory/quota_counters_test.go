package memory

import (
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/operations"
)

// TestQuotaCounterRepositoryEnforcesLimitsAndUniqueSources verifies atomic quota behavior.
func TestQuotaCounterRepositoryEnforcesLimitsAndUniqueSources(t *testing.T) {
	t.Parallel()

	repository := NewQuotaCounterRepository()
	request := testQuotaCounterRequest(t)
	if err := repository.Increment(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if err := repository.Increment(t.Context(), request); !errors.Is(err, domain.ErrRateLimitExceeded) {
		t.Fatalf("second Increment() error = %v", err)
	}

	uniqueRequest := request
	uniqueRequest.QuotaName = operations.QuotaWebhookDelivery
	if err := repository.IncrementUnique(t.Context(), uniqueRequest, "whk_1:evt_1"); err != nil {
		t.Fatal(err)
	}
	if err := repository.IncrementUnique(t.Context(), uniqueRequest, "whk_1:evt_1"); err != nil {
		t.Fatalf("replayed IncrementUnique() error = %v", err)
	}
	if err := repository.IncrementUnique(t.Context(), uniqueRequest, "whk_2:evt_2"); !errors.Is(err, domain.ErrRateLimitExceeded) {
		t.Fatalf("new IncrementUnique() error = %v", err)
	}
}

// testQuotaCounterRequest creates one monthly quota fixture.
func testQuotaCounterRequest(t *testing.T) operations.CounterRequest {
	t.Helper()

	return operations.CounterRequest{
		SellerID: mustMemoryID(
			t,
			"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
			domain.SellerIDPrefix,
		),
		QuotaName:   operations.QuotaAPIRequest,
		PeriodStart: domain.NewTimestamp(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)),
		PeriodEnd:   domain.NewTimestamp(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)),
		Limit:       1,
		UpdatedAt:   domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)),
	}
}
