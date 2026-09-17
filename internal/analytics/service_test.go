package analytics

import (
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// TestServiceAggregatesByDayAssetNetworkRouteAndStage verifies ANL-001 grouping.
func TestServiceAggregatesByDayAssetNetworkRouteAndStage(t *testing.T) {
	t.Parallel()

	sellerID := mustAnalyticsID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	first := analyticsTransaction(
		t,
		"txn_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		sellerID,
		"100",
		"USDC",
		"eip155:84532",
		time.Date(2026, time.September, 17, 23, 59, 0, 0, time.UTC),
		transactions.ReconciliationStageFulfilled,
	)
	second := analyticsTransaction(
		t,
		"txn_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		sellerID,
		"250",
		"USDC",
		"eip155:84532",
		time.Date(2026, time.September, 17, 23, 59, 30, 0, time.UTC),
		transactions.ReconciliationStageFulfilled,
	)
	third := analyticsTransaction(
		t,
		"txn_01K5D09YJ0C0M7RJM4FWQ0K9H9",
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		sellerID,
		"75",
		"USDC",
		"eip155:84532",
		time.Date(2026, time.September, 18, 0, 1, 0, 0, time.UTC),
		transactions.ReconciliationStageFailed,
	)
	fourth := analyticsTransaction(
		t,
		"txn_01K5D09YJ0C0M7RJM4FWQ0K9HA",
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		sellerID,
		"9",
		"EURC",
		"eip155:84532",
		time.Date(2026, time.September, 17, 23, 59, 45, 0, time.UTC),
		transactions.ReconciliationStageFulfilled,
	)

	aggregates, err := NewService().AggregateSeller(
		sellerID,
		[]transactions.Transaction{first, second, third, fourth},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertAnalyticsAggregate(
		t,
		aggregates,
		"2026-09-17",
		"USDC",
		"eip155:84532",
		"",
		transactions.ReconciliationStageFulfilled,
		2,
		"350",
	)
	assertAnalyticsAggregate(
		t,
		aggregates,
		"2026-09-17",
		"USDC",
		"eip155:84532",
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		transactions.ReconciliationStageFulfilled,
		2,
		"350",
	)
	assertAnalyticsAggregate(
		t,
		aggregates,
		"2026-09-17",
		"EURC",
		"eip155:84532",
		"",
		transactions.ReconciliationStageFulfilled,
		1,
		"9",
	)
	assertAnalyticsAggregate(
		t,
		aggregates,
		"2026-09-18",
		"USDC",
		"eip155:84532",
		"",
		transactions.ReconciliationStageFailed,
		1,
		"75",
	)
}

// TestServiceDeduplicatesTransactionVersions verifies retry-safe aggregation.
func TestServiceDeduplicatesTransactionVersions(t *testing.T) {
	t.Parallel()

	sellerID := mustAnalyticsID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	challenged := analyticsTransaction(
		t,
		"txn_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		sellerID,
		"100",
		"USDC",
		"eip155:84532",
		time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
		transactions.ReconciliationStageChallenged,
	)
	fulfilled := analyticsTransaction(
		t,
		"txn_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		sellerID,
		"100",
		"USDC",
		"eip155:84532",
		time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
		transactions.ReconciliationStageFulfilled,
	)

	aggregates, err := NewService().AggregateSeller(
		sellerID,
		[]transactions.Transaction{challenged, fulfilled, fulfilled},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertAnalyticsAggregate(
		t,
		aggregates,
		"2026-09-17",
		"USDC",
		"eip155:84532",
		"",
		transactions.ReconciliationStageFulfilled,
		1,
		"100",
	)
}

// TestServiceRejectsCrossSellerTransactions protects tenant isolation.
func TestServiceRejectsCrossSellerTransactions(t *testing.T) {
	t.Parallel()

	sellerID := mustAnalyticsID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	otherSellerID := mustAnalyticsID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		domain.SellerIDPrefix,
	)
	transaction := analyticsTransaction(
		t,
		"txn_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		"rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		otherSellerID,
		"100",
		"USDC",
		"eip155:84532",
		time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
		transactions.ReconciliationStageFulfilled,
	)

	_, err := NewService().AggregateSeller(
		sellerID,
		[]transactions.Transaction{transaction},
	)
	if !errors.Is(err, ErrSellerScope) {
		t.Fatalf("AggregateSeller() error = %v, want seller scope error", err)
	}
}

// analyticsTransaction creates a transaction at the requested reconciliation stage.
func analyticsTransaction(
	t *testing.T,
	transactionID string,
	routeID string,
	sellerID domain.ID,
	amount string,
	asset string,
	network string,
	createdAt time.Time,
	stage transactions.ReconciliationStage,
) transactions.Transaction {
	t.Helper()

	createdTimestamp := domain.NewTimestamp(createdAt)
	transaction, err := transactions.NewTransaction(transactions.TransactionParams{
		TransactionID: mustAnalyticsID(t, transactionID, domain.TransactionIDPrefix),
		IntentID: mustAnalyticsID(
			t,
			"int_"+transactionID[len("txn_"):],
			domain.IntentIDPrefix,
		),
		SellerID:  sellerID,
		RouteID:   mustAnalyticsID(t, routeID, domain.RouteIDPrefix),
		BuyerID:   "buyer-test",
		Amount:    domain.MustParseAmount(amount),
		Asset:     asset,
		Network:   network,
		CreatedAt: createdTimestamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.RequirePayment(createdTimestamp.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if stage == transactions.ReconciliationStageChallenged {
		return transaction
	}
	if err := transaction.VerifyPayment(
		"payment-"+transactionID,
		mustAnalyticsDigest(t),
		createdTimestamp.Add(2*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if stage == transactions.ReconciliationStageVerified {
		return transaction
	}
	if err := transaction.FinalizePayment(
		"payment-"+transactionID,
		"reference-"+transactionID,
		createdTimestamp.Add(3*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if stage == transactions.ReconciliationStageFinalized {
		return transaction
	}
	if stage == transactions.ReconciliationStageFailed {
		if err := transaction.MarkForwarded(createdTimestamp.Add(4 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if err := transaction.MarkFailed(
			"seller_timeout",
			nil,
			nil,
			createdTimestamp.Add(5*time.Second),
		); err != nil {
			t.Fatal(err)
		}
		return transaction
	}
	if err := transaction.MarkForwarded(createdTimestamp.Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkFulfilled(
		200,
		mustAnalyticsDigest(t),
		transactions.ResponseSummary{ContentType: "application/json", ContentLength: 1},
		createdTimestamp.Add(5*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if stage == transactions.ReconciliationStageDisputed {
		if err := transaction.OpenDispute(createdTimestamp.Add(6 * time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	return transaction
}

// assertAnalyticsAggregate locates and verifies one expected aggregate bucket.
func assertAnalyticsAggregate(
	t *testing.T,
	aggregates []SalesAggregate,
	bucketDate string,
	asset string,
	network string,
	routeID string,
	stage transactions.ReconciliationStage,
	wantCount uint64,
	wantAmount string,
) {
	t.Helper()

	for _, aggregate := range aggregates {
		if aggregate.BucketDate == bucketDate &&
			aggregate.Asset == asset &&
			aggregate.Network == network &&
			aggregate.RouteID == routeID &&
			aggregate.Stage == stage {
			if aggregate.TransactionCount != wantCount ||
				aggregate.Amount.String() != wantAmount {
				t.Fatalf("aggregate = %#v", aggregate)
			}
			return
		}
	}
	t.Fatalf(
		"aggregate not found for %s/%s/%s/%s/%s in %#v",
		bucketDate,
		asset,
		network,
		routeID,
		stage,
		aggregates,
	)
}

// mustAnalyticsID parses one deterministic analytics fixture identifier.
func mustAnalyticsID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()

	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}

// mustAnalyticsDigest returns one valid test digest.
func mustAnalyticsDigest(t *testing.T) intents.SHA256Digest {
	t.Helper()

	digest, err := intents.ParseSHA256Digest(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
