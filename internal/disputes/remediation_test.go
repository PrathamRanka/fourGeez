package disputes

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/transactions"
)

func TestRecordManualRefundIsBoundedAndIdempotent(t *testing.T) {
	t.Parallel()

	dispute := mustRefundDispute(t)
	transaction := mustRefundTransaction(t)
	repository := &memoryRefundRecordRepository{}
	service := NewManualRemediationService(
		&refundDisputeRepository{dispute: dispute},
		&refundTransactionRepository{transaction: transaction},
		repository,
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 5, 0, 0, time.UTC)},
	)
	request := RecordManualRefundRequest{
		DisputeID: dispute.DisputeID, SellerID: transaction.SellerID(), Amount: transaction.Amount(), Asset: transaction.Asset(),
		Network: transaction.Network(), Reference: "0xrefund-reference", RecordedBy: "operator@example.com",
	}
	first, err := service.RecordManualRefund(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.RecordManualRefund(t.Context(), request)
	if err != nil || second != first {
		t.Fatalf("idempotent replay = (%#v, %v)", second, err)
	}
	changed := request
	changed.Reference = "0xdifferent"
	if _, err := service.RecordManualRefund(t.Context(), changed); !errors.Is(err, ErrRemediationConflict) {
		t.Fatalf("changed replay error = %v", err)
	}
}

func TestRecordManualRefundRejectsUnboundedOrUnfinalizedRefunds(t *testing.T) {
	t.Parallel()

	dispute := mustRefundDispute(t)
	finalized := mustRefundTransaction(t)
	confirmedSnapshot := finalized.Snapshot()
	confirmedSnapshot.PaymentFinality = transactions.PaymentFinalityConfirmed
	confirmed, err := transactions.Restore(confirmedSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		transaction transactions.Transaction
		mutate      func(*RecordManualRefundRequest)
	}{
		{name: "unfinalized", transaction: confirmed},
		{name: "wrong amount", transaction: finalized, mutate: func(request *RecordManualRefundRequest) { request.Amount = domain.MustParseAmount("101") }},
		{name: "wrong asset", transaction: finalized, mutate: func(request *RecordManualRefundRequest) { request.Asset = "OTHER" }},
		{name: "wrong network", transaction: finalized, mutate: func(request *RecordManualRefundRequest) { request.Network = "eip155:1" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := RecordManualRefundRequest{
				DisputeID: dispute.DisputeID, SellerID: finalized.SellerID(), Amount: finalized.Amount(), Asset: finalized.Asset(),
				Network: finalized.Network(), Reference: "0xrefund-reference", RecordedBy: "operator@example.com",
			}
			if test.mutate != nil {
				test.mutate(&request)
			}
			service := NewManualRemediationService(
				&refundDisputeRepository{dispute: dispute},
				&refundTransactionRepository{transaction: test.transaction},
				&memoryRefundRecordRepository{},
				domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 5, 0, 0, time.UTC)},
			)
			if _, err := service.RecordManualRefund(t.Context(), request); !errors.Is(err, ErrRefundNotAllowed) {
				t.Fatalf("RecordManualRefund() error = %v", err)
			}
		})
	}
}

func TestRecordManualRefundRejectsNonRecommendedDisputeAsStateConflict(t *testing.T) {
	t.Parallel()

	dispute := mustRefundDispute(t)
	dispute.Status = StatusOpen
	transaction := mustRefundTransaction(t)
	service := NewManualRemediationService(
		&refundDisputeRepository{dispute: dispute},
		&refundTransactionRepository{transaction: transaction},
		&memoryRefundRecordRepository{},
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 5, 0, 0, time.UTC)},
	)
	_, err := service.RecordManualRefund(t.Context(), RecordManualRefundRequest{
		DisputeID: dispute.DisputeID, SellerID: transaction.SellerID(), Amount: transaction.Amount(),
		Asset: transaction.Asset(), Network: transaction.Network(), Reference: "0xrefund-reference",
		RecordedBy: "operator@example.com",
	})
	if !errors.Is(err, ErrRemediationStateConflict) {
		t.Fatalf("RecordManualRefund() error = %v, want state conflict", err)
	}
}

