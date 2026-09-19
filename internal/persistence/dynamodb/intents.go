package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// PurchaseIntentRepository persists immutable purchase intents in DynamoDB.
type PurchaseIntentRepository struct {
	repositoryBase
}

// Update replaces lifecycle metadata when the stored version matches.
func (repository *PurchaseIntentRepository) Update(ctx context.Context, purchaseIntent intents.PurchaseIntent, expectedVersion uint64) error {
	record, err := newStoredRecord(intentPartitionKey(purchaseIntent.IntentID().String()), profileSortKey, "purchaseIntent", purchaseIntent.Snapshot())
	if err != nil {
		return err
	}
	record.Version = purchaseIntent.Version()
	item, err := marshalStoredRecord(record)
	if err != nil {
		return err
	}
	condition := "#version = :expectedVersion"
	if expectedVersion == 1 {
		condition = "attribute_not_exists(#version) OR #version = :expectedVersion"
	}
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:                 &repository.tableName,
		Item:                      item,
		ConditionExpression:       &condition,
		ExpressionAttributeNames:  map[string]string{"#version": "version"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":expectedVersion": numberAttributeValue(expectedVersion)},
	})
	if isConditionalFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

// NewPurchaseIntentRepository creates a DynamoDB-backed intent repository.
func NewPurchaseIntentRepository(client Client, tableName string) *PurchaseIntentRepository {
	return &PurchaseIntentRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Create inserts an immutable purchase intent when its identifier is unused.
func (repository *PurchaseIntentRepository) Create(
	ctx context.Context,
	purchaseIntent intents.PurchaseIntent,
) error {
	intentRecord, err := newStoredRecord(
		intentPartitionKey(purchaseIntent.IntentID().String()),
		profileSortKey,
		"purchaseIntent",
		purchaseIntent.Snapshot(),
	)
	if err != nil {
		return err
	}
	intentRecord.Version = purchaseIntent.Version()

	intentItem, err := marshalStoredRecord(intentRecord)
	if err != nil {
		return err
	}

	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                intentItem,
		ConditionExpression: stringPointer(createItemCondition),
	})
	if isConditionalFailure(err) {
		return persistence.ErrAlreadyExists
	}

	return err
}

// Get loads and validates an immutable purchase intent snapshot.
func (repository *PurchaseIntentRepository) Get(
	ctx context.Context,
	intentID domain.ID,
) (intents.PurchaseIntent, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			intentPartitionKey(intentID.String()),
			profileSortKey,
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return intents.PurchaseIntent{}, err
	}

	var snapshot intents.Snapshot
	if err := unmarshalPayload(output.Item, &snapshot); err != nil {
		return intents.PurchaseIntent{}, err
	}

	return intents.Restore(snapshot)
}
