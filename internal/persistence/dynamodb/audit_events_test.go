package dynamodb

import (
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestAuditEventRepositoryUsesAppendOnlySellerKeys verifies DynamoDB audit access.
func TestAuditEventRepositoryUsesAppendOnlySellerKeys(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	repository := NewAuditEventRepository(client, "agentpay-dev")
	event := testDynamoAuditEvent(t)
	if err := repository.Create(t.Context(), event); err != nil {
		t.Fatal(err)
	}
	if client.putInput == nil ||
		readStringAttribute(client.putInput.Item["PK"]) != sellerPartitionKey(event.SellerID().String()) ||
		readStringAttribute(client.putInput.Item["SK"]) != auditEventSortKey(
			event.OccurredAt().Time(),
			event.AuditEventID().String(),
		) {
		t.Fatalf("audit item = %#v", client.putInput)
	}

	client.queryOutput = &awssdk.QueryOutput{
		Items: []map[string]types.AttributeValue{
			client.putInput.Item,
		},
		LastEvaluatedKey: primaryKey(
			sellerPartitionKey(event.SellerID().String()),
			auditEventSortKey(event.OccurredAt().Time(), event.AuditEventID().String()),
		),
	}
	events, nextCursor, err := repository.ListBySeller(
		t.Context(),
		event.SellerID(),
		1,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || nextCursor == nil {
		t.Fatalf("events = %#v, cursor = %#v", events, nextCursor)
	}
	if client.queryInput == nil ||
		client.queryInput.ScanIndexForward == nil ||
		*client.queryInput.ScanIndexForward ||
		client.queryInput.KeyConditionExpression == nil ||
		!strings.Contains(*client.queryInput.KeyConditionExpression, "begins_with") {
		t.Fatalf("query input = %#v", client.queryInput)
	}
}

// testDynamoAuditEvent creates one valid immutable audit fixture.
func testDynamoAuditEvent(t *testing.T) audit.Event {
	t.Helper()

	sellerID := mustDynamoID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	event, err := audit.NewEvent(audit.EventParams{
		AuditEventID: mustDynamoID(
			t,
			"aud_01K5D09YJ0C0M7RJM4FWQ0K9H8",
			domain.AuditEventIDPrefix,
		),
		SellerID:   sellerID,
		ActorType:  audit.ActorTypeAdministrator,
		ActorID:    "admin-subject",
		Action:     audit.ActionSellerSuspended,
		TargetType: audit.TargetTypeSeller,
		TargetID:   sellerID.String(),
		Outcome:    audit.OutcomeSucceeded,
		RequestID:  "request-123",
		ChangedFields: []string{
			"status",
		},
		OccurredAt: domain.NewTimestamp(
			time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
		),
	})
	if err != nil {
		t.Fatal(err)
	}
	return event
}
