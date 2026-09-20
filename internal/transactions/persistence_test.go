package transactions

import (
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

// TestTransactionSnapshotRoundTripsPaymentReconciliation verifies PAY-010 persistence.
func TestTransactionSnapshotRoundTripsPaymentReconciliation(t *testing.T) {
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
	if err := transaction.FinalizePayment(
		"payment-123",
		"0xtestnettransaction",
		createdAt.Add(3*time.Second),
	); err != nil {
		t.Fatal(err)
	}

	restored, err := Restore(transaction.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if restored.PaymentFinality() != PaymentFinalityFinalized ||
		restored.PaymentReference() != transaction.PaymentReference() ||
		restored.ReconciledAt() == nil ||
		restored.ReconciledAt().String() != transaction.ReconciledAt().String() {
		t.Fatalf("restored reconciliation = %#v", restored.Reconciliation())
	}
}

// TestTransactionRestoreConservativelyMigratesLegacyVerifiedPayment covers old records.
func TestTransactionRestoreConservativelyMigratesLegacyVerifiedPayment(t *testing.T) {
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
	snapshot := transaction.Snapshot()
	snapshot.PaymentFinality = ""
	snapshot.ReconciledAt = nil

	restored, err := Restore(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if restored.PaymentFinality() != PaymentFinalityConfirmed ||
		restored.PaymentReference() != "" {
		t.Fatalf("legacy reconciliation = %#v", restored.Reconciliation())
	}
}

func TestTransactionRestoreClassifiesLegacyActivityAsTest(t *testing.T) {
	t.Parallel()

	transaction := newTestTransaction(t)
	snapshot := transaction.Snapshot()
	snapshot.ActivityMode = ""
	snapshot.CheckoutExpiresAt = domain.Timestamp{}

	restored, err := Restore(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ActivityMode() != ActivityModeTest ||
		restored.CheckoutExpiresAt() != restored.CreatedAt().Add(defaultCheckoutLifetime) {
		t.Fatalf("legacy activity = %#v", restored.Snapshot())
	}
}
