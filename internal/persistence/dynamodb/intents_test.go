package dynamodb

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestPurchaseIntentUpdateUsesExpectedVersionCondition(t *testing.T) {
	t.Parallel()

	purchaseIntent := testDynamoPurchaseIntent(t)
	client := &fakeClient{}
	repository := NewPurchaseIntentRepository(client, "agentpay-dev")
	if err := repository.Create(t.Context(), purchaseIntent); err != nil {
		t.Fatal(err)
	}
	createdVersion, ok := client.putInput.Item["version"].(*types.AttributeValueMemberN)
	if !ok || createdVersion.Value != strconv.FormatUint(purchaseIntent.Version(), 10) {
		t.Fatalf("created version attribute = %#v, want %d", client.putInput.Item["version"], purchaseIntent.Version())
	}

	expectedVersion := purchaseIntent.Version()
	if err := purchaseIntent.Cancel(testDynamoTime().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	if err := repository.Update(t.Context(), purchaseIntent, expectedVersion); err != nil {
		t.Fatal(err)
	}
	if client.putInput == nil || client.putInput.ConditionExpression == nil ||
		*client.putInput.ConditionExpression != "#version = :expectedVersion" {
		t.Fatalf("intent update condition = %#v", client.putInput)
	}
	expectedVersionAttribute, ok := client.putInput.ExpressionAttributeValues[":expectedVersion"].(*types.AttributeValueMemberN)
	if !ok || expectedVersionAttribute.Value != strconv.FormatUint(expectedVersion, 10) {
		t.Fatalf("expected version attribute = %#v, want %d", client.putInput.ExpressionAttributeValues[":expectedVersion"], expectedVersion)
	}
	storedVersion, ok := client.putInput.Item["version"].(*types.AttributeValueMemberN)
	if !ok || storedVersion.Value != strconv.FormatUint(purchaseIntent.Version(), 10) {
		t.Fatalf("stored version attribute = %#v, want %d", client.putInput.Item["version"], purchaseIntent.Version())
	}

	client.putErr = &types.ConditionalCheckFailedException{Message: stringPointer("stale")}
	if err := repository.Update(t.Context(), purchaseIntent, expectedVersion); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("Update() error = %v, want condition failed", err)
	}
}

func testDynamoPurchaseIntent(t *testing.T) intents.PurchaseIntent {
	t.Helper()

	requestBodyHash, err := intents.HashRequestBody([]byte(`{"topic":"agent commerce"}`), "application/json")
	if err != nil {
		t.Fatal(err)
	}
	createdAt := testDynamoTime()
	purchaseIntent, err := intents.NewPurchaseIntent(intents.PurchaseIntentParams{
		IntentID:             mustDynamoID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:             mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		RouteID:              mustDynamoID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:              "agent-123",
		PurchaseChannel:      intents.PurchaseChannelAgent,
		ProductDisplayName:   "Research report",
		ProductSlug:          "research-report",
		PaymentDestinationID: mustDynamoID(t, "dst_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.PaymentDestinationIDPrefix),
		PayTo:                "0x1111111111111111111111111111111111111111",
		RequestMethod:        intents.RequestMethodPost,
		RequestPath:          "/research/board",
		RequestBodyHash:      requestBodyHash,
		Amount:               domain.MustParseAmount("35000000"),
		Asset:                "test-usdc",
		Network:              "test-network",
		MaximumAmount:        domain.MustParseAmount("50000000"),
		CreatedAt:            createdAt,
		ExpiresAt:            createdAt.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	return purchaseIntent
}
