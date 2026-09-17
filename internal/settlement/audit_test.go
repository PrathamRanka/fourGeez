package settlement

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestServiceAuditsDestinationVerificationAndRotation verifies wallet lifecycle events.
func TestServiceAuditsDestinationVerificationAndRotation(t *testing.T) {
	t.Parallel()

	sellerID := mustSettlementID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	}
	recorder := &settlementAuditRecorder{}
	service := NewService(
		newTestPaymentDestinationRepository(),
		testSellerAuthorizer{sellerID: sellerID, ownerSubject: "seller-user"},
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("a", 512))),
		fixedOwnershipNonceGenerator{nonce: "stable-nonce"},
		testOwnershipVerifier{valid: true},
		clock,
		recorder,
	)

	first := createAuditedDestination(t, service, sellerID, "0x1111111111111111111111111111111111111111")
	verifyAuditedDestination(t, service, sellerID, first, false)
	second := createAuditedDestination(t, service, sellerID, "0x2222222222222222222222222222222222222222")
	verifyAuditedDestination(t, service, sellerID, second, true)

	actions := make([]audit.Action, len(recorder.requests))
	for index, request := range recorder.requests {
		actions[index] = request.Action
	}
	want := []audit.Action{
		audit.ActionPaymentDestinationCreated,
		audit.ActionPaymentDestinationVerified,
		audit.ActionPaymentDestinationCreated,
		audit.ActionPaymentDestinationVerified,
		audit.ActionPaymentDestinationRotated,
	}
	if len(actions) != len(want) {
		t.Fatalf("audit actions = %#v, want %#v", actions, want)
	}
	for index := range want {
		if actions[index] != want[index] {
			t.Fatalf("audit actions = %#v, want %#v", actions, want)
		}
	}
}

// createAuditedDestination creates one pending destination fixture.
func createAuditedDestination(
	t *testing.T,
	service *Service,
	sellerID domain.ID,
	address string,
) PaymentDestination {
	t.Helper()

	destination, err := service.Create(
		t.Context(),
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

// verifyAuditedDestination activates one destination fixture.
func verifyAuditedDestination(
	t *testing.T,
	service *Service,
	sellerID domain.ID,
	destination PaymentDestination,
	confirmRotation bool,
) {
	t.Helper()

	challenge, err := service.IssueOwnershipChallenge(
		t.Context(),
		"seller-user",
		sellerID,
		destination.DestinationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.VerifyOwnership(
		t.Context(),
		"seller-user",
		sellerID,
		destination.DestinationID,
		VerifyOwnershipRequest{
			Challenge:       challenge.Challenge,
			Signature:       "valid-signature",
			ConfirmRotation: confirmRotation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}

type settlementAuditRecorder struct {
	requests []audit.RecordRequest
}

// Record captures one payment-destination audit request.
func (recorder *settlementAuditRecorder) Record(
	_ context.Context,
	request audit.RecordRequest,
) error {
	recorder.requests = append(recorder.requests, request)
	return nil
}
