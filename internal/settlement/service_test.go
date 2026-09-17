package settlement

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// TestServiceCreatesListsAndReadsSellerDestination verifies WAL-001 behavior.
func TestServiceCreatesListsAndReadsSellerDestination(t *testing.T) {
	t.Parallel()

	sellerID := mustSettlementID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	repository := newTestPaymentDestinationRepository()
	service := NewService(
		repository,
		testSellerAuthorizer{sellerID: sellerID, ownerSubject: "seller-user"},
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("a", 128))),
		fixedOwnershipNonceGenerator{nonce: "stable-nonce"},
		testOwnershipVerifier{valid: true},
		clock,
	)

	created, err := service.Create(
		context.Background(),
		"seller-user",
		sellerID,
		CreatePaymentDestinationRequest{
			Asset:   "USDC",
			Network: "eip155:84532",
			Address: "0x1111111111111111111111111111111111111111",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := service.List(context.Background(), "seller-user", sellerID)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := service.Get(
		context.Background(),
		"seller-user",
		sellerID,
		created.DestinationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || loaded != created {
		t.Fatalf("created/listed/loaded mismatch: %#v %#v %#v", created, listed, loaded)
	}
}

// TestServiceHidesDestinationsFromOtherSellers verifies tenant isolation.
func TestServiceHidesDestinationsFromOtherSellers(t *testing.T) {
	t.Parallel()

	sellerID := mustSettlementID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	clock := domain.FixedClock{Value: time.Now().UTC()}
	service := NewService(
		newTestPaymentDestinationRepository(),
		testSellerAuthorizer{sellerID: sellerID, ownerSubject: "seller-user"},
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("b", 128))),
		fixedOwnershipNonceGenerator{nonce: "stable-nonce"},
		testOwnershipVerifier{valid: true},
		clock,
	)

	_, err := service.List(context.Background(), "different-user", sellerID)
	if !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("List() error = %v, want not found", err)
	}
}

type testSellerAuthorizer struct {
	sellerID     domain.ID
	ownerSubject string
}

// AuthorizeSeller verifies the expected owner and seller fixture.
func (authorizer testSellerAuthorizer) AuthorizeSeller(
	_ context.Context,
	ownerSubject string,
	sellerID domain.ID,
) error {
	if ownerSubject != authorizer.ownerSubject || sellerID != authorizer.sellerID {
		return persistence.ErrNotFound
	}
	return nil
}

type testPaymentDestinationRepository struct {
	destinations map[domain.ID]PaymentDestination
}

// newTestPaymentDestinationRepository creates an empty repository fixture.
func newTestPaymentDestinationRepository() *testPaymentDestinationRepository {
	return &testPaymentDestinationRepository{
		destinations: make(map[domain.ID]PaymentDestination),
	}
}

// Create stores one destination once.
func (repository *testPaymentDestinationRepository) Create(
	_ context.Context,
	destination PaymentDestination,
) error {
	if _, exists := repository.destinations[destination.DestinationID]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.destinations[destination.DestinationID] = destination
	return nil
}

// Get loads one seller-owned destination.
func (repository *testPaymentDestinationRepository) Get(
	_ context.Context,
	sellerID domain.ID,
	destinationID domain.ID,
) (PaymentDestination, error) {
	destination, exists := repository.destinations[destinationID]
	if !exists || destination.SellerID != sellerID {
		return PaymentDestination{}, persistence.ErrNotFound
	}
	return destination, nil
}

// ListBySeller returns all destinations owned by one seller.
func (repository *testPaymentDestinationRepository) ListBySeller(
	_ context.Context,
	sellerID domain.ID,
) ([]PaymentDestination, error) {
	destinations := make([]PaymentDestination, 0)
	for _, destination := range repository.destinations {
		if destination.SellerID == sellerID {
			destinations = append(destinations, destination)
		}
	}
	return destinations, nil
}
