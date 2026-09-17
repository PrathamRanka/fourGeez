package settlement

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// TestServiceIssuesAndVerifiesOwnershipChallenge covers the successful WAL-002 flow.
func TestServiceIssuesAndVerifiesOwnershipChallenge(t *testing.T) {
	t.Parallel()

	service, repository, sellerID := newOwnershipTestService(t)
	destination := createOwnershipTestDestination(t, service, sellerID, "0x1111111111111111111111111111111111111111")

	challenge, err := service.IssueOwnershipChallenge(
		context.Background(),
		"seller-user",
		sellerID,
		destination.DestinationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repository.Get(context.Background(), sellerID, destination.DestinationID)
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Challenge == "" || stored.ChallengeHash == "" {
		t.Fatalf("challenge response or persisted hash is empty: %#v %#v", challenge, stored)
	}
	if strings.Contains(stored.ChallengeHash, challenge.Challenge) {
		t.Fatal("repository stored raw challenge material")
	}
	if stored.Version != 2 {
		t.Fatalf("challenge version = %d, want 2", stored.Version)
	}

	verified, err := service.VerifyOwnership(
		context.Background(),
		"seller-user",
		sellerID,
		destination.DestinationID,
		VerifyOwnershipRequest{
			Challenge: challenge.Challenge,
			Signature: "0xsigned-proof",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Status != PaymentDestinationStatusActive || verified.VerifiedAt == nil {
		t.Fatalf("verified destination = %#v", verified)
	}
	if verified.ChallengeHash != "" || verified.ChallengeExpiresAt != nil {
		t.Fatalf("verified challenge material was not cleared: %#v", verified)
	}
}

// TestServiceRejectsInvalidOwnershipChallenges covers proof and expiry failures.
func TestServiceRejectsInvalidOwnershipChallenges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		challenge     string
		verifierValid bool
		advanceClock  time.Duration
		wantError     error
	}{
		{
			name:          "challenge mismatch",
			challenge:     "different challenge",
			verifierValid: true,
			wantError:     ErrOwnershipChallengeMismatch,
		},
		{
			name:          "invalid signature",
			verifierValid: false,
			wantError:     ErrOwnershipProofInvalid,
		},
		{
			name:          "expired challenge",
			verifierValid: true,
			advanceClock:  OwnershipChallengeLifetime + time.Second,
			wantError:     ErrOwnershipChallengeExpired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clock := &mutableSettlementClock{
				now: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
			}
			repository := newTestPaymentDestinationRepository()
			sellerID := mustSettlementID(
				t,
				"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				domain.SellerIDPrefix,
			)
			service := NewService(
				repository,
				testSellerAuthorizer{sellerID: sellerID, ownerSubject: "seller-user"},
				domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("a", 128))),
				fixedOwnershipNonceGenerator{nonce: "stable-nonce"},
				testOwnershipVerifier{valid: test.verifierValid},
				clock,
				audit.NoopRecorder{},
			)
			destination := createOwnershipTestDestination(
				t,
				service,
				sellerID,
				"0x1111111111111111111111111111111111111111",
			)
			issued, err := service.IssueOwnershipChallenge(
				context.Background(),
				"seller-user",
				sellerID,
				destination.DestinationID,
			)
			if err != nil {
				t.Fatal(err)
			}
			clock.now = clock.now.Add(test.advanceClock)
			challenge := issued.Challenge
			if test.challenge != "" {
				challenge = test.challenge
			}
			_, err = service.VerifyOwnership(
				context.Background(),
				"seller-user",
				sellerID,
				destination.DestinationID,
				VerifyOwnershipRequest{
					Challenge: challenge,
					Signature: "0xsigned-proof",
				},
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("VerifyOwnership() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

// TestServiceRequiresConfirmedRotation verifies one active destination per pair.
func TestServiceRequiresConfirmedRotation(t *testing.T) {
	t.Parallel()

	service, repository, sellerID := newOwnershipTestService(t)
	first := createOwnershipTestDestination(t, service, sellerID, "0x1111111111111111111111111111111111111111")
	activateOwnershipTestDestination(t, service, sellerID, first.DestinationID, false)
	second := createOwnershipTestDestination(t, service, sellerID, "0x2222222222222222222222222222222222222222")
	challenge, err := service.IssueOwnershipChallenge(
		context.Background(),
		"seller-user",
		sellerID,
		second.DestinationID,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.VerifyOwnership(
		context.Background(),
		"seller-user",
		sellerID,
		second.DestinationID,
		VerifyOwnershipRequest{
			Challenge: challenge.Challenge,
			Signature: "0xsigned-proof",
		},
	)
	if !errors.Is(err, ErrRotationConfirmationRequired) {
		t.Fatalf("VerifyOwnership() error = %v, want rotation confirmation", err)
	}
	activated, err := service.VerifyOwnership(
		context.Background(),
		"seller-user",
		sellerID,
		second.DestinationID,
		VerifyOwnershipRequest{
			Challenge:       challenge.Challenge,
			Signature:       "0xsigned-proof",
			ConfirmRotation: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := repository.Get(context.Background(), sellerID, first.DestinationID)
	if err != nil {
		t.Fatal(err)
	}
	if activated.Status != PaymentDestinationStatusActive ||
		rotated.Status != PaymentDestinationStatusRotated {
		t.Fatalf("activation states = %q and %q", activated.Status, rotated.Status)
	}
}

// newOwnershipTestService creates deterministic ownership-verification dependencies.
func newOwnershipTestService(
	t *testing.T,
) (*Service, *testPaymentDestinationRepository, domain.ID) {
	t.Helper()

	sellerID := mustSettlementID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	}
	repository := newTestPaymentDestinationRepository()
	service := NewService(
		repository,
		testSellerAuthorizer{sellerID: sellerID, ownerSubject: "seller-user"},
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("a", 256))),
		fixedOwnershipNonceGenerator{nonce: "stable-nonce"},
		testOwnershipVerifier{valid: true},
		clock,
		audit.NoopRecorder{},
	)
	return service, repository, sellerID
}

// createOwnershipTestDestination creates one pending destination fixture.
func createOwnershipTestDestination(
	t *testing.T,
	service *Service,
	sellerID domain.ID,
	address string,
) PaymentDestination {
	t.Helper()

	destination, err := service.Create(
		context.Background(),
		"seller-user",
		sellerID,
		CreatePaymentDestinationRequest{
			Asset:   "USDC",
			Network: "eip155:84532",
			Address: address,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

// activateOwnershipTestDestination completes a challenge for one destination.
func activateOwnershipTestDestination(
	t *testing.T,
	service *Service,
	sellerID domain.ID,
	destinationID domain.ID,
	confirmRotation bool,
) PaymentDestination {
	t.Helper()

	challenge, err := service.IssueOwnershipChallenge(
		context.Background(),
		"seller-user",
		sellerID,
		destinationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	destination, err := service.VerifyOwnership(
		context.Background(),
		"seller-user",
		sellerID,
		destinationID,
		VerifyOwnershipRequest{
			Challenge:       challenge.Challenge,
			Signature:       "0xsigned-proof",
			ConfirmRotation: confirmRotation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

type fixedOwnershipNonceGenerator struct {
	nonce string
}

// NewNonce returns deterministic challenge entropy for tests.
func (generator fixedOwnershipNonceGenerator) NewNonce() (string, error) {
	return generator.nonce, nil
}

type testOwnershipVerifier struct {
	valid bool
}

// VerifyOwnership returns the configured proof result without storing proof material.
func (verifier testOwnershipVerifier) VerifyOwnership(
	_ context.Context,
	_ PaymentDestination,
	_ string,
	_ string,
) (bool, error) {
	return verifier.valid, nil
}

type mutableSettlementClock struct {
	now time.Time
}

// Now returns the mutable test instant.
func (clock *mutableSettlementClock) Now() time.Time {
	return clock.now
}

// SaveChallenge replaces one destination after a version check.
func (repository *testPaymentDestinationRepository) SaveChallenge(
	_ context.Context,
	destination PaymentDestination,
	expectedVersion uint64,
) error {
	stored, exists := repository.destinations[destination.DestinationID]
	if !exists || stored.Version != expectedVersion {
		return persistence.ErrConditionFailed
	}
	repository.destinations[destination.DestinationID] = destination
	return nil
}

// Activate atomically activates one destination and rotates its predecessor.
func (repository *testPaymentDestinationRepository) Activate(
	_ context.Context,
	activation PaymentDestinationActivation,
) error {
	stored, exists := repository.destinations[activation.Destination.DestinationID]
	if !exists || stored.Version != activation.ExpectedVersion {
		return persistence.ErrConditionFailed
	}
	if activation.RotatedDestination != nil {
		rotated, exists := repository.destinations[activation.RotatedDestination.DestinationID]
		if !exists || rotated.Version != activation.RotatedExpectedVersion {
			return persistence.ErrConditionFailed
		}
		repository.destinations[rotated.DestinationID] = *activation.RotatedDestination
	}
	repository.destinations[activation.Destination.DestinationID] = activation.Destination
	return nil
}
