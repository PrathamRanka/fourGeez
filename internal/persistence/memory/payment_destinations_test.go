package memory

import (
	"context"
	"strings"
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

// TestPaymentDestinationRepositoryActivatesAndRotatesAtomically verifies WAL-002 state writes.
func TestPaymentDestinationRepositoryActivatesAndRotatesAtomically(t *testing.T) {
	t.Parallel()

	repository := NewPaymentDestinationRepository()
	first := testMemoryPaymentDestination(t)
	second := first
	second.DestinationID = mustMemoryID(
		t,
		"dst_01K5D09YJ0C0M7RJM4FWQ0K9H9",
		domain.PaymentDestinationIDPrefix,
	)
	second.Address = "0x2222222222222222222222222222222222222222"
	if err := repository.Create(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	first.Status = settlement.PaymentDestinationStatusActive
	first.Version++
	if err := repository.Activate(
		context.Background(),
		settlement.PaymentDestinationActivation{
			Destination:     first,
			ExpectedVersion: 1,
		},
	); err != nil {
		t.Fatal(err)
	}
	rotated := first
	rotated.Status = settlement.PaymentDestinationStatusRotated
	rotated.Version++
	second.Status = settlement.PaymentDestinationStatusActive
	second.Version++
	if err := repository.Activate(
		context.Background(),
		settlement.PaymentDestinationActivation{
			Destination:            second,
			ExpectedVersion:        1,
			RotatedDestination:     &rotated,
			RotatedExpectedVersion: 2,
		},
	); err != nil {
		t.Fatal(err)
	}
	loadedFirst, _ := repository.Get(context.Background(), first.SellerID, first.DestinationID)
	loadedSecond, _ := repository.Get(context.Background(), second.SellerID, second.DestinationID)
	if loadedFirst.Status != settlement.PaymentDestinationStatusRotated ||
		loadedSecond.Status != settlement.PaymentDestinationStatusActive {
		t.Fatalf("rotation states = %q and %q", loadedFirst.Status, loadedSecond.Status)
	}
}

// TestPaymentDestinationRepositoryRejectsStaleChallengeWrite verifies optimistic concurrency.
func TestPaymentDestinationRepositoryRejectsStaleChallengeWrite(t *testing.T) {
	t.Parallel()

	repository := NewPaymentDestinationRepository()
	destination := testMemoryPaymentDestination(t)
	if err := repository.Create(context.Background(), destination); err != nil {
		t.Fatal(err)
	}
	destination.ChallengeHash = strings.Repeat("a", 64)
	destination.Version++
	err := repository.SaveChallenge(context.Background(), destination, 99)
	if err != persistence.ErrConditionFailed {
		t.Fatalf("SaveChallenge() error = %v, want condition failure", err)
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
