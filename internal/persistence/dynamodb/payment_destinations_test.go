package dynamodb

import (
	"context"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/settlement"
)

// TestPaymentDestinationRepositoryUsesSellerScopedKeys verifies WAL-001 persistence.
func TestPaymentDestinationRepositoryUsesSellerScopedKeys(t *testing.T) {
	t.Parallel()

	destination := testDynamoPaymentDestination(t)
	client := &fakeClient{}
	repository := NewPaymentDestinationRepository(client, "agentpay-dev")
	if err := repository.Create(context.Background(), destination); err != nil {
		t.Fatal(err)
	}
	if client.putInput == nil {
		t.Fatal("Create() did not write a destination")
	}
	if readStringAttribute(client.putInput.Item["PK"]) !=
		sellerPartitionKey(destination.SellerID.String()) {
		t.Fatalf("partition key = %#v", client.putInput.Item["PK"])
	}
	if readStringAttribute(client.putInput.Item["SK"]) !=
		paymentDestinationSortKey(destination.DestinationID.String()) {
		t.Fatalf("sort key = %#v", client.putInput.Item["SK"])
	}

	client.queryOutput = &awssdk.QueryOutput{
		Items: []map[string]types.AttributeValue{client.putInput.Item},
	}
	destinations, err := repository.ListBySeller(
		context.Background(),
		destination.SellerID,
	)
	if err != nil || len(destinations) != 1 {
		t.Fatalf("ListBySeller() = (%#v, %v)", destinations, err)
	}
	if client.queryInput == nil ||
		client.queryInput.KeyConditionExpression == nil ||
		!strings.Contains(*client.queryInput.KeyConditionExpression, "begins_with") {
		t.Fatalf("query input = %#v", client.queryInput)
	}
	if client.queryInput.ConsistentRead == nil || !*client.queryInput.ConsistentRead {
		t.Fatalf("destination eligibility query must be strongly consistent: %#v", client.queryInput)
	}
}

// TestPaymentDestinationRepositoryPersistsChallengeHash verifies the private snapshot field.
func TestPaymentDestinationRepositoryPersistsChallengeHash(t *testing.T) {
	t.Parallel()

	destination := testDynamoPaymentDestination(t)
	destination.ChallengeHash = strings.Repeat("a", 64)
	expiresAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 10, 0, 0, time.UTC))
	destination.ChallengeExpiresAt = &expiresAt
	client := &fakeClient{}
	repository := NewPaymentDestinationRepository(client, "agentpay-dev")
	if err := repository.Create(context.Background(), destination); err != nil {
		t.Fatal(err)
	}
	client.queryOutput = &awssdk.QueryOutput{
		Items: []map[string]types.AttributeValue{client.putInput.Item},
	}
	destinations, err := repository.ListBySeller(context.Background(), destination.SellerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(destinations) != 1 || destinations[0].ChallengeHash != destination.ChallengeHash {
		t.Fatalf("persisted destinations = %#v", destinations)
	}
}

// TestPaymentDestinationRepositoryUsesConditionalChallengeAndActivationWrites verifies WAL-002 concurrency.
func TestPaymentDestinationRepositoryUsesConditionalChallengeAndActivationWrites(t *testing.T) {
	t.Parallel()
	destination := testDynamoPaymentDestination(t)
	client := &fakeClient{}
	repository := NewPaymentDestinationRepository(client, "agentpay-dev")
	destination.ChallengeHash = strings.Repeat("b", 64)
	destination.Version++
	if err := repository.SaveChallenge(context.Background(), destination, 1); err != nil {
		t.Fatal(err)
	}
	if client.putInput == nil ||
		client.putInput.ConditionExpression == nil ||
		!strings.Contains(*client.putInput.ConditionExpression, "#version = :expectedVersion") {
		t.Fatalf("challenge condition = %#v", client.putInput)
	}

	client.putInput = nil
	destination.Status = settlement.PaymentDestinationStatusActive
	destination.Version++
	if err := repository.Activate(
		context.Background(),
		settlement.PaymentDestinationActivation{
			Destination:     destination,
			ExpectedVersion: 2,
		},
	); err != nil {
		t.Fatal(err)
	}
	if client.transactWriteInput == nil || len(client.transactWriteInput.TransactItems) != 2 {
		t.Fatalf("activation transaction = %#v", client.transactWriteInput)
	}
	claimPut := client.transactWriteInput.TransactItems[0].Put
	if claimPut == nil {
		t.Fatalf("activation claim write = %#v", client.transactWriteInput.TransactItems[0])
	}
	if claimPut.ExpressionAttributeValues != nil {
		t.Fatalf(
			"create-only active-destination claim must omit expression values, got %#v",
			claimPut.ExpressionAttributeValues,
		)
	}
}

// testDynamoPaymentDestination creates one valid DynamoDB fixture.
func testDynamoPaymentDestination(t *testing.T) settlement.PaymentDestination {
	t.Helper()

	destination, err := settlement.NewPaymentDestination(
		settlement.PaymentDestinationParams{
			DestinationID: mustDynamoID(
				t,
				"dst_01K5D09YJ0C0M7RJM4FWQ0K9H8",
				domain.PaymentDestinationIDPrefix,
			),
			SellerID: mustDynamoID(
				t,
				"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				domain.SellerIDPrefix,
			),
			Asset:     "USDC",
			Network:   "eip155:84532",
			Address:   "0x1111111111111111111111111111111111111111",
			CreatedAt: testDynamoTime(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return destination
}
