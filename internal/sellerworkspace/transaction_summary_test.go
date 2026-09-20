package sellerworkspace

import (
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/transactions"
)

func TestSummarizeTransactionsSeparatesTestAndAbandonedActivity(t *testing.T) {
	t.Parallel()

	now := domain.NewTimestamp(time.Date(2026, time.September, 20, 12, 0, 0, 0, time.UTC))
	live := summaryTransaction(t, transactions.ActivityModeLive, now.Add(-time.Hour))
	test := summaryTransaction(t, transactions.ActivityModeTest, now.Add(-time.Hour))
	abandoned := summaryTransaction(t, transactions.ActivityModeLive, now.Add(-10*time.Minute))
	if err := abandoned.RequirePayment(now.Add(-9 * time.Minute)); err != nil {
		t.Fatal(err)
	}

	summary := summarizeTransactions([]transactions.Transaction{live, test, abandoned}, now)
	if summary.Total != 3 || summary.Live != 2 || summary.Test != 1 ||
		summary.Abandoned != 1 || summary.Pending != 2 {
		t.Fatalf("summary = %#v", summary)
	}
}

func summaryTransaction(t *testing.T, activityMode transactions.ActivityMode, createdAt domain.Timestamp) transactions.Transaction {
	t.Helper()
	transaction, err := transactions.NewTransaction(transactions.TransactionParams{
		TransactionID:     mustWorkspaceID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.TransactionIDPrefix),
		IntentID:          mustWorkspaceID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.IntentIDPrefix),
		SellerID:          mustWorkspaceID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		RouteID:           mustWorkspaceID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:           "buyer",
		ActivityMode:      activityMode,
		CheckoutExpiresAt: createdAt.Add(5 * time.Minute),
		Amount:            domain.MustParseAmount("100"),
		Asset:             "USDC",
		Network:           "eip155:84532",
		CreatedAt:         createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return transaction
}
