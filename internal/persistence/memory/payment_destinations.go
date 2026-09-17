package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/settlement"
)

// PaymentDestinationRepository stores seller destinations for local development.
type PaymentDestinationRepository struct {
	mutex        sync.RWMutex
	destinations map[domain.ID]settlement.PaymentDestination
}

// NewPaymentDestinationRepository creates an empty in-memory destination repository.
func NewPaymentDestinationRepository() *PaymentDestinationRepository {
	return &PaymentDestinationRepository{
		destinations: make(map[domain.ID]settlement.PaymentDestination),
	}
}

// Create stores one new payment destination.
func (repository *PaymentDestinationRepository) Create(
	_ context.Context,
	destination settlement.PaymentDestination,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.destinations[destination.DestinationID]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.destinations[destination.DestinationID] = clonePaymentDestination(destination)
	return nil
}

// Get returns one destination only when it belongs to the supplied seller.
func (repository *PaymentDestinationRepository) Get(
	_ context.Context,
	sellerID domain.ID,
	destinationID domain.ID,
) (settlement.PaymentDestination, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	destination, exists := repository.destinations[destinationID]
	if !exists || destination.SellerID != sellerID {
		return settlement.PaymentDestination{}, persistence.ErrNotFound
	}
	return clonePaymentDestination(destination), nil
}

// ListBySeller returns a copy of every destination owned by one seller.
func (repository *PaymentDestinationRepository) ListBySeller(
	_ context.Context,
	sellerID domain.ID,
) ([]settlement.PaymentDestination, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	destinations := make([]settlement.PaymentDestination, 0)
	for _, destination := range repository.destinations {
		if destination.SellerID == sellerID {
			destinations = append(
				destinations,
				clonePaymentDestination(destination),
			)
		}
	}
	return destinations, nil
}

// clonePaymentDestination protects optional timestamp pointers from aliasing.
func clonePaymentDestination(
	destination settlement.PaymentDestination,
) settlement.PaymentDestination {
	if destination.ChallengeExpiresAt != nil {
		challengeExpiresAt := *destination.ChallengeExpiresAt
		destination.ChallengeExpiresAt = &challengeExpiresAt
	}
	if destination.VerifiedAt != nil {
		verifiedAt := *destination.VerifiedAt
		destination.VerifiedAt = &verifiedAt
	}
	return destination
}
