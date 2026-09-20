package dynamodb

import (
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/publicationops"
)

func TestPublicationCompletionStoreUsesConsistentReadAndConditionalCompletion(t *testing.T) {
	t.Parallel()
	client := &fakeClient{getOutput: &awssdk.GetItemOutput{}}
	store := NewPublicationCompletionStore(client, "agentpay-dev")
	event := publicationops.Event{
		SchemaVersion:    publicationops.SchemaVersionV1,
		EventID:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		EventType:        publicationops.EventRouteChanged,
		SellerID:         "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		AggregateID:      "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		AggregateVersion: 2,
		OccurredAt:       domain.NewTimestamp(time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)),
	}
	completed, err := store.Completed(t.Context(), event.EventID)
	if err != nil || completed {
		t.Fatalf("Completed() = %v, %v", completed, err)
	}
	if client.getInput == nil || client.getInput.ConsistentRead == nil || !*client.getInput.ConsistentRead {
		t.Fatalf("completion read = %#v", client.getInput)
	}
	if err := store.Complete(t.Context(), event); err != nil {
		t.Fatal(err)
	}
	if client.putInput == nil || client.putInput.ConditionExpression == nil || *client.putInput.ConditionExpression != createItemCondition {
		t.Fatalf("completion write = %#v", client.putInput)
	}
	client.putErr = &types.ConditionalCheckFailedException{}
	if err := store.Complete(t.Context(), event); err != nil {
		t.Fatalf("duplicate completion error = %v", err)
	}
}
