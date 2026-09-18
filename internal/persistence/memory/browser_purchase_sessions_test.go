package memory

import (
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestBrowserPurchaseSessionRepositoryCreationGrantLookupAndIdempotency(t *testing.T) {
	t.Parallel()
	repository := NewBrowserPurchaseSessionRepository()
	session := testBrowserPurchaseSession()
	created, replay, err := repository.Create(t.Context(), session, "demo-store/research-report", domain.IdempotencyKey("checkout-0001"), "request-hash")
	if err != nil || replay || created.PurchaseSessionID != session.PurchaseSessionID {
		t.Fatalf("Create() = %#v, %v, %v", created, replay, err)
	}
	byGrant, err := repository.GetByGrantHash(t.Context(), session.BrowserGrantHash)
	if err != nil || byGrant.PurchaseSessionID != session.PurchaseSessionID {
		t.Fatalf("GetByGrantHash() = %#v, %v", byGrant, err)
	}
	replayed, replay, err := repository.Create(t.Context(), testBrowserPurchaseSession(), "demo-store/research-report", domain.IdempotencyKey("checkout-0001"), "request-hash")
	if err != nil || !replay || replayed.PurchaseSessionID != session.PurchaseSessionID {
		t.Fatalf("replay Create() = %#v, %v, %v", replayed, replay, err)
	}
	if _, _, err := repository.Create(t.Context(), testBrowserPurchaseSession(), "demo-store/research-report", domain.IdempotencyKey("checkout-0001"), "changed"); !errors.Is(err, browserpurchase.ErrIdempotencyConflict) {
		t.Fatalf("changed replay error = %v", err)
	}
}

func TestBrowserPurchaseSessionRepositoryRecoveryIsAtomicAndSingleUse(t *testing.T) {
	t.Parallel()
	repository := NewBrowserPurchaseSessionRepository()
	session := testBrowserPurchaseSession()
	walletBindingHash := "wallet-hash"
	transactionID := domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HA")
	session.Status = browserpurchase.StatusCompleted
	session.WalletBindingHash = &walletBindingHash
	session.TransactionID = &transactionID
	if _, _, err := repository.Create(t.Context(), session, "demo-store/research-report", domain.IdempotencyKey("checkout-0001"), "request-hash"); err != nil {
		t.Fatal(err)
	}
	challenge := browserpurchase.BrowserPurchaseRecoveryChallengeRecord{ChallengeID: "bpr_01K5D09YJ0C0M7RJM4FWQ0K9HB", PurchaseSessionID: session.PurchaseSessionID, ExpectedWalletBindingHash: "wallet-hash", Nonce: "nonce", MessageHash: "message-hash", ExpiresAt: session.AccessExpiresAt}
	if err := repository.CreateChallenge(t.Context(), challenge); err != nil {
		t.Fatal(err)
	}
	previousGrantHash := session.BrowserGrantHash
	session.BrowserGrantHash = "rotated-grant-hash"
	session.CSRFTokenHash = "rotated-csrf-hash"
	usedAt := session.CreatedAt.Add(time.Minute)
	challenge.UsedAt = &usedAt
	if err := repository.Recover(t.Context(), session, previousGrantHash, challenge); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetByGrantHash(t.Context(), previousGrantHash); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("old grant error = %v", err)
	}
	if _, err := repository.GetByGrantHash(t.Context(), session.BrowserGrantHash); err != nil {
		t.Fatal(err)
	}
	if err := repository.Recover(t.Context(), session, previousGrantHash, challenge); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("replayed Recover() error = %v", err)
	}
}

func TestBrowserPurchaseSessionRepositoryClaimsOneTransaction(t *testing.T) {
	t.Parallel()
	repository := NewBrowserPurchaseSessionRepository()
	session := testBrowserPurchaseSession()
	if _, _, err := repository.Create(t.Context(), session, "demo-store/research-report", domain.IdempotencyKey("checkout-0001"), "request-hash"); err != nil {
		t.Fatal(err)
	}
	transactionID := domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HA")
	claimed, err := repository.ClaimTransaction(t.Context(), session.PurchaseSessionID, transactionID, session.CreatedAt.Add(time.Minute))
	if err != nil || claimed.TransactionID == nil || *claimed.TransactionID != transactionID {
		t.Fatalf("ClaimTransaction() = %#v, %v", claimed, err)
	}
	if _, err := repository.ClaimTransaction(t.Context(), session.PurchaseSessionID, transactionID, session.CreatedAt.Add(2*time.Minute)); err != nil {
		t.Fatalf("idempotent claim error = %v", err)
	}
	if _, err := repository.ClaimTransaction(t.Context(), session.PurchaseSessionID, domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HB"), session.CreatedAt.Add(2*time.Minute)); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("changed claim error = %v", err)
	}
}

func testBrowserPurchaseSession() browserpurchase.BrowserPurchaseSession {
	now := domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC))
	return browserpurchase.BrowserPurchaseSession{
		PurchaseSessionID: "bps_01K5D09YJ0C0M7RJM4FWQ0K9HA",
		SellerID:          domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		RouteID:           domain.ID("rte_01K5D09YJ0C0M7RJM4FWQ0K9H8"),
		ProductSlug:       "research-report",
		RequestBodyHash:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MaximumAmount:     domain.MustParseAmount("150"),
		BrowserGrantHash:  "grant-hash",
		CSRFTokenHash:     "csrf-hash",
		Status:            browserpurchase.StatusActive,
		CommerceExpiresAt: now.Add(browserpurchase.MaximumCommerceLifetime),
		AccessExpiresAt:   now.Add(browserpurchase.MaximumCommerceLifetime),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}
