package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// UsageMeterEventRepository persists immutable seller usage facts.
type UsageMeterEventRepository struct {
	repositoryBase
}

// NewUsageMeterEventRepository creates a DynamoDB usage repository.
func NewUsageMeterEventRepository(
	client Client,
	tableName string,
) *UsageMeterEventRepository {
	return &UsageMeterEventRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// CreateIfAbsent atomically claims one meter source and stores its event.
func (repository *UsageMeterEventRepository) CreateIfAbsent(
	ctx context.Context,
	event billing.UsageMeterEvent,
) (billing.UsageMeterEvent, bool, error) {
	partitionKey := sellerPartitionKey(event.SellerID().String())
	eventSortKey := usageMeterSortKey(
		event.OccurredAt().Time(),
		event.MeterEventID().String(),
	)
	claimRecord, err := newStoredRecord(
		partitionKey,
		usageMeterSourceSortKey(
			string(event.MeterName()),
			event.SourceTransactionID().String(),
		),
		"usageMeterClaim",
		usageMeterClaim{EventSortKey: eventSortKey},
	)
	if err != nil {
		return billing.UsageMeterEvent{}, false, err
	}
	eventRecord, err := newStoredRecord(
		partitionKey,
		eventSortKey,
		"usageMeterEvent",
		event.Snapshot(),
	)
	if err != nil {
		return billing.UsageMeterEvent{}, false, err
	}
	claimItem, err := marshalStoredRecord(claimRecord)
	if err != nil {
		return billing.UsageMeterEvent{}, false, err
	}
	eventItem, err := marshalStoredRecord(eventRecord)
	if err != nil {
		return billing.UsageMeterEvent{}, false, err
	}
	_, err = repository.client.TransactWriteItems(
		ctx,
		&awssdk.TransactWriteItemsInput{
			TransactItems: []types.TransactWriteItem{
				{
					Put: &types.Put{
						TableName:           &repository.tableName,
						Item:                claimItem,
						ConditionExpression: stringPointer(createItemCondition),
					},
				},
				{
					Put: &types.Put{
						TableName:           &repository.tableName,
						Item:                eventItem,
						ConditionExpression: stringPointer(createItemCondition),
					},
				},
			},
		},
	)
	if err == nil {
		return event, true, nil
	}
	if !isTransactionFailure(err) {
		return billing.UsageMeterEvent{}, false, err
	}
	stored, loadErr := repository.getBySource(ctx, event)
	if loadErr != nil {
		return billing.UsageMeterEvent{}, false, persistence.ErrAlreadyExists
	}
	return stored, false, nil
}

// ListBySellerWindow queries bounded usage without scanning the table.
func (repository *UsageMeterEventRepository) ListBySellerWindow(
	ctx context.Context,
	sellerID domain.ID,
	from domain.Timestamp,
	to domain.Timestamp,
	limit int,
) ([]billing.UsageMeterEvent, error) {
	condition := "PK = :pk AND SK BETWEEN :from AND :to"
	queryLimit := int32(limit)
	output, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName:              &repository.tableName,
		KeyConditionExpression: &condition,
		Limit:                  &queryLimit,
		ConsistentRead:         boolPointer(true),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":   stringAttributeValue(sellerPartitionKey(sellerID.String())),
			":from": stringAttributeValue("METER#" + from.String()),
			":to":   stringAttributeValue("METER#" + to.String()),
		},
	})
	if err != nil {
		return nil, err
	}
	result := make([]billing.UsageMeterEvent, 0, len(output.Items))
	for _, item := range output.Items {
		var snapshot billing.UsageMeterEventSnapshot
		if err := unmarshalPayload(item, &snapshot); err != nil {
			return nil, err
		}
		event, err := billing.RestoreUsageMeterEvent(snapshot)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, nil
}

// usageMeterClaim points a unique source claim at its immutable event item.
type usageMeterClaim struct {
	EventSortKey string `json:"eventSortKey"`
}

// getBySource resolves an existing meter source to its immutable event.
func (repository *UsageMeterEventRepository) getBySource(
	ctx context.Context,
	event billing.UsageMeterEvent,
) (billing.UsageMeterEvent, error) {
	partitionKey := sellerPartitionKey(event.SellerID().String())
	claimOutput, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			partitionKey,
			usageMeterSourceSortKey(
				string(event.MeterName()),
				event.SourceTransactionID().String(),
			),
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return billing.UsageMeterEvent{}, err
	}
	var claim usageMeterClaim
	if err := unmarshalPayload(claimOutput.Item, &claim); err != nil {
		return billing.UsageMeterEvent{}, err
	}
	eventOutput, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(partitionKey, claim.EventSortKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return billing.UsageMeterEvent{}, err
	}
	var snapshot billing.UsageMeterEventSnapshot
	if err := unmarshalPayload(eventOutput.Item, &snapshot); err != nil {
		return billing.UsageMeterEvent{}, err
	}
	return billing.RestoreUsageMeterEvent(snapshot)
}
