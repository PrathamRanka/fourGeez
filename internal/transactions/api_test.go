package transactions_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// TestTransactionRoutesReturnVerifiedRedactedEvidence verifies API-007 reads.
func TestTransactionRoutesReturnVerifiedRedactedEvidence(t *testing.T) {
	t.Parallel()

	handler, seller, transaction := newTransactionAPIHandler(t, 1)
	getRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/transactions/"+transaction.TransactionID().String(),
		nil,
	)
	getRequest.Header.Set(api.AgentKeyHeader, "agent-secret")
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getResponse.Code, getResponse.Body.String())
	}
	if strings.Contains(getResponse.Body.String(), "payment-demo") ||
		strings.Contains(getResponse.Body.String(), "paymentProofHash") {
		t.Fatal("transaction response exposed payment verification details")
	}
	var detail transactions.DetailResponse
	if err := json.Unmarshal(getResponse.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if !detail.Evidence.Valid || len(detail.Evidence.Events) != 2 {
		t.Fatalf("evidence summary = %#v", detail.Evidence)
	}
	if getResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("transaction response did not disable caching")
	}

	listRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/sellers/"+seller.SellerID.String()+"/transactions?limit=1",
		nil,
	)
	listRequest.Header.Set("Authorization", "Bearer seller-secret")
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	var page transactions.ListResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].TransactionID != transaction.TransactionID() {
		t.Fatalf("transaction page = %#v", page)
	}
}

// TestSellerTransactionRoutePaginatesAndEnforcesOwnership verifies list guards.
func TestSellerTransactionRoutePaginatesAndEnforcesOwnership(t *testing.T) {
	t.Parallel()

	handler, seller, _ := newTransactionAPIHandler(t, 2)
	firstRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/sellers/"+seller.SellerID.String()+"/transactions?limit=1",
		nil,
	)
	firstRequest.Header.Set("Authorization", "Bearer seller-secret")
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, firstRequest)
	var firstPage transactions.ListResponse
	if err := json.Unmarshal(firstResponse.Body.Bytes(), &firstPage); err != nil {
		t.Fatal(err)
	}
	if firstResponse.Code != http.StatusOK || firstPage.NextCursor == nil {
		t.Fatalf("first page status/body = %d/%s", firstResponse.Code, firstResponse.Body.String())
	}

	secondRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/sellers/"+seller.SellerID.String()+
			"/transactions?limit=1&cursor="+*firstPage.NextCursor,
		nil,
	)
	secondRequest.Header.Set("Authorization", "Bearer seller-secret")
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, secondRequest)
	var secondPage transactions.ListResponse
	if err := json.Unmarshal(secondResponse.Body.Bytes(), &secondPage); err != nil {
		t.Fatal(err)
	}
	if secondResponse.Code != http.StatusOK || len(secondPage.Items) != 1 {
		t.Fatalf("second page status/body = %d/%s", secondResponse.Code, secondResponse.Body.String())
	}
	if secondPage.Items[0].TransactionID == firstPage.Items[0].TransactionID {
		t.Fatal("pagination returned the first transaction twice")
	}

	otherSellerRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H8/transactions",
		nil,
	)
	otherSellerRequest.Header.Set("Authorization", "Bearer seller-secret")
	otherSellerResponse := httptest.NewRecorder()
	handler.ServeHTTP(otherSellerResponse, otherSellerRequest)
	if otherSellerResponse.Code != http.StatusNotFound {
		t.Fatalf("other seller status = %d", otherSellerResponse.Code)
	}
}

// TestTransactionServiceReportsTamperedEvidence verifies integrity reporting.
func TestTransactionServiceReportsTamperedEvidence(t *testing.T) {
	t.Parallel()

	_, _, _, _, transaction := newTransactionAPIFixture(t, 1)
	events := transactionEvidence(t, transaction)
	events[0].Payload["amount"] = "1"
	service := transactions.NewService(
		&transactionTestRepository{transaction: transaction},
		&transactionTestEvidenceRepository{events: events},
		transactionTestSigner{},
		nil,
	)
	detail, err := service.Get(t.Context(), transaction.TransactionID())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if detail.Evidence.Valid {
		t.Fatal("tampered evidence was reported as valid")
	}
}

