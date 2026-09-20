package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/fourgeez/agentpay/internal/publicationops"
)

type PublicationCompletionStore struct{ repositoryBase }

func NewPublicationCompletionStore(client Client, tableName string) *PublicationCompletionStore {
	return &PublicationCompletionStore{repositoryBase: newRepositoryBase(client, tableName)}
}

func (store *PublicationCompletionStore) Completed(ctx context.Context, eventID string) (bool, error) {
	output, err := store.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &store.tableName,
		Key:            primaryKey(publicationEventPartitionKey(eventID), publicationCompletionSortKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return false, err
	}
	return len(output.Item) > 0, nil
}

func (store *PublicationCompletionStore) Complete(ctx context.Context, event publicationops.Event) error {
	record, err := newStoredRecord(
		publicationEventPartitionKey(event.EventID),
		publicationCompletionSortKey,
		"publicationCompletion",
		event,
	)
	if err != nil {
		return err
	}
	item, err := marshalStoredRecord(record)
	if err != nil {
		return err
	}
	_, err = store.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &store.tableName,
		Item:                item,
		ConditionExpression: stringPointer(createItemCondition),
	})
	if isConditionalFailure(err) {
		return nil
	}
	return err
}
