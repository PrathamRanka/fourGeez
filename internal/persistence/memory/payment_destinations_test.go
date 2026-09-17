package memory

import (
	"context"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/settlement"
)

// TestPaymentDestinationRepositoryScopesReadsToSeller verifies tenant isolation.
func TestPaymentDestinationRepositoryScopesReadsToSeller(t *testing.T) {
	t.Parallel()

	repository := NewPaymentDestinationRepository()
	destination := testMemoryPaymentDestination(t)
	if err := repository.Create(context.Background(), destination); err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.Get(
		context.Background(),
		destination.SellerID,
		destination.DestinationID,
	)
	if err != nil || loaded != destination {
		t.Fatalf("Get() = (%#v, %v)", loaded, err)
	}
	otherSellerID := mustMemoryID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H6",
		domain.SellerIDPrefix,
	)
	if _, err := repository.Get(
		context.Background(),
		otherSellerID,
		destination.DestinationID,
	); err != persistence.ErrNotFound {
		t.Fatalf("cross-seller Get() error = %v", err)
	}
}

// testMemoryPaymentDestination creates one valid persistence fixture.
func testMemoryPaymentDestination(t *testing.T) settlement.PaymentDestination {
	t.Helper()

	destination, err := settlement.NewPaymentDestination(
		settlement.PaymentDestinationParams{
			DestinationID: mustMemoryID(
				t,
				"dst_01K5D09YJ0C0M7RJM4FWQ0K9H8",
				domain.PaymentDestinationIDPrefix,
			),
			SellerID: mustMemoryID(
				t,
				"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				domain.SellerIDPrefix,
			),
			Asset:     "USDC",
			Network:   "eip155:84532",
			Address:   "0x1111111111111111111111111111111111111111",
			CreatedAt: domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

// mustMemoryID parses one stable repository fixture identifier.
func mustMemoryID(t *testing.T, value string, prefix domain.IDPrefix) domain.ID {
	t.Helper()

	identifier, err := domain.ParseID(value, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
