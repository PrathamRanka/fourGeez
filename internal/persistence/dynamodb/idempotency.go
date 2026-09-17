package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/fourgeez/agentpay/internal/domain"
)

// IdempotencyStore persists mutation replay records in DynamoDB.
type IdempotencyStore struct {
	repositoryBase
}

// NewIdempotencyStore creates a DynamoDB-backed idempotency store.
func NewIdempotencyStore(client Client, tableName string) *IdempotencyStore {
	return &IdempotencyStore{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// SaveIfAbsent stores the first result for a scope and idempotency key.
func (store *IdempotencyStore) SaveIfAbsent(
	ctx context.Context,
	idempotencyRecord domain.IdempotencyRecord,
) (bool, error) {
	storedRecord, err := newStoredRecord(
		idempotencyPartitionKey(idempotencyRecord.Scope),
		string(idempotencyRecord.Key),
		"idempotency",
		idempotencyRecord,
	)
	if err != nil {
		return false, err
	}

	item, err := marshalStoredRecord(storedRecord)
	if err != nil {
		return false, err
	}

	_, err = store.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &store.tableName,
		Item:                item,
		ConditionExpression: stringPointer(createItemCondition),
	})
	if isConditionalFailure(err) {
		return false, nil
	}

	return err == nil, err
}

// Load returns the stored result for a scope and idempotency key.
func (store *IdempotencyStore) Load(
	ctx context.Context,
	scope string,
	idempotencyKey domain.IdempotencyKey,
) (domain.IdempotencyRecord, bool, error) {
	output, err := store.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &store.tableName,
		Key: primaryKey(
			idempotencyPartitionKey(scope),
			string(idempotencyKey),
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return domain.IdempotencyRecord{}, false, err
	}
	if len(output.Item) == 0 {
		return domain.IdempotencyRecord{}, false, nil
	}

	var idempotencyRecord domain.IdempotencyRecord
	if err := unmarshalPayload(output.Item, &idempotencyRecord); err != nil {
		return domain.IdempotencyRecord{}, false, err
	}

	return idempotencyRecord, true, nil
}

var _ domain.IdempotencyStore = (*IdempotencyStore)(nil)
