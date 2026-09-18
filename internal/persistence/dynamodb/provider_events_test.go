package dynamodb

import (
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

func TestProviderEventRepositoryStoresStripeInboxByGlobalEventID(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	repository := NewProviderEventRepository(client, "agentpay-dev")
	event := testDynamoProviderEvent()
	result, err := repository.Store(t.Context(), event)
	if err != nil || result != billing.ProviderEventStoreResultInserted {
		t.Fatalf("Store() = (%v, %v)", result, err)
	}
	if client.putInput == nil ||
		readStringAttribute(client.putInput.Item["PK"]) != providerEventPartitionKey(event.EventID) ||
		readStringAttribute(client.putInput.Item["SK"]) != providerEventInboxSortKey ||
		readStringAttribute(client.putInput.Item["eventHash"]) != event.PayloadHash ||
		client.putInput.ConditionExpression == nil || *client.putInput.ConditionExpression != createItemCondition {
		t.Fatalf("provider inbox put = %#v", client.putInput)
	}
}

func TestProviderEventRepositoryTreatsSameHashAsDuplicate(t *testing.T) {
	t.Parallel()

	event := testDynamoProviderEvent()
	item, err := marshalProviderEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{
		putErr:    &types.ConditionalCheckFailedException{Message: stringPointer("duplicate")},
		getOutput: &awssdk.GetItemOutput{Item: item},
	}
	repository := NewProviderEventRepository(client, "agentpay-dev")
	result, err := repository.Store(t.Context(), event)
	if err != nil || result != billing.ProviderEventStoreResultDuplicate {
		t.Fatalf("Store() = (%v, %v)", result, err)
	}
}

func TestProviderEventRepositoryQuarantinesSameIDWithDifferentHash(t *testing.T) {
	t.Parallel()

	stored := testDynamoProviderEvent()
	item, err := marshalProviderEvent(stored)
	if err != nil {
		t.Fatal(err)
	}
	conflict := stored
	conflict.PayloadHash = strings.Repeat("f", 64)
	client := &fakeClient{
		putErrors: []error{&types.ConditionalCheckFailedException{Message: stringPointer("duplicate")}, nil},
		getOutput: &awssdk.GetItemOutput{Item: item},
	}
	repository := NewProviderEventRepository(client, "agentpay-dev")
	result, err := repository.Store(t.Context(), conflict)
	if err != nil || result != billing.ProviderEventStoreResultConflict || client.putCallCount != 2 {
		t.Fatalf("Store() = (%v, %v), put calls=%d", result, err, client.putCallCount)
	}
	var quarantined billing.SubscriptionProviderEvent
	if err := unmarshalPayload(client.putInput.Item, &quarantined); err != nil {
		t.Fatal(err)
	}
	if quarantined.ProcessingState != billing.ProviderEventStateQuarantined || quarantined.ConflictPayloadHash != conflict.PayloadHash {
		t.Fatalf("quarantined event = %#v", quarantined)
	}
}

func testDynamoProviderEvent() billing.SubscriptionProviderEvent {
	return billing.SubscriptionProviderEvent{
		EventID: "evt_123", Provider: billing.EntitlementProviderStripe,
		EventType: "invoice.paid", PayloadHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Livemode: true, AccountID: "acct_123", APIVersion: "2026-08-27.basil",
		ProviderCreatedAt: domain.NewTimestamp(testDynamoTime().Time().Add(-time.Hour)),
		ReceivedAt:        testDynamoTime(), ProcessingState: billing.ProviderEventStateReceived,
	}
}
