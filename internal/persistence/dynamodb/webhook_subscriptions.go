package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// WebhookSubscriptionRepository persists seller webhook subscriptions.
type WebhookSubscriptionRepository struct{ repositoryBase }

// NewWebhookSubscriptionRepository creates a DynamoDB subscription repository.
func NewWebhookSubscriptionRepository(
	client Client,
	tableName string,
) *WebhookSubscriptionRepository {
	return &WebhookSubscriptionRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Create stores one subscription under its seller partition.
func (repository *WebhookSubscriptionRepository) Create(
	ctx context.Context,
	subscription notifications.Subscription,
) error {
	record, err := newStoredRecord(
		sellerPartitionKey(subscription.SellerID().String()),
		webhookSubscriptionSortKey(subscription.SubscriptionID().String()),
		"webhookSubscription",
		subscription.Snapshot(),
	)
	if err != nil {
		return err
	}
	record.Version = subscription.Version()
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

// Get loads one seller-owned webhook subscription.
func (repository *WebhookSubscriptionRepository) Get(
	ctx context.Context,
	sellerID domain.ID,
	subscriptionID domain.ID,
) (notifications.Subscription, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			sellerPartitionKey(sellerID.String()),
			webhookSubscriptionSortKey(subscriptionID.String()),
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return notifications.Subscription{}, err
	}
	var snapshot notifications.SubscriptionSnapshot
	if err := unmarshalPayload(output.Item, &snapshot); err != nil {
		return notifications.Subscription{}, err
	}
	if snapshot.SellerID != sellerID || snapshot.SubscriptionID != subscriptionID {
		return notifications.Subscription{}, persistence.ErrNotFound
	}
	return notifications.RestoreSubscription(snapshot)
}

// ListBySeller queries subscriptions without scanning the table.
func (repository *WebhookSubscriptionRepository) ListBySeller(
	ctx context.Context,
	sellerID domain.ID,
) ([]notifications.Subscription, error) {
	keyCondition := "PK = :partitionKey AND begins_with(SK, :sortKeyPrefix)"
	output, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName:              &repository.tableName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":partitionKey":  stringAttributeValue(sellerPartitionKey(sellerID.String())),
			":sortKeyPrefix": stringAttributeValue("WEBHOOK#"),
		},
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return nil, err
	}
	result := make([]notifications.Subscription, 0, len(output.Items))
	for _, item := range output.Items {
		var snapshot notifications.SubscriptionSnapshot
		if err := unmarshalPayload(item, &snapshot); err != nil {
			return nil, err
		}
		subscription, err := notifications.RestoreSubscription(snapshot)
		if err != nil {
			return nil, err
		}
		result = append(result, subscription)
	}
	return result, nil
}
