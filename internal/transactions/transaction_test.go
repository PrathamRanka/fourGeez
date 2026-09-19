package transactions

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

func TestTransactionWithoutApprovalCompletes(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	createdAt := transaction.UpdatedAt()
	proofHash := mustTransactionDigest(t, strings.Repeat("a", 64))
	responseHash := mustTransactionDigest(t, strings.Repeat("b", 64))

	steps := []struct {
		name string
		run  func() error
		want TransactionStatus
	}{
		{name: "payment required", run: func() error { return transaction.RequirePayment(createdAt.Add(time.Second)) }, want: StatusPaymentRequired},
		{name: "payment verified", run: func() error { return transaction.VerifyPayment("payment-123", proofHash, createdAt.Add(2*time.Second)) }, want: StatusPaymentVerified},
		{name: "payment finalized", run: func() error {
			return transaction.FinalizePayment("payment-123", "0xtestnettransaction", createdAt.Add(3*time.Second))
		}, want: StatusPaymentVerified},
		{name: "forwarded", run: func() error { return transaction.MarkForwarded(createdAt.Add(4 * time.Second)) }, want: StatusForwarded},
		{name: "fulfilled", run: func() error {
			return transaction.MarkFulfilled(200, responseHash, ResponseSummary{ContentType: "application/json", ContentLength: 42}, createdAt.Add(5*time.Second))
		}, want: StatusFulfilled},
	}

	for _, step := range steps {
		if err := step.run(); err != nil {
			t.Fatalf("%s error = %v", step.name, err)
		}
		if transaction.Status() != step.want {
			t.Fatalf("%s status = %q, want %q", step.name, transaction.Status(), step.want)
		}
	}

	if transaction.PaymentIdentifier() != "payment-123" || transaction.PaymentProofHash() != proofHash {
		t.Fatal("payment evidence was not retained")
	}
	if transaction.UpstreamStatus() == nil || *transaction.UpstreamStatus() != 200 || transaction.ResponseHash() == nil || *transaction.ResponseHash() != responseHash {
		t.Fatal("delivery evidence was not retained")
	}
	if transaction.Version() != 6 {
		t.Fatalf("Version() = %d, want 6", transaction.Version())
	}
}

func TestTransactionWithApprovalAndDisputeCompletes(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	createdAt := transaction.UpdatedAt()

	transitions := []struct {
		status TransactionStatus
		run    func() error
	}{
		{status: StatusApprovalPending, run: func() error { return transaction.RequireApproval(createdAt.Add(time.Second)) }},
		{status: StatusApproved, run: func() error { return transaction.MarkApproved(createdAt.Add(2 * time.Second)) }},
		{status: StatusPaymentRequired, run: func() error { return transaction.RequirePayment(createdAt.Add(3 * time.Second)) }},
		{status: StatusPaymentVerified, run: func() error {
			return transaction.VerifyPayment("payment-123", mustTransactionDigest(t, strings.Repeat("a", 64)), createdAt.Add(4*time.Second))
		}},
		{status: StatusPaymentVerified, run: func() error {
			return transaction.FinalizePayment("payment-123", "0xtestnettransaction", createdAt.Add(5*time.Second))
		}},
		{status: StatusForwarded, run: func() error { return transaction.MarkForwarded(createdAt.Add(6 * time.Second)) }},
		{status: StatusFailed, run: func() error { return transaction.MarkFailed("seller_timeout", nil, nil, createdAt.Add(7*time.Second)) }},
		{status: StatusDisputed, run: func() error { return transaction.OpenDispute(createdAt.Add(8 * time.Second)) }},
		{status: StatusRefundRecommended, run: func() error { return transaction.RecommendRefund(createdAt.Add(9 * time.Second)) }},
		{status: StatusResolved, run: func() error { return transaction.Resolve(createdAt.Add(10 * time.Second)) }},
	}

	for _, transition := range transitions {
		if err := transition.run(); err != nil {
			t.Fatalf("transition to %q error = %v", transition.status, err)
		}
		if transaction.Status() != transition.status {
			t.Fatalf("Status() = %q, want %q", transaction.Status(), transition.status)
		}
	}
}

