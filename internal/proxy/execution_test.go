package proxy

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// TestExecutionServiceForwardsExactlyOnce verifies the conditional claim gate.
func TestExecutionServiceForwardsExactlyOnce(t *testing.T) {
	t.Parallel()

	repository := memory.NewTransactionRepository()
	transaction := verifiedTransaction(t)
	if err := repository.Create(t.Context(), transaction); err != nil {
		t.Fatal(err)
	}
	forwarder := &recordingForwarder{}
	signer := &recordingRequestSigner{}
	recorder := &recordingLifecycleRecorder{}
	service := NewExecutionService(
		repository,
		signer,
		forwarder,
		recorder,
		domain.FixedClock{
			Value: time.Date(2026, time.September, 17, 10, 1, 0, 0, time.UTC),
		},
	)
	request := validExecutionRequest(t, transaction)

	const attempts = 20
	var waitGroup sync.WaitGroup
	var successes atomic.Int32
	var duplicates atomic.Int32
	for range attempts {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			_, err := service.Execute(context.Background(), request)
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, ErrAlreadyForwarded):
				duplicates.Add(1)
			default:
				t.Errorf("Execute() error = %v", err)
			}
		}()
	}
	waitGroup.Wait()

	if successes.Load() != 1 || duplicates.Load() != attempts-1 {
		t.Fatalf(
			"successes = %d, duplicates = %d",
			successes.Load(),
			duplicates.Load(),
		)
	}
	if forwarder.calls.Load() != 1 || signer.calls.Load() != 1 {
		t.Fatalf(
			"forward calls = %d, signer calls = %d",
			forwarder.calls.Load(),
			signer.calls.Load(),
		)
	}
	if recorder.forwarding.Load() != 1 || recorder.delivery.Load() != 1 {
		t.Fatalf(
			"evidence calls = (%d, %d)",
			recorder.forwarding.Load(),
			recorder.delivery.Load(),
		)
	}
	stored, err := repository.Get(t.Context(), transaction.TransactionID())
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status() != transactions.StatusFulfilled {
		t.Fatalf("transaction status = %s", stored.Status())
	}
}

// TestExecutionServiceMetersSuccessWithoutBlockingFulfillment verifies billing isolation.
func TestExecutionServiceMetersSuccessWithoutBlockingFulfillment(t *testing.T) {
	t.Parallel()

	repository := memory.NewTransactionRepository()
	transaction := verifiedTransaction(t)
	if err := repository.Create(t.Context(), transaction); err != nil {
		t.Fatal(err)
	}
	recorder := &failingUsageRecorder{}
	service := NewExecutionService(
		repository,
		&recordingRequestSigner{},
		&recordingForwarder{},
		&recordingLifecycleRecorder{},
		domain.FixedClock{Value: time.Date(2026, time.September, 17, 10, 1, 0, 0, time.UTC)},
	)
	service.SetUsageRecorder(recorder)
	response, err := service.Execute(t.Context(), validExecutionRequest(t, transaction))
	if err != nil || response.StatusCode != 200 {
		t.Fatalf("Execute() = (%#v, %v)", response, err)
	}
	if recorder.calls.Load() != 1 {
		t.Fatalf("usage recorder calls = %d, want 1", recorder.calls.Load())
	}
}

// verifiedTransaction creates a transaction ready for its forwarding claim.
func verifiedTransaction(t *testing.T) transactions.Transaction {
	t.Helper()

	createdAt := domain.NewTimestamp(
		time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	)
	transaction, err := transactions.NewTransaction(
		transactions.TransactionParams{
			TransactionID: mustProxyID(
				t,
				"txn_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				domain.TransactionIDPrefix,
			),
			IntentID: mustProxyID(
				t,
				"int_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				domain.IntentIDPrefix,
			),
			SellerID: mustProxyID(
				t,
				"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				domain.SellerIDPrefix,
			),
			RouteID: mustProxyID(
				t,
				"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				domain.RouteIDPrefix,
			),
			BuyerID:   "agent-123",
			Amount:    domain.MustParseAmount("10000"),
			Asset:     "test-usdc",
			Network:   "test-network",
			CreatedAt: createdAt,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.RequirePayment(createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	digest, err := intents.ParseSHA256Digest(strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.VerifyPayment(
		"x402_payment",
		digest,
		createdAt.Add(2*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if err := transaction.FinalizePayment(
		"x402_payment",
		"0xtestnettransaction",
		createdAt.Add(3*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	return transaction
}

// validExecutionRequest creates a complete authorized forwarding request.
func validExecutionRequest(
	t *testing.T,
	transaction transactions.Transaction,
) ExecutionRequest {
	t.Helper()

	seller := catalog.Seller{
		SellerID:         transaction.SellerID(),
		UpstreamBaseURL:  "https://seller.example",
		SigningSecretRef: "secret/seller",
		Status:           catalog.SellerStatusActive,
	}
	route := catalog.PaidRoute{
		RouteID:                transaction.RouteID(),
		SellerID:               transaction.SellerID(),
		Method:                 catalog.RouteMethodPost,
		PathPattern:            "/research",
		MIMEType:               "application/json",
		UpstreamTimeoutSeconds: 20,
		Enabled:                true,
	}
	return ExecutionRequest{
		Transaction: transaction,
		Seller:      seller,
		Route:       route,
		Method:      route.Method,
		Path:        route.PathPattern,
		Body:        []byte(`{"topic":"payments"}`),
		ContentType: "application/json",
	}
}

type recordingRequestSigner struct {
	calls atomic.Int32
}

// Sign records one post-claim signing operation.
func (signer *recordingRequestSigner) Sign(
	context.Context,
	string,
	SigningInput,
) (SignatureHeaders, error) {
	signer.calls.Add(1)
	return SignatureHeaders{
		Signature:   "signature",
		Timestamp:   "2026-09-17T10:01:00Z",
		Transaction: "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7",
	}, nil
}

type recordingForwarder struct {
	calls atomic.Int32
}

type recordingLifecycleRecorder struct {
	forwarding atomic.Int32
	delivery   atomic.Int32
}

type failingUsageRecorder struct {
	calls atomic.Int32
}

// RecordSuccessfulTransactionUsage records a call and simulates billing unavailability.
func (recorder *failingUsageRecorder) RecordSuccessfulTransactionUsage(
	_ context.Context,
	_ domain.ID,
) error {
	recorder.calls.Add(1)
	return errors.New("billing unavailable")
}

// RecordProxyForwarding records the pre-forward evidence call.
func (recorder *recordingLifecycleRecorder) RecordProxyForwarding(
	context.Context,
	domain.ID,
	evidence.ProxyForwardingFacts,
) error {
	recorder.forwarding.Add(1)
	return nil
}

// RecordDelivery records the post-forward evidence call.
func (recorder *recordingLifecycleRecorder) RecordDelivery(
	context.Context,
	domain.ID,
	evidence.DeliveryFacts,
) error {
	recorder.delivery.Add(1)
	return nil
}

// Forward records one seller invocation.
func (forwarder *recordingForwarder) Forward(
	context.Context,
	ForwardRequest,
) (ForwardResponse, error) {
	forwarder.calls.Add(1)
	return ForwardResponse{
		StatusCode:  200,
		Body:        []byte(`{"ok":true}`),
		ContentType: "application/json",
	}, nil
}

// mustProxyID parses a stable prefixed test identifier.
func mustProxyID(
	t *testing.T,
	raw string,
	prefix domain.IDPrefix,
) domain.ID {
	t.Helper()

	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
