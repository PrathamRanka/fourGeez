package dynamodb

import (
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestBrowserPurchaseSessionRepositoryCreatesDocumentedRecordsAtomically(t *testing.T) {
	t.Parallel()
	client := &fakeClient{}
	repository := NewBrowserPurchaseSessionRepository(client, "agentpay-dev")
	session := testDynamoBrowserPurchaseSession()
	key, err := domain.ParseIdempotencyKey("checkout-0001")
	if err != nil {
		t.Fatal(err)
	}

	created, replay, err := repository.Create(t.Context(), session, "demo-store/research-report", key, strings.Repeat("b", 64))
	if err != nil || replay || created.PurchaseSessionID != session.PurchaseSessionID {
		t.Fatalf("Create() = (%#v, %v, %v)", created, replay, err)
	}
	input := client.transactWriteInput
	if input == nil || len(input.TransactItems) != 3 {
		t.Fatalf("TransactItems = %#v, want idempotency, grant claim, and session", input)
	}
	if got := readStringAttribute(input.TransactItems[0].Put.Item["PK"]); got != idempotencyPartitionKey("browser-purchase:demo-store/research-report") {
		t.Fatalf("idempotency PK = %q", got)
	}
	if got := readStringAttribute(input.TransactItems[1].Put.Item["PK"]); got != browserGrantPartitionKey(session.BrowserGrantHash) {
		t.Fatalf("grant PK = %q", got)
	}
	if got := readStringAttribute(input.TransactItems[2].Put.Item["PK"]); got != purchaseSessionPartitionKey(session.PurchaseSessionID.String()) {
		t.Fatalf("session PK = %q", got)
	}
}

func TestBrowserPurchaseSessionRepositoryMapsConcurrentClaimFailure(t *testing.T) {
	t.Parallel()
	session := testDynamoBrowserPurchaseSession()
	record, err := browserPurchaseSessionRecord(session)
	if err != nil {
		t.Fatal(err)
	}
	item, err := marshalStoredRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{
		getOutput: &awssdk.GetItemOutput{Item: item},
		putErr:    &types.ConditionalCheckFailedException{Message: stringPointer("lost race")},
	}
	repository := NewBrowserPurchaseSessionRepository(client, "agentpay-dev")
	transactionID := domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HB")

	_, err = repository.ClaimTransaction(t.Context(), session.PurchaseSessionID, transactionID, session.UpdatedAt.Add(time.Second))
	if !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("ClaimTransaction() error = %v", err)
	}
}

func testDynamoBrowserPurchaseSession() browserpurchase.BrowserPurchaseSession {
	now := testDynamoTime()
	return browserpurchase.BrowserPurchaseSession{
		PurchaseSessionID: "bps_01K5D09YJ0C0M7RJM4FWQ0K9HA",
		SellerID:          domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		RouteID:           domain.ID("rte_01K5D09YJ0C0M7RJM4FWQ0K9H8"),
		ProductSlug:       "research-report",
		RequestBodyHash:   strings.Repeat("a", 64),
		MaximumAmount:     domain.MustParseAmount("150"),
		BrowserGrantHash:  strings.Repeat("c", 64),
		CSRFTokenHash:     strings.Repeat("d", 64),
		Status:            browserpurchase.StatusActive,
		CommerceExpiresAt: now.Add(browserpurchase.MaximumCommerceLifetime),
		AccessExpiresAt:   now.Add(browserpurchase.MaximumCommerceLifetime),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}