// TestTransactionServicePreventsCrossSellerReads verifies tenant isolation.
func TestTransactionServicePreventsCrossSellerReads(t *testing.T) {
	t.Parallel()

	_, _, _, seller, transaction := newTransactionAPIFixture(t, 1)
	seller.OwnerSubject = "different-seller"
	service := transactions.NewService(
		&transactionTestRepository{transaction: transaction},
		&transactionTestEvidenceRepository{
			events: transactionEvidence(t, transaction),
		},
		transactionTestSigner{},
		&transactionTestSellerRepository{seller: seller},
	)
	_, err := service.GetForSeller(
		t.Context(),
		transaction.TransactionID(),
		"local-seller",
	)
	if !errors.Is(err, transactions.ErrSellerAccess) {
		t.Fatalf("GetForSeller() error = %v, want ErrSellerAccess", err)
	}
}

// newTransactionAPIHandler creates a seeded API-007 HTTP handler.
func newTransactionAPIHandler(
	t *testing.T,
	transactionCount int,
) (http.Handler, catalog.Seller, transactions.Transaction) {
	t.Helper()
	catalogRepository, transactionRepository, evidenceRepository, seller, firstTransaction := newTransactionAPIFixture(
		t,
		transactionCount,
	)
	service := transactions.NewService(
		transactionRepository,
		evidenceRepository,
		transactionTestSigner{},
		catalogRepository,
	)
	controller := transactions.NewHTTPController(service)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	return api.Middleware(
		api.Config{
			Authenticator: api.NewStaticAuthenticator(
				"seller-secret",
				"agent-secret",
			),
		},
		mux,
	), seller, firstTransaction
}

// newTransactionAPIFixture seeds seller, transaction, and evidence repositories.
func newTransactionAPIFixture(
	t *testing.T,
	transactionCount int,
) (
	*memory.CatalogRepository,
	*memory.TransactionRepository,
	*memory.EvidenceRepository,
	catalog.Seller,
	transactions.Transaction,
) {
	t.Helper()
	createdAt := domain.NewTimestamp(
		time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	)
	catalogRepository := memory.NewCatalogRepository()
	transactionRepository := memory.NewTransactionRepository()
	evidenceRepository := memory.NewEvidenceRepository()
	seller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID:        mustTransactionAPIID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		OwnerSubject:    "local-seller",
		Slug:            "demo-shop",
		Name:            "Demo Shop",
		UpstreamBaseURL: "https://seller.example",
		CreatedAt:       createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogRepository.CreateSeller(t.Context(), seller); err != nil {
		t.Fatal(err)
	}

	var firstTransaction transactions.Transaction
	for index := 0; index < transactionCount; index++ {
		transactionID := "txn_01K5D09YJ0C0M7RJM4FWQ0K9H" + string(rune('7'+index))
		transaction := newFulfilledTransaction(
			t,
			transactionID,
			seller.SellerID,
			createdAt.Add(time.Duration(index)*time.Second),
		)
		if index == 0 {
			firstTransaction = transaction
		}
		if err := transactionRepository.Create(t.Context(), transaction); err != nil {
			t.Fatal(err)
		}
		for _, event := range transactionEvidence(t, transaction) {
			if err := evidenceRepository.Append(t.Context(), event); err != nil {
				t.Fatal(err)
			}
		}
	}
	return catalogRepository, transactionRepository, evidenceRepository, seller, firstTransaction
}

