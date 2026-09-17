package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// EvidenceRepository persists append-only evidence events in DynamoDB.
type EvidenceRepository struct {
	repositoryBase
}

// NewEvidenceRepository creates a DynamoDB-backed evidence repository.
func NewEvidenceRepository(client Client, tableName string) *EvidenceRepository {
	return &EvidenceRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Append writes an event only when its sequence and previous hash are valid.
func (repository *EvidenceRepository) Append(
	ctx context.Context,
	event evidence.Event,
) error {
	eventRecord, err := newStoredRecord(
		transactionPartitionKey(event.TransactionID.String()),
		eventSortKey(event.Sequence),
		"evidenceEvent",
		event,
	)
	if err != nil {
		return err
	}

	eventRecord.EventHash = event.EventHash.String()
	eventItem, err := marshalStoredRecord(eventRecord)
	if err != nil {
		return err
	}

	if event.Sequence == 1 {
		return repository.appendFirstEvent(ctx, eventItem)
	}
	if event.PreviousEventHash == nil {
		return persistence.ErrConditionFailed
	}

	return repository.appendFollowingEvent(ctx, event, eventItem)
}

// ListByTransaction returns evidence events in ascending sequence order.
func (repository *EvidenceRepository) ListByTransaction(
	ctx context.Context,
	transactionID domain.ID,
) ([]evidence.Event, error) {
	keyCondition := "PK = :partitionKey AND begins_with(SK, :sortKeyPrefix)"
	ascending := true
	output, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName:              &repository.tableName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":partitionKey":  stringAttributeValue(transactionPartitionKey(transactionID.String())),
			":sortKeyPrefix": stringAttributeValue("EVENT#"),
		},
		ScanIndexForward: &ascending,
	})
	if err != nil {
		return nil, err
	}

	events := make([]evidence.Event, 0, len(output.Items))
	for _, item := range output.Items {
		var event evidence.Event
		if err := unmarshalPayload(item, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

// appendFirstEvent conditionally creates sequence one.
func (repository *EvidenceRepository) appendFirstEvent(
	ctx context.Context,
	eventItem map[string]types.AttributeValue,
) error {
	_, err := repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                eventItem,
		ConditionExpression: stringPointer(createItemCondition),
	})
	if isConditionalFailure(err) {
		return persistence.ErrConditionFailed
	}

	return err
}

// appendFollowingEvent checks the prior hash and creates the next sequence atomically.
func (repository *EvidenceRepository) appendFollowingEvent(
	ctx context.Context,
	event evidence.Event,
	eventItem map[string]types.AttributeValue,
) error {
	previousHashCondition := "eventHash = :previousHash"
	_, err := repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				ConditionCheck: &types.ConditionCheck{
					TableName: &repository.tableName,
					Key: primaryKey(
						transactionPartitionKey(event.TransactionID.String()),
						eventSortKey(event.Sequence-1),
					),
					ConditionExpression: &previousHashCondition,
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":previousHash": stringAttributeValue(event.PreviousEventHash.String()),
					},
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
	})
	if isTransactionFailure(err) {
		return persistence.ErrConditionFailed
	}

	return err
}