func TestTransactionRejectsForwardingBeforePaymentFinality(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	createdAt := transaction.UpdatedAt()
	if err := transaction.RequirePayment(createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.VerifyPayment(
		"payment-123",
		mustTransactionDigest(t, strings.Repeat("a", 64)),
		createdAt.Add(2*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	version := transaction.Version()

	err := transaction.MarkForwarded(createdAt.Add(3 * time.Second))
	var transitionError InvalidTransitionError
	if !errors.As(err, &transitionError) {
		t.Fatalf("MarkForwarded() error = %v, want InvalidTransitionError", err)
	}
	if transaction.Status() != StatusPaymentVerified || transaction.Version() != version {
		t.Fatal("non-final payment forwarding attempt mutated the transaction")
	}
}

func TestTransactionRejectsInvalidTransitionWithoutMutation(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	version := transaction.Version()
	updatedAt := transaction.UpdatedAt()

	err := transaction.MarkForwarded(updatedAt.Add(time.Second))
	var transitionError InvalidTransitionError
	if !errors.As(err, &transitionError) {
		t.Fatalf("error = %v, want InvalidTransitionError", err)
	}
	if transitionError.From != StatusProposed || transitionError.To != StatusForwarded {
		t.Fatalf("transition error = %#v", transitionError)
	}
	if transaction.Status() != StatusProposed || transaction.Version() != version || transaction.UpdatedAt() != updatedAt {
		t.Fatal("invalid transition mutated transaction")
	}
}

func TestTransactionRejectsNonChronologicalTransition(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	err := transaction.RequirePayment(transaction.UpdatedAt().Add(-time.Second))
	assertTransactionValidationField(t, err, "updatedAt")
	if transaction.Status() != StatusProposed {
		t.Fatal("non-chronological transition mutated status")
	}
}

func TestVerifyPaymentValidatesEvidence(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	if err := transaction.RequirePayment(transaction.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatalf("RequirePayment() error = %v", err)
	}
	transitionAt := transaction.UpdatedAt().Add(time.Second)

	if err := transaction.VerifyPayment("", mustTransactionDigest(t, strings.Repeat("a", 64)), transitionAt); err == nil {
		t.Fatal("VerifyPayment() accepted empty payment identifier")
	}
	if transaction.Status() != StatusPaymentRequired {
		t.Fatal("failed verification mutated status")
	}
}

// TestTransactionReconciliationDistinguishesPaymentAndDeliveryStages verifies PAY-010 reporting.
func TestTransactionReconciliationDistinguishesPaymentAndDeliveryStages(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	createdAt := transaction.UpdatedAt()
	proofHash := mustTransactionDigest(t, strings.Repeat("a", 64))
	responseHash := mustTransactionDigest(t, strings.Repeat("b", 64))
	if err := transaction.RequirePayment(createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if transaction.Reconciliation().Stage != ReconciliationStageChallenged {
		t.Fatalf("challenge reconciliation = %#v", transaction.Reconciliation())
	}
	if err := transaction.VerifyPayment(
		"payment-123",
		proofHash,
		createdAt.Add(2*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if transaction.PaymentFinality() != PaymentFinalityConfirmed ||
		transaction.Reconciliation().Stage != ReconciliationStageVerified {
		t.Fatalf("verified reconciliation = %#v", transaction.Reconciliation())
	}
	if err := transaction.FinalizePayment(
		"payment-123",
		"0xtestnettransaction",
		createdAt.Add(3*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if transaction.PaymentFinality() != PaymentFinalityFinalized ||
		transaction.PaymentReference() != "0xtestnettransaction" ||
		transaction.Reconciliation().Stage != ReconciliationStageFinalized {
		t.Fatalf("finalized reconciliation = %#v", transaction.Reconciliation())
	}
	if err := transaction.MarkForwarded(createdAt.Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkFulfilled(
		200,
		responseHash,
		ResponseSummary{ContentType: "application/json", ContentLength: 42},
		createdAt.Add(5*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if transaction.Reconciliation().Stage != ReconciliationStageFulfilled {
		t.Fatalf("fulfilled reconciliation = %#v", transaction.Reconciliation())
	}
	if err := transaction.OpenDispute(createdAt.Add(6 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if transaction.Reconciliation().Stage != ReconciliationStageDisputed {
		t.Fatalf("disputed reconciliation = %#v", transaction.Reconciliation())
	}
}

// TestTransactionRejectsMismatchedPaymentFinalization protects proof identity.
func TestTransactionRejectsMismatchedPaymentFinalization(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	createdAt := transaction.UpdatedAt()
	if err := transaction.RequirePayment(createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.VerifyPayment(
		"payment-123",
		mustTransactionDigest(t, strings.Repeat("a", 64)),
		createdAt.Add(2*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	version := transaction.Version()
	err := transaction.FinalizePayment(
		"different-payment",
		"0xtestnettransaction",
		createdAt.Add(3*time.Second),
	)
	assertTransactionValidationField(t, err, "paymentIdentifier")
	if transaction.Version() != version ||
		transaction.PaymentFinality() != PaymentFinalityConfirmed {
		t.Fatal("rejected finalization mutated the transaction")
	}
}

// TestTransactionRecordsRejectedSettlementAsFailed verifies failed payment reconciliation.
func TestTransactionRecordsRejectedSettlementAsFailed(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	createdAt := transaction.UpdatedAt()
	if err := transaction.RequirePayment(createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.VerifyPayment(
		"payment-123",
		mustTransactionDigest(t, strings.Repeat("a", 64)),
		createdAt.Add(2*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if err := transaction.FailPayment(
		"settlement_rejected",
		"",
		createdAt.Add(3*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if transaction.Status() != StatusFailed ||
		transaction.PaymentFinality() != PaymentFinalityFailed ||
		transaction.Reconciliation().Stage != ReconciliationStageFailed {
		t.Fatalf("failed reconciliation = %#v", transaction.Reconciliation())
	}
}

func TestTransactionCommerceLifecycleAndPriceProjection(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	createdAt := transaction.UpdatedAt()
	assertLifecycle := func(commerce, payment, fulfillment, refund, recovery string) {
		t.Helper()
		projection := transaction.CommerceLifecycle(false)
		if string(projection.CommerceState) != commerce || string(projection.PaymentState) != payment ||
			string(projection.FulfillmentState) != fulfillment || string(projection.RefundState) != refund ||
			string(projection.RecoveryAction) != recovery {
			t.Fatalf("projection = %#v", projection)
		}
	}

	if breakdown := transaction.PriceBreakdown(); breakdown.Total != transaction.Amount() ||
		breakdown.Subtotal != transaction.Amount() || !breakdown.Adjustments.IsZero() || breakdown.Quantity != 1 {
		t.Fatalf("price breakdown = %#v", breakdown)
	}
	assertLifecycle("awaiting_payment", "pending", "not_started", "not_requested", "retry_same_request")
	if err := transaction.RequirePayment(createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.VerifyPayment("payment-123", mustTransactionDigest(t, strings.Repeat("a", 64)), createdAt.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	assertLifecycle("payment_processing", "confirmed", "not_started", "not_requested", "await_reconciliation")
	if err := transaction.FinalizePayment("payment-123", "0xpayment", createdAt.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkForwarded(createdAt.Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkFailed("seller_timeout", nil, nil, createdAt.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	assertLifecycle("failed", "finalized", "failed", "not_requested", "open_dispute")
	if err := transaction.OpenDispute(createdAt.Add(6 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.RecommendRefund(createdAt.Add(7 * time.Second)); err != nil {
		t.Fatal(err)
	}
	assertLifecycle("refund_recommended", "finalized", "failed", "recommended", "record_external_refund")
	projection := transaction.CommerceLifecycle(true)
	if projection.RefundState != RefundStateSellerReported || projection.RecoveryAction != RecoveryActionNone {
		t.Fatalf("seller-reported projection = %#v", projection)
	}
}

func TestNewTransactionValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*TransactionParams)
		wantField string
	}{
		{name: "transaction ID", mutate: func(params *TransactionParams) {
			params.TransactionID = mustTransactionID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
		}, wantField: "transactionId"},
		{name: "intent ID", mutate: func(params *TransactionParams) {
			params.IntentID = mustTransactionID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
		}, wantField: "intentId"},
		{name: "seller ID", mutate: func(params *TransactionParams) {
			params.SellerID = mustTransactionID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
		}, wantField: "sellerId"},
		{name: "route ID", mutate: func(params *TransactionParams) {
			params.RouteID = mustTransactionID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
		}, wantField: "routeId"},
		{name: "buyer required", mutate: func(params *TransactionParams) { params.BuyerID = "" }, wantField: "buyerId"},
		{name: "amount positive", mutate: func(params *TransactionParams) { params.Amount = domain.MustParseAmount("0") }, wantField: "amount"},
		{name: "asset required", mutate: func(params *TransactionParams) { params.Asset = "" }, wantField: "asset"},
		{name: "network required", mutate: func(params *TransactionParams) { params.Network = "" }, wantField: "network"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			params := validTransactionParams(t)
			test.mutate(&params)
			_, err := NewTransaction(params)
			assertTransactionValidationField(t, err, test.wantField)
		})
	}
}

func TestTransactionPreservesLeanV1CommerceSnapshot(t *testing.T) {
	t.Parallel()

	params := validTransactionParams(t)
	params.ProductDisplayName = "Research Report"
	params.ProductSlug = "research-report"
	params.PaymentDestinationID = mustTransactionID(t, "dst_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.PaymentDestinationIDPrefix)
	params.PurchaseChannel = PurchaseChannelBrowser
	params.PurchaseSessionID = "bps_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	params.PaymentRail = PaymentRailX402
	transaction, err := NewTransaction(params)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Restore(transaction.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if restored.ProductDisplayName() != params.ProductDisplayName ||
		restored.ProductSlug() != params.ProductSlug ||
		restored.PaymentDestinationID() != params.PaymentDestinationID ||
		restored.PurchaseChannel() != PurchaseChannelBrowser ||
		restored.PurchaseSessionID() != params.PurchaseSessionID ||
		restored.PaymentRail() != PaymentRailX402 {
		t.Fatalf("restored commerce snapshot = %#v", restored.Snapshot())
	}
	if receiptTransaction(restored).ProductSlug != params.ProductSlug {
		t.Fatal("receipt omitted immutable product snapshot")
	}
	if receiptSchemaVersion(restored) != purchaseReceiptSchemaVersionV2 {
		t.Fatalf("receipt schema = %q, want v2", receiptSchemaVersion(restored))
	}
	if receiptSchemaVersion(*newTestTransaction(t)) != purchaseReceiptSchemaVersionV1 {
		t.Fatal("legacy transaction did not retain receipt schema v1")
	}
}

func newTestTransaction(t *testing.T) *Transaction {
	t.Helper()
	transaction, err := NewTransaction(validTransactionParams(t))
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	return &transaction
}

func validTransactionParams(t *testing.T) TransactionParams {
	t.Helper()
	return TransactionParams{
		TransactionID: mustTransactionID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		IntentID:      mustTransactionID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:      mustTransactionID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		RouteID:       mustTransactionID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:       "agent_demo_123",
		Amount:        domain.MustParseAmount("35000000"),
		Asset:         "test-usdc",
		Network:       "test-network",
		CreatedAt:     domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)),
	}
}

func mustTransactionID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatalf("ParseID(%q) error = %v", raw, err)
	}
	return identifier
}

func mustTransactionDigest(t *testing.T, raw string) intents.SHA256Digest {
	t.Helper()
	digest, err := intents.ParseSHA256Digest(raw)
	if err != nil {
		t.Fatalf("ParseSHA256Digest() error = %v", err)
	}
	return digest
}

func assertTransactionValidationField(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error for %q", field)
	}
	var validationErrors domain.ValidationErrors
	if errors.As(err, &validationErrors) && validationErrors.HasField(field) {
		return
	}
	var validationError domain.ValidationError
	if errors.As(err, &validationError) && validationError.Field == field {
		return
	}
	t.Fatalf("error = %#v, want validation error for %q", err, field)
}