func TestRecordManualRefundConcealsCrossSellerDispute(t *testing.T) {
	t.Parallel()
	dispute := mustRefundDispute(t)
	transaction := mustRefundTransaction(t)
	service := NewManualRemediationService(
		&refundDisputeRepository{dispute: dispute},
		&refundTransactionRepository{transaction: transaction},
		&memoryRefundRecordRepository{},
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 5, 0, 0, time.UTC)},
	)
	request := RecordManualRefundRequest{
		DisputeID: dispute.DisputeID,
		SellerID:  mustDisputeID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9HZ", domain.SellerIDPrefix),
		Amount:    transaction.Amount(), Asset: transaction.Asset(), Network: transaction.Network(),
		Reference: "0xrefund-reference", RecordedBy: "operator@example.com",
	}
	if _, err := service.RecordManualRefund(t.Context(), request); !errors.Is(err, ErrRemediationAccess) {
		t.Fatalf("RecordManualRefund() error = %v, want concealed access error", err)
	}
}

func TestGetManualRefundReturnsSellerReportedRecordToOwningSeller(t *testing.T) {
	t.Parallel()

	dispute := mustRefundDispute(t)
	transaction := mustRefundTransaction(t)
	repository := &memoryRefundRecordRepository{}
	service := NewManualRemediationService(
		&refundDisputeRepository{dispute: dispute},
		&refundTransactionRepository{transaction: transaction},
		repository,
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 5, 0, 0, time.UTC)},
	)
	request := RecordManualRefundRequest{
		DisputeID: dispute.DisputeID, SellerID: transaction.SellerID(), Amount: transaction.Amount(), Asset: transaction.Asset(),
		Network: transaction.Network(), Reference: "0xrefund-reference", RecordedBy: "operator@example.com",
	}
	recorded, err := service.RecordManualRefund(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := service.GetManualRefund(t.Context(), dispute.DisputeID, transaction.SellerID())
	if err != nil {
		t.Fatal(err)
	}
	if loaded != recorded {
		t.Fatalf("GetManualRefund() = %#v, want %#v", loaded, recorded)
	}
	if loaded.VerificationState != RefundVerificationSellerReported {
		t.Fatalf("verification state = %q, want %q", loaded.VerificationState, RefundVerificationSellerReported)
	}

	otherSellerID := mustDisputeID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9HZ", domain.SellerIDPrefix)
	if _, err := service.GetManualRefund(t.Context(), dispute.DisputeID, otherSellerID); !errors.Is(err, ErrRemediationAccess) {
		t.Fatalf("cross-seller GetManualRefund() error = %v, want concealed access error", err)
	}
}

type refundDisputeRepository struct{ dispute Dispute }

func (repository *refundDisputeRepository) Create(context.Context, Dispute) error { return nil }
func (repository *refundDisputeRepository) Get(context.Context, domain.ID) (Dispute, error) {
	return repository.dispute, nil
}

type refundTransactionRepository struct{ transaction transactions.Transaction }

func (repository *refundTransactionRepository) Get(context.Context, domain.ID) (transactions.Transaction, error) {
	return repository.transaction, nil
}
func (repository *refundTransactionRepository) Update(context.Context, transactions.Transaction, uint64) error {
	return nil
}

type memoryRefundRecordRepository struct{ record *ManualRefundRecord }

func (repository *memoryRefundRecordRepository) SaveIfAbsent(_ context.Context, record ManualRefundRecord) (ManualRefundRecord, bool, error) {
	if repository.record != nil {
		return *repository.record, false, nil
	}
	repository.record = &record
	return record, true, nil
}

func (repository *memoryRefundRecordRepository) Get(_ context.Context, disputeID domain.ID) (ManualRefundRecord, error) {
	if repository.record == nil || repository.record.DisputeID != disputeID {
		return ManualRefundRecord{}, persistence.ErrNotFound
	}
	return *repository.record, nil
}

func mustRefundDispute(t *testing.T) Dispute {
	t.Helper()
	delivery := false
	dispute, err := Classify(validDisputeParams(t, ReasonNotDelivered), Facts{DeliverySucceeded: &delivery})
	if err != nil {
		t.Fatal(err)
	}
	return dispute
}

func mustRefundTransaction(t *testing.T) transactions.Transaction {
	t.Helper()
	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	transaction, err := transactions.NewTransaction(transactions.TransactionParams{
		TransactionID: mustDisputeID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		IntentID:      mustDisputeID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:      mustDisputeID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		RouteID:       mustDisputeID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:       "buyer", Amount: domain.MustParseAmount("100"), Asset: "USDC", Network: "eip155:84532", CreatedAt: createdAt,
	})
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
	if err := transaction.VerifyPayment("payment-refund", digest, createdAt.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.FinalizePayment("payment-refund", "0xpayment", createdAt.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	return transaction
}
