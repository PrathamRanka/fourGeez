package payments

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/proxy"
)

// TestCheckoutServiceRecordsChallengeAndVerification verifies safe payment evidence.
func TestCheckoutServiceRecordsChallengeAndVerification(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	transactionRepository := memory.NewTransactionRepository()
	evidenceRepository := memory.NewEvidenceRepository()
	evidenceSigner, err := evidence.NewLocalHMACSigner(
		"local-evidence-key",
		[]byte(strings.Repeat("e", 32)),
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder := evidence.NewRecorder(
		evidenceRepository,
		domain.NewULIDGenerator(
			fixture.clock,
			strings.NewReader(strings.Repeat("r", 256)),
		),
		evidenceSigner,
		fixture.clock,
	)
	executor := &checkoutExecutor{}
	service := NewCheckoutService(
		fixture.service,
		NewMockAdapter(),
		transactionRepository,
		recorder,
		executor,
		fixture.clock,
	)
	request := CheckoutRequest{
		PaidRouteRequest: PaidRouteRequest{
			Slug:      fixture.seller.Slug,
			Method:    fixture.route.Method,
			ProxyPath: fixture.route.PathPattern,
			IntentID:  fixture.purchaseIntent.IntentID(),
			BuyerID:   fixture.purchaseIntent.BuyerID(),
		},
	}

	challenge, err := service.Execute(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Challenge == nil || challenge.Response != nil {
		t.Fatalf("challenge result = %#v", challenge)
	}

	request.PaymentProof = MockApprovedProof
	delivery, err := service.Execute(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if delivery.Response == nil || delivery.SettlementHeader == "" {
		t.Fatalf("delivery result = %#v", delivery)
	}
	events, err := evidenceRepository.ListByTransaction(
		t.Context(),
		delivery.TransactionID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 ||
		events[0].EventType != evidence.EventPaymentChallenged ||
		events[1].EventType != evidence.EventPaymentVerified {
		t.Fatalf("events = %#v", events)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), MockApprovedProof) {
		t.Fatal("evidence contains the raw payment proof")
	}
}

type checkoutExecutor struct{}

// Execute returns a deterministic seller response for checkout tests.
func (executor *checkoutExecutor) Execute(
	context.Context,
	proxy.ExecutionRequest,
) (proxy.ForwardResponse, error) {
	return proxy.ForwardResponse{
		StatusCode:  200,
		Body:        []byte(`{"ok":true}`),
		ContentType: "application/json",
	}, nil
}
