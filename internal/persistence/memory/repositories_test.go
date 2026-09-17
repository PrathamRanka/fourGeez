package memory

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/transactions"
)

func TestCatalogRepositoryEnforcesIDsAndSlugUniqueness(t *testing.T) {
	t.Parallel()
	repository := NewCatalogRepository()
	ctx := context.Background()
	seller := testSeller(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", "demo-seller")
	if err := repository.CreateSeller(ctx, seller); err != nil {
		t.Fatalf("CreateSeller() error = %v", err)
	}
	if err := repository.CreateSeller(ctx, seller); !errors.Is(err, persistence.ErrAlreadyExists) {
		t.Fatalf("duplicate CreateSeller() error = %v", err)
	}
	other := testSeller(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H8", seller.Slug)
	if err := repository.CreateSeller(ctx, other); !errors.Is(err, persistence.ErrAlreadyExists) {
		t.Fatalf("duplicate slug error = %v", err)
	}
	resolved, err := repository.ResolveSellerBySlug(ctx, seller.Slug)
	if err != nil || resolved.SellerID != seller.SellerID {
		t.Fatalf("ResolveSellerBySlug() = (%v, %v)", resolved, err)
	}

	route := testRoute(t, seller.SellerID)
	if err := repository.CreateRoute(ctx, route); err != nil {
		t.Fatalf("CreateRoute() error = %v", err)
	}
	routes, err := repository.ListRoutesBySeller(ctx, seller.SellerID)
	if err != nil || len(routes) != 1 || routes[0].RouteID != route.RouteID {
		t.Fatalf("ListRoutesBySeller() = (%v, %v)", routes, err)
	}
}

func TestPurchaseIntentRepositoryIsCreateOnly(t *testing.T) {
	t.Parallel()
	repository := NewPurchaseIntentRepository()
	ctx := context.Background()
	intent := testIntent(t)
	if err := repository.Create(ctx, intent); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repository.Create(ctx, intent); !errors.Is(err, persistence.ErrAlreadyExists) {
		t.Fatalf("duplicate Create() error = %v", err)
	}
	loaded, err := repository.Get(ctx, intent.IntentID())
	if err != nil || loaded.IntentHash() != intent.IntentHash() {
		t.Fatalf("Get() = (%v, %v)", loaded, err)
	}
}

func TestApprovalRepositoryUsesOptimisticVersion(t *testing.T) {
	t.Parallel()
	repository := NewApprovalRepository()
	ctx := context.Background()
	session := testApprovalSession(t)
	if err := repository.Create(ctx, session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repository.Update(ctx, session, 99); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("Update() stale version error = %v", err)
	}
}

func TestTransactionRepositoryPreventsPaymentReplayAndDuplicateForwarding(t *testing.T) {
	t.Parallel()
	repository := NewTransactionRepository()
	ctx := context.Background()
	first := testVerifiedTransaction(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", "payment-123")
	second := testVerifiedTransaction(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H8", "payment-123")
	if err := repository.Create(ctx, first); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	if err := repository.Create(ctx, second); !errors.Is(err, persistence.ErrPaymentIdentifierConflict) {
		t.Fatalf("Create(second) replay error = %v", err)
	}

	claimed, won, err := repository.ClaimForwarding(ctx, first.TransactionID(), first.Version(), first.UpdatedAt().Add(time.Second))
	if err != nil || !won || claimed.Status() != transactions.StatusForwarded {
		t.Fatalf("first ClaimForwarding() = (%v, %v, %v)", claimed.Status(), won, err)
	}
	_, won, err = repository.ClaimForwarding(ctx, first.TransactionID(), first.Version(), first.UpdatedAt().Add(time.Second))
	if err != nil || won {
		t.Fatalf("second ClaimForwarding() = (%v, %v), want lost claim without error", won, err)
	}
}

func TestTransactionRepositoryAllowsOneConcurrentForwardingClaim(t *testing.T) {
	t.Parallel()
	repository := NewTransactionRepository()
	ctx := context.Background()
	transaction := testVerifiedTransaction(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H9", "payment-concurrent")
	if err := repository.Create(ctx, transaction); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var winners atomic.Int32
	var waitGroup sync.WaitGroup
	for range 20 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, won, err := repository.ClaimForwarding(ctx, transaction.TransactionID(), transaction.Version(), transaction.UpdatedAt().Add(time.Second))
			if err != nil {
				t.Errorf("ClaimForwarding() error = %v", err)
				return
			}
			if won {
				winners.Add(1)
			}
		}()
	}
	waitGroup.Wait()
	if winners.Load() != 1 {
		t.Fatalf("forwarding winners = %d, want 1", winners.Load())
	}
}

func TestEvidenceRepositoryIsAppendOnlyAndContiguous(t *testing.T) {
	t.Parallel()
	repository := NewEvidenceRepository()
	ctx := context.Background()
	events := testEvidenceChain(t)
	if err := repository.Append(ctx, events[0]); err != nil {
		t.Fatalf("Append(first) error = %v", err)
	}
	if err := repository.Append(ctx, events[0]); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("duplicate Append() error = %v", err)
	}
	if err := repository.Append(ctx, events[1]); err != nil {
		t.Fatalf("Append(second) error = %v", err)
	}
	loaded, err := repository.ListByTransaction(ctx, events[0].TransactionID)
	if err != nil || len(loaded) != 2 {
		t.Fatalf("ListByTransaction() = (%v, %v)", loaded, err)
	}
	loaded[0].Payload["amount"] = "1"
	again, _ := repository.ListByTransaction(ctx, events[0].TransactionID)
	if again[0].Payload["amount"] == "1" {
		t.Fatal("repository returned mutable evidence payload storage")
	}
}

func TestDisputeAndIdempotencyRepositories(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	disputeRepository := NewDisputeRepository()
	dispute := testDispute(t)
	if err := disputeRepository.Create(ctx, dispute); err != nil {
		t.Fatalf("Create(dispute) error = %v", err)
	}
	if _, err := disputeRepository.Get(ctx, dispute.DisputeID); err != nil {
		t.Fatalf("Get(dispute) error = %v", err)
	}

	idempotencyStore := NewIdempotencyStore()
	key, _ := domain.ParseIdempotencyKey("request-123")
	record := domain.IdempotencyRecord{Scope: "create-dispute", Key: key, RequestHash: strings.Repeat("a", 64), ResponseStatus: 201, ResponseBody: []byte(`{"ok":true}`), CreatedAt: testTime(), ExpiresAt: testTime().Add(time.Hour)}
	created, err := idempotencyStore.SaveIfAbsent(ctx, record)
	if err != nil || !created {
		t.Fatalf("SaveIfAbsent() = (%v, %v)", created, err)
	}
	created, err = idempotencyStore.SaveIfAbsent(ctx, record)
	if err != nil || created {
		t.Fatalf("duplicate SaveIfAbsent() = (%v, %v)", created, err)
	}
	loaded, found, err := idempotencyStore.Load(ctx, record.Scope, key)
	if err != nil || !found || loaded.RequestHash != record.RequestHash {
		t.Fatalf("Load() = (%v, %v, %v)", loaded, found, err)
	}
}

// TestIntegrationCredentialRepositoryEnforcesSellerScopeAndVersion verifies storage guards.
func TestIntegrationCredentialRepositoryEnforcesSellerScopeAndVersion(t *testing.T) {
	t.Parallel()

	repository := NewIntegrationCredentialRepository()
	sellerID := mustID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	credentialID := mustID(
		t,
		"key_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		domain.CredentialIDPrefix,
	)
	credential, err := integrations.NewCredential(
		integrations.CredentialParams{
			CredentialID: credentialID,
			SellerID:     sellerID,
			TokenHash:    strings.Repeat("a", 64),
			Label:        "Claude Code",
			Scopes:       []integrations.Scope{integrations.ScopeRead},
			CreatedAt:    testTime(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(t.Context(), credential); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Get(
		t.Context(),
		mustID(
			t,
			"sel_01K5D09YJ0C0M7RJM4FWQ0K9H8",
			domain.SellerIDPrefix,
		),
		credentialID,
	); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("cross-seller Get() error = %v", err)
	}
	listed, err := repository.ListBySeller(t.Context(), sellerID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListBySeller() = (%v, %v)", listed, err)
	}
	if err := credential.Revoke(testTime().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repository.Update(
		t.Context(),
		credential,
		99,
	); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("stale Update() error = %v", err)
	}
}

func testSeller(t *testing.T, rawID, slug string) catalog.Seller {
	t.Helper()
	seller, err := catalog.NewSeller(catalog.SellerParams{SellerID: mustID(t, rawID, domain.SellerIDPrefix), OwnerSubject: "owner-123", Slug: slug, Name: "Demo Seller", UpstreamBaseURL: "https://seller.example", CreatedAt: testTime()})
	if err != nil {
		t.Fatalf("NewSeller() error = %v", err)
	}
	return seller
}

func testRoute(t *testing.T, sellerID domain.ID) catalog.PaidRoute {
	t.Helper()
	route, err := catalog.NewPaidRoute(catalog.PaidRouteParams{RouteID: mustID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix), SellerID: sellerID, Method: catalog.RouteMethodPost, PathPattern: "/research", Description: "Research", MIMEType: "application/json", Amount: domain.MustParseAmount("35000000"), Asset: "test-usdc", Network: "test-network", PayTo: "0x123", UpstreamTimeoutSeconds: 20, CreatedAt: testTime()})
	if err != nil {
		t.Fatalf("NewPaidRoute() error = %v", err)
	}
	return route
}

func testIntent(t *testing.T) intents.PurchaseIntent {
	t.Helper()
	bodyHash, _ := intents.HashRequestBody([]byte(`{"topic":"payments"}`), "application/json")
	intent, err := intents.NewPurchaseIntent(intents.PurchaseIntentParams{IntentID: mustID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix), SellerID: mustID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix), RouteID: mustID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix), BuyerID: "agent-123", RequestMethod: intents.RequestMethodPost, RequestPath: "/research", RequestBodyHash: bodyHash, Amount: domain.MustParseAmount("35000000"), Asset: "test-usdc", Network: "test-network", MaximumAmount: domain.MustParseAmount("40000000"), CreatedAt: testTime(), ExpiresAt: testTime().Add(10 * time.Minute)})
	if err != nil {
		t.Fatalf("NewPurchaseIntent() error = %v", err)
	}
	return intent
}

