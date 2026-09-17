package billing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// TestUsageServiceMetersSuccessfulTransactionExactlyOnce verifies BIL-002 facts.
func TestUsageServiceMetersSuccessfulTransactionExactlyOnce(t *testing.T) {
	t.Parallel()

	occurredAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	transaction := fulfilledBillingTransaction(t, occurredAt)
	planRepository := newBillingRepository()
	planService := NewService(
		planRepository,
		billingSellerAuthorizer{sellerID: transaction.SellerID()},
		domain.FixedClock{Value: occurredAt.Time()},
	)
	if _, err := planService.GetSellerPlan(
		context.Background(),
		"seller-user",
		transaction.SellerID(),
	); err != nil {
		t.Fatal(err)
	}
	usageRepository := newUsageRepository()
	service := NewUsageService(
		usageRepository,
		planService,
		billingTransactionReader{transaction: transaction},
		billingSellerAuthorizer{sellerID: transaction.SellerID()},
		domain.NewULIDGenerator(
			domain.FixedClock{Value: occurredAt.Time()},
			strings.NewReader(strings.Repeat("m", 128)),
		),
		domain.FixedClock{Value: occurredAt.Time()},
	)

	first, err := service.RecordSuccessfulTransaction(
		context.Background(),
		transaction.TransactionID(),
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.RecordSuccessfulTransaction(
		context.Background(),
		transaction.TransactionID(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.MeterEventID() != second.MeterEventID() ||
		first.Quantity() != 1 ||
		len(usageRepository.events) != 1 {
		t.Fatalf("meter events = (%#v, %#v), stored = %d", first.Snapshot(), second.Snapshot(), len(usageRepository.events))
	}
}

// TestUsageServiceExportsPlanVersionedQuantities verifies invoice aggregation.
func TestUsageServiceExportsPlanVersionedQuantities(t *testing.T) {
	t.Parallel()

	occurredAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	transaction := fulfilledBillingTransaction(t, occurredAt)
	planRepository := newBillingRepository()
	planService := NewService(
		planRepository,
		billingSellerAuthorizer{sellerID: transaction.SellerID()},
		domain.FixedClock{Value: occurredAt.Time()},
	)
	if _, err := planService.GetSellerPlan(context.Background(), "seller-user", transaction.SellerID()); err != nil {
		t.Fatal(err)
	}
	usageRepository := newUsageRepository()
	service := NewUsageService(
		usageRepository,
		planService,
		billingTransactionReader{transaction: transaction},
		billingSellerAuthorizer{sellerID: transaction.SellerID()},
		domain.NewULIDGenerator(domain.FixedClock{Value: occurredAt.Time()}, strings.NewReader(strings.Repeat("n", 128))),
		domain.FixedClock{Value: occurredAt.Time()},
	)
	if _, err := service.RecordSuccessfulTransaction(context.Background(), transaction.TransactionID()); err != nil {
		t.Fatal(err)
	}
	export, err := service.ExportInvoice(
		context.Background(),
		"seller-user",
		transaction.SellerID(),
		domain.NewTimestamp(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)),
		domain.NewTimestamp(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(export.LineItems) != 1 ||
		export.LineItems[0].Quantity != 1 ||
		export.LineItems[0].PlanID != PlanStarter {
		t.Fatalf("invoice export = %#v", export)
	}
}

// TestUsageServiceRejectsInvoiceExportsAboveBound verifies no silent truncation.
func TestUsageServiceRejectsInvoiceExportsAboveBound(t *testing.T) {
	t.Parallel()

	occurredAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	transaction := fulfilledBillingTransaction(t, occurredAt)
	event, err := NewUsageMeterEvent(UsageMeterEventParams{
		MeterEventID:        mustBillingDomainID(t, "mtr_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.UsageMeterEventIDPrefix),
		SellerID:            transaction.SellerID(),
		MeterName:           MeterSuccessfulTransaction,
		Quantity:            1,
		SourceTransactionID: transaction.TransactionID(),
		PlanID:              PlanStarter,
		PlanVersion:         1,
		OccurredAt:          occurredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	repository := newUsageRepository()
	for index := 0; index <= maximumInvoiceEvents; index++ {
		repository.events[string(rune(index))] = event
	}
	service := NewUsageService(
		repository,
		nil,
		nil,
		billingSellerAuthorizer{sellerID: transaction.SellerID()},
		nil,
		domain.FixedClock{Value: occurredAt.Time()},
	)
	_, err = service.ExportInvoice(
		context.Background(),
		"seller-user",
		transaction.SellerID(),
		occurredAt.Add(-time.Hour),
		occurredAt.Add(time.Hour),
	)
	if !errors.Is(err, ErrUsageExportTooLarge) {
		t.Fatalf("ExportInvoice() error = %v, want ErrUsageExportTooLarge", err)
	}
}

// fulfilledBillingTransaction creates one finalized and fulfilled usage source.
func fulfilledBillingTransaction(t *testing.T, createdAt domain.Timestamp) transactions.Transaction {
	t.Helper()
	transaction, err := transactions.NewTransaction(transactions.TransactionParams{
		TransactionID: mustBillingDomainID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		IntentID:      mustBillingDomainID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:      mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		RouteID:       mustBillingDomainID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:       "agent-buyer",
		Amount:        domain.MustParseAmount("1000"),
		Asset:         "USDC",
		Network:       "eip155:84532",
		CreatedAt:     createdAt,
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
	if err := transaction.VerifyPayment("payment-1", digest, createdAt.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.FinalizePayment("payment-1", "0xreference", createdAt.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkForwarded(createdAt.Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkFulfilled(200, digest, transactions.ResponseSummary{ContentType: "application/json", ContentLength: 2}, createdAt.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	return transaction
}

type usageRepository struct {
	events map[string]UsageMeterEvent
}

// newUsageRepository creates an empty usage repository fixture.
func newUsageRepository() *usageRepository {
	return &usageRepository{events: make(map[string]UsageMeterEvent)}
}

// CreateIfAbsent stores one unique meter source.
func (repository *usageRepository) CreateIfAbsent(
	_ context.Context,
	event UsageMeterEvent,
) (UsageMeterEvent, bool, error) {
	key := string(event.MeterName()) + "\x00" + event.SourceTransactionID().String()
	if stored, exists := repository.events[key]; exists {
		return stored, false, nil
	}
	repository.events[key] = event
	return event, true, nil
}

// ListBySellerWindow returns usage in the requested UTC window.
func (repository *usageRepository) ListBySellerWindow(
	_ context.Context,
	sellerID domain.ID,
	from domain.Timestamp,
	to domain.Timestamp,
	_ int,
) ([]UsageMeterEvent, error) {
	result := make([]UsageMeterEvent, 0)
	for _, event := range repository.events {
		if event.SellerID() == sellerID &&
			!event.OccurredAt().Before(from) &&
			event.OccurredAt().Before(to) {
			result = append(result, event)
		}
	}
	return result, nil
}

type billingTransactionReader struct {
	transaction transactions.Transaction
}

// Get returns the configured transaction fixture.
func (reader billingTransactionReader) Get(
	_ context.Context,
	_ domain.ID,
) (transactions.Transaction, error) {
	return reader.transaction, nil
}

// mustBillingDomainID parses one billing test identifier.
func mustBillingDomainID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
