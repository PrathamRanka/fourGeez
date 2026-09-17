package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// AuditEventRepository persists immutable seller audit history in DynamoDB.
type AuditEventRepository struct {
	repositoryBase
}

// NewAuditEventRepository creates a DynamoDB audit repository.
func NewAuditEventRepository(
	client Client,
	tableName string,
) *AuditEventRepository {
	return &AuditEventRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Create appends one immutable seller audit event.
func (repository *AuditEventRepository) Create(
	ctx context.Context,
	event audit.Event,
) error {
	record, err := newStoredRecord(
		sellerPartitionKey(event.SellerID().String()),
		auditEventSortKey(event.OccurredAt().Time(), event.AuditEventID().String()),
		"auditEvent",
		event.Snapshot(),
	)
	if err != nil {
		return err
	}
	item, err := marshalStoredRecord(record)
	if err != nil {
		return err
	}
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                item,
		ConditionExpression: stringPointer(createItemCondition),
	})
	if isConditionalFailure(err) {
		return persistence.ErrAlreadyExists
	}
	return err
}

// ListBySeller queries one bounded newest-first audit page without scanning.
func (repository *AuditEventRepository) ListBySeller(
	ctx context.Context,
	sellerID domain.ID,
	limit int,
	rawCursor string,
) ([]audit.Event, *string, error) {
	keyCondition := "PK = :partitionKey AND begins_with(SK, :sortKeyPrefix)"
	ascending := false
	queryLimit := int32(limit)
	input := &awssdk.QueryInput{
		TableName:              &repository.tableName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":partitionKey":  stringAttributeValue(sellerPartitionKey(sellerID.String())),
			":sortKeyPrefix": stringAttributeValue("AUDIT#"),
		},
		ConsistentRead:   boolPointer(true),
		ScanIndexForward: &ascending,
		Limit:            &queryLimit,
	}
	if rawCursor != "" {
		cursor, err := audit.DecodeCursor(rawCursor, sellerID)
		if err != nil {
			return nil, nil, err
		}
		input.ExclusiveStartKey = primaryKey(
			sellerPartitionKey(sellerID.String()),
			auditEventSortKey(cursor.OccurredAt.Time(), cursor.AuditEventID.String()),
		)
	}
	output, err := repository.client.Query(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	events := make([]audit.Event, 0, len(output.Items))
	for _, item := range output.Items {
		var snapshot audit.EventSnapshot
		if err := unmarshalPayload(item, &snapshot); err != nil {
			return nil, nil, err
		}
		event, err := audit.RestoreEvent(snapshot)
		if err != nil {
			return nil, nil, err
		}
		events = append(events, event)
	}
	var nextCursor *string
	if len(output.LastEvaluatedKey) > 0 && len(events) > 0 {
		encoded, err := audit.EncodeCursor(events[len(events)-1])
		if err != nil {
			return nil, nil, err
		}
		nextCursor = &encoded
	}
	return events, nextCursor, nil
}