func testApprovalSession(t *testing.T) approvals.Session {
	t.Helper()
	intent := testIntent(t)
	generator := &fixedTokenGenerator{tokens: []string{strings.Repeat("a", 43), strings.Repeat("b", 43)}}
	session, _, err := approvals.NewSession(approvals.SessionParams{SessionID: mustID(t, "aps_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.ApprovalIDPrefix), IntentID: intent.IntentID(), IntentHash: intent.IntentHash(), ApproverLabels: []string{"Finance", "Security"}, CreatedAt: testTime(), ExpiresAt: testTime().Add(10 * time.Minute)}, generator)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	return session
}

type fixedTokenGenerator struct {
	tokens []string
	next   int
}

func (generator *fixedTokenGenerator) NewToken() (string, error) {
	token := generator.tokens[generator.next]
	generator.next++
	return token, nil
}

func testVerifiedTransaction(t *testing.T, rawID, paymentIdentifier string) transactions.Transaction {
	t.Helper()
	transaction, err := transactions.NewTransaction(transactions.TransactionParams{TransactionID: mustID(t, rawID, domain.TransactionIDPrefix), IntentID: mustID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix), SellerID: mustID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix), RouteID: mustID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix), BuyerID: "agent-123", Amount: domain.MustParseAmount("35000000"), Asset: "test-usdc", Network: "test-network", CreatedAt: testTime()})
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	if err := transaction.RequirePayment(testTime().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	digest, _ := intents.ParseSHA256Digest(strings.Repeat("a", 64))
	if err := transaction.VerifyPayment(paymentIdentifier, digest, testTime().Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	return transaction
}

func testEvidenceChain(t *testing.T) []evidence.Event {
	t.Helper()
	signer := testSigner{}
	transactionID := mustID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
	first, err := evidence.Append(context.Background(), evidence.EventParams{EventID: mustID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.EvidenceIDPrefix), TransactionID: transactionID, Sequence: 1, EventType: evidence.EventIntentCreated, ActorType: evidence.ActorBuyer, Payload: map[string]any{"amount": "35000000"}, CreatedAt: testTime()}, nil, signer)
	if err != nil {
		t.Fatal(err)
	}
	second, err := evidence.Append(context.Background(), evidence.EventParams{EventID: mustID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.EvidenceIDPrefix), TransactionID: transactionID, Sequence: 2, EventType: evidence.EventPaymentVerified, ActorType: evidence.ActorSystem, Payload: map[string]any{}, CreatedAt: testTime().Add(time.Second)}, &first, signer)
	if err != nil {
		t.Fatal(err)
	}
	return []evidence.Event{first, second}
}

type testSigner struct{}

func (testSigner) Sign(_ context.Context, digest []byte) (evidence.Signature, error) {
	return evidence.Signature{KeyID: "key", Value: string(digest)}, nil
}
func (testSigner) Verify(_ context.Context, _ string, digest []byte, signature string) (bool, error) {
	return string(digest) == signature, nil
}

func testDispute(t *testing.T) disputes.Dispute {
	t.Helper()
	delivery := false
	dispute, err := disputes.Classify(disputes.Params{DisputeID: mustID(t, "dsp_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.DisputeIDPrefix), TransactionID: mustID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix), Reason: disputes.ReasonNotDelivered, CreatedAt: testTime()}, disputes.Facts{DeliverySucceeded: &delivery})
	if err != nil {
		t.Fatal(err)
	}
	return dispute
}

func mustID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	id, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatalf("ParseID() error = %v", err)
	}
	return id
}

func testTime() domain.Timestamp {
	return domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
}