// newFulfilledTransaction creates a transaction containing sensitive payment state.
func newFulfilledTransaction(
	t *testing.T,
	rawTransactionID string,
	sellerID domain.ID,
	createdAt domain.Timestamp,
) transactions.Transaction {
	t.Helper()
	transaction, err := transactions.NewTransaction(transactions.TransactionParams{
		TransactionID: mustTransactionAPIID(t, rawTransactionID, domain.TransactionIDPrefix),
		IntentID:      mustTransactionAPIID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:      sellerID,
		RouteID:       mustTransactionAPIID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:       "local-agent",
		Amount:        domain.MustParseAmount("35000000"),
		Asset:         "test-usdc",
		Network:       "test-network",
		CreatedAt:     createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.RequirePayment(createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.VerifyPayment(
		"payment-"+rawTransactionID,
		mustTransactionAPIDigest(t, strings.Repeat("a", 64)),
		createdAt.Add(2*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkForwarded(createdAt.Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkFulfilled(
		http.StatusOK,
		mustTransactionAPIDigest(t, strings.Repeat("b", 64)),
		transactions.ResponseSummary{
			ContentType:   "application/json",
			ContentLength: 20,
		},
		createdAt.Add(4*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	return transaction
}

// transactionEvidence creates a valid two-event chain for a transaction.
func transactionEvidence(
	t *testing.T,
	transaction transactions.Transaction,
) []evidence.Event {
	t.Helper()
	first, err := evidence.Append(t.Context(), evidence.EventParams{
		EventID:       mustTransactionAPIID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.EvidenceIDPrefix),
		TransactionID: transaction.TransactionID(),
		Sequence:      1,
		EventType:     evidence.EventPaymentVerified,
		ActorType:     evidence.ActorSystem,
		Payload:       map[string]any{"amount": transaction.Amount().String()},
		CreatedAt:     transaction.CreatedAt().Add(2 * time.Second),
	}, nil, transactionTestSigner{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := evidence.Append(t.Context(), evidence.EventParams{
		EventID:       mustTransactionAPIID(t, "evt_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.EvidenceIDPrefix),
		TransactionID: transaction.TransactionID(),
		Sequence:      2,
		EventType:     evidence.EventDeliverySucceeded,
		ActorType:     evidence.ActorSeller,
		ActorID:       transaction.SellerID().String(),
		Payload:       map[string]any{"upstreamStatus": http.StatusOK},
		CreatedAt:     transaction.CreatedAt().Add(4 * time.Second),
	}, &first, transactionTestSigner{})
	if err != nil {
		t.Fatal(err)
	}
	return []evidence.Event{first, second}
}

// transactionTestSigner signs evidence deterministically for API tests.
type transactionTestSigner struct{}

// Sign returns an HMAC signature for a test evidence digest.
func (transactionTestSigner) Sign(
	_ context.Context,
	digest []byte,
) (evidence.Signature, error) {
	mac := hmac.New(sha256.New, []byte("transaction-api-test-key"))
	_, _ = mac.Write(digest)
	return evidence.Signature{
		KeyID: "test-key-v1",
		Value: base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	}, nil
}

// Verify checks an HMAC signature for a test evidence digest.
func (signer transactionTestSigner) Verify(
	ctx context.Context,
	keyID string,
	digest []byte,
	signature string,
) (bool, error) {
	if keyID != "test-key-v1" {
		return false, nil
	}
	expected, err := signer.Sign(ctx, digest)
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(expected.Value), []byte(signature)), nil
}

// transactionTestRepository returns one transaction for a service test.
type transactionTestRepository struct {
	transaction transactions.Transaction
}

// Get returns the configured transaction.
func (repository *transactionTestRepository) Get(
	_ context.Context,
	_ domain.ID,
) (transactions.Transaction, error) {
	return repository.transaction, nil
}

// ListBySeller returns no rows because the integrity test uses Get only.
func (repository *transactionTestRepository) ListBySeller(
	_ context.Context,
	_ domain.ID,
	_ int,
	_ string,
) ([]transactions.Transaction, *string, error) {
	return nil, nil, nil
}

// transactionTestEvidenceRepository returns a configured evidence chain.
type transactionTestEvidenceRepository struct {
	events []evidence.Event
}

// transactionTestSellerRepository returns one seller ownership record.
type transactionTestSellerRepository struct {
	seller catalog.Seller
}

// GetSeller returns the configured seller.
func (repository *transactionTestSellerRepository) GetSeller(
	_ context.Context,
	_ domain.ID,
) (catalog.Seller, error) {
	return repository.seller, nil
}

// ListByTransaction returns the configured evidence chain.
func (repository *transactionTestEvidenceRepository) ListByTransaction(
	_ context.Context,
	_ domain.ID,
) ([]evidence.Event, error) {
	return repository.events, nil
}

// mustTransactionAPIID parses a test domain identifier.
func mustTransactionAPIID(
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

// mustTransactionAPIDigest parses a test SHA-256 digest.
func mustTransactionAPIDigest(
	t *testing.T,
	raw string,
) intents.SHA256Digest {
	t.Helper()
	digest, err := intents.ParseSHA256Digest(raw)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
