package transactions

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestUnpaidCheckoutBecomesAbandonedAtFrozenDeadline(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	createdAt := transaction.CreatedAt()
	if err := transaction.RequirePayment(createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	beforeExpiry := transaction.CommerceLifecycleAt(
		transaction.CheckoutExpiresAt().Add(-time.Nanosecond),
		false,
	)
	if beforeExpiry.CommerceState != CommerceStateAwaitingPayment ||
		beforeExpiry.PaymentState != PaymentStatePending ||
		transaction.SellerOutcomeAt(transaction.CheckoutExpiresAt().Add(-time.Nanosecond)) != SellerOutcomeAwaitingPayment {
		t.Fatalf("before-expiry projection = %#v", beforeExpiry)
	}

	atExpiry := transaction.CommerceLifecycleAt(transaction.CheckoutExpiresAt(), false)
	if atExpiry.CommerceState != CommerceStateAbandoned ||
		atExpiry.PaymentState != PaymentStateExpired ||
		atExpiry.FulfillmentState != FulfillmentStateNotStarted ||
		atExpiry.RecoveryAction != RecoveryActionCreateNewIntent ||
		transaction.SellerOutcomeAt(transaction.CheckoutExpiresAt()) != SellerOutcomeAbandoned {
		t.Fatalf("expired projection = %#v", atExpiry)
	}
	if transaction.Status() != StatusPaymentRequired {
		t.Fatalf("derived classification mutated status to %q", transaction.Status())
	}
	if response := transactionResponse(*transaction, transaction.CheckoutExpiresAt()); response.Reconciliation != nil {
		t.Fatalf("abandoned checkout entered sales reconciliation: %#v", response.Reconciliation)
	}
}

func TestSellerOutcomeSeparatesPaymentAndFulfillmentFailures(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 20, 9, 0, 0, 0, time.UTC))
	paymentRejected := newTestTransaction(t)
	if err := paymentRejected.RequirePayment(createdAt); err != nil {
		t.Fatal(err)
	}
	if err := paymentRejected.VerifyPayment("payment-rejected", mustTransactionDigest(t, strings.Repeat("a", 64)), createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := paymentRejected.FailPayment("facilitator_rejected", "", createdAt.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := paymentRejected.SellerOutcomeAt(createdAt.Add(3 * time.Second)); got != SellerOutcomePaymentRejected {
		t.Fatalf("payment outcome = %q", got)
	}

	fulfillmentFailed := newTestTransaction(t)
	if err := fulfillmentFailed.RequirePayment(createdAt); err != nil {
		t.Fatal(err)
	}
	if err := fulfillmentFailed.VerifyPayment("payment-finalized", mustTransactionDigest(t, strings.Repeat("b", 64)), createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := fulfillmentFailed.FinalizePayment("payment-finalized", "0xfinalized", createdAt.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := fulfillmentFailed.MarkForwarded(createdAt.Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := fulfillmentFailed.MarkFailed("seller_timeout", nil, nil, createdAt.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	projection := fulfillmentFailed.CommerceLifecycleAt(createdAt.Add(5*time.Second), false)
	if got := fulfillmentFailed.SellerOutcomeAt(createdAt.Add(5 * time.Second)); got != SellerOutcomeFulfillmentFailed {
		t.Fatalf("fulfillment outcome = %q", got)
	}
	if projection.PaymentState != PaymentStateFinalized || projection.FulfillmentState != FulfillmentStateFailed {
		t.Fatalf("fulfillment projection = %#v", projection)
	}
}

func TestSellerTransactionQueryFiltersActivityAndOutcome(t *testing.T) {
	t.Parallel()

	sellerID := mustTransactionID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	query, err := ParseSellerTransactionQuery(sellerID, url.Values{
		"activityMode": []string{"test"},
		"outcome":      []string{"abandoned"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	transaction := newTestTransaction(t)
	if err := transaction.RequirePayment(transaction.CreatedAt().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	query.AsOf = transaction.CheckoutExpiresAt()
	if !query.Matches(*transaction) {
		t.Fatalf("query did not match %#v", transaction.Snapshot())
	}
	query.ActivityMode = ActivityModeLive
	if query.Matches(*transaction) {
		t.Fatal("live filter included test activity")
	}
}

func TestTransactionLifecycleCompletesThreeConsecutiveRunsDeterministically(t *testing.T) {
	t.Parallel()

	for run := 0; run < 3; run++ {
		transaction := newTestTransaction(t)
		createdAt := transaction.CreatedAt().Add(time.Duration(run) * time.Hour)
		if err := transaction.RequirePayment(createdAt.Add(time.Second)); err != nil {
			t.Fatalf("run %d require payment: %v", run+1, err)
		}
		paymentIdentifier := "payment-run-" + string(rune('1'+run))
		if err := transaction.VerifyPayment(paymentIdentifier, mustTransactionDigest(t, strings.Repeat(string(rune('a'+run)), 64)), createdAt.Add(2*time.Second)); err != nil {
			t.Fatalf("run %d verify: %v", run+1, err)
		}
		if err := transaction.FinalizePayment(paymentIdentifier, "0xrun", createdAt.Add(3*time.Second)); err != nil {
			t.Fatalf("run %d finalize: %v", run+1, err)
		}
		if err := transaction.MarkForwarded(createdAt.Add(4 * time.Second)); err != nil {
			t.Fatalf("run %d forward: %v", run+1, err)
		}
		if err := transaction.MarkFulfilled(200, mustTransactionDigest(t, strings.Repeat("f", 64)), ResponseSummary{ContentType: "application/json", ContentLength: 2}, createdAt.Add(5*time.Second)); err != nil {
			t.Fatalf("run %d fulfill: %v", run+1, err)
		}
		if transaction.SellerOutcomeAt(createdAt.Add(6*time.Second)) != SellerOutcomeFulfilled || transaction.Version() != 6 {
			t.Fatalf("run %d final snapshot = %#v", run+1, transaction.Snapshot())
		}
	}
}
