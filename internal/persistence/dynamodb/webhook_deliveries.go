package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// WebhookDeliveryRepository persists webhook deliveries and event claims.
type WebhookDeliveryRepository struct {
	repositoryBase
}

// NewWebhookDeliveryRepository creates a DynamoDB delivery repository.
func NewWebhookDeliveryRepository(
	client Client,
	tableName string,
) *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// CreateIfAbsent atomically claims one subscription event and stores its delivery.
func (repository *WebhookDeliveryRepository) CreateIfAbsent(
	ctx context.Context,
	delivery notifications.Delivery,
) (notifications.Delivery, bool, error) {
	claimItem, deliveryItem, err := marshalWebhookDeliveryItems(delivery)
	if err != nil {
		return notifications.Delivery{}, false, err
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
						Item:                deliveryItem,
						ConditionExpression: stringPointer(createItemCondition),
					},
				},
			},
		},
	)
	if err == nil {
		return delivery, true, nil
	}
	if !isTransactionFailure(err) {
		return notifications.Delivery{}, false, err
	}
	existing, loadErr := repository.getByEvent(
		ctx,
		delivery.SellerID(),
		delivery.SubscriptionID(),
		delivery.Event().EventID,
	)
	if loadErr == nil {
		return existing, false, nil
	}
	return notifications.Delivery{}, false, persistence.ErrAlreadyExists
}

// Get loads one seller-owned webhook delivery.
func (repository *WebhookDeliveryRepository) Get(
	ctx context.Context,
	sellerID domain.ID,
	deliveryID domain.ID,
) (notifications.Delivery, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			sellerPartitionKey(sellerID.String()),
			webhookDeliverySortKey(deliveryID.String()),
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return notifications.Delivery{}, err
	}
	var snapshot notifications.DeliverySnapshot
	if err := unmarshalPayload(output.Item, &snapshot); err != nil {
		return notifications.Delivery{}, err
	}
	if snapshot.SellerID != sellerID || snapshot.DeliveryID != deliveryID {
		return notifications.Delivery{}, persistence.ErrNotFound
	}
	return notifications.RestoreDelivery(snapshot)
}

// Update replaces a delivery only when its stored version matches.
func (repository *WebhookDeliveryRepository) Update(
	ctx context.Context,
	delivery notifications.Delivery,
	expectedVersion uint64,
) error {
	record, err := newStoredRecord(
		sellerPartitionKey(delivery.SellerID().String()),
		webhookDeliverySortKey(delivery.DeliveryID().String()),
		"webhookDelivery",
		delivery.Snapshot(),
	)
	if err != nil {
		return err
	}
	record.Version = delivery.Version()
	record.Status = string(delivery.Status())
	item, err := marshalStoredRecord(record)
	if err != nil {
		return err
	}
	condition := "#version = :expectedVersion"
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                item,
		ConditionExpression: &condition,
		ExpressionAttributeNames: map[string]string{
			"#version": "version",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expectedVersion": numberAttributeValue(expectedVersion),
		},
	})
	if isConditionalFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

// ListBySeller queries one bounded newest-first delivery page without scanning.
func (repository *WebhookDeliveryRepository) ListBySeller(
	ctx context.Context,
	sellerID domain.ID,
	limit int,
	cursor string,
) ([]notifications.Delivery, *string, error) {
	keyCondition := "PK = :partitionKey AND begins_with(SK, :sortKeyPrefix)"
	ascending := false
	queryLimit := int32(limit)
	input := &awssdk.QueryInput{
		TableName:              &repository.tableName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":partitionKey":  stringAttributeValue(sellerPartitionKey(sellerID.String())),
			":sortKeyPrefix": stringAttributeValue("WEBHOOK_DELIVERY#"),
		},
		ConsistentRead:   boolPointer(true),
		ScanIndexForward: &ascending,
		Limit:            &queryLimit,
	}
	if cursor != "" {
		cursorID, err := domain.ParseID(cursor, domain.WebhookDeliveryIDPrefix)
		if err != nil {
			return nil, nil, domain.NewValidationError("cursor", "format", "must identify a webhook delivery")
		}
		input.ExclusiveStartKey = primaryKey(
			sellerPartitionKey(sellerID.String()),
			webhookDeliverySortKey(cursorID.String()),
		)
	}
	output, err := repository.client.Query(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	deliveries := make([]notifications.Delivery, 0, len(output.Items))
	for _, item := range output.Items {
		var snapshot notifications.DeliverySnapshot
		if err := unmarshalPayload(item, &snapshot); err != nil {
			return nil, nil, err
		}
		delivery, err := notifications.RestoreDelivery(snapshot)
		if err != nil {
			return nil, nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	var nextCursor *string
	if len(output.LastEvaluatedKey) > 0 && len(deliveries) > 0 {
		value := deliveries[len(deliveries)-1].DeliveryID().String()
		nextCursor = &value
	}
	return deliveries, nextCursor, nil
}

// getByEvent follows one subscription-event claim to its stable delivery.
func (repository *WebhookDeliveryRepository) getByEvent(
	ctx context.Context,
	sellerID domain.ID,
	subscriptionID domain.ID,
	eventID domain.ID,
) (notifications.Delivery, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			sellerPartitionKey(sellerID.String()),
			webhookEventClaimSortKey(subscriptionID.String(), eventID.String()),
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return notifications.Delivery{}, err
	}
	var claim webhookDeliveryClaim
	if err := unmarshalPayload(output.Item, &claim); err != nil {
		return notifications.Delivery{}, err
	}
	deliveryID, err := domain.ParseID(claim.DeliveryID, domain.WebhookDeliveryIDPrefix)
	if err != nil {
		return notifications.Delivery{}, err
	}
	return repository.Get(ctx, sellerID, deliveryID)
}

// webhookDeliveryClaim links one unique subscription event to its delivery.
type webhookDeliveryClaim struct {
	DeliveryID string `json:"deliveryId"`
}

// marshalWebhookDeliveryItems creates the claim and delivery transaction items.
func marshalWebhookDeliveryItems(
	delivery notifications.Delivery,
) (map[string]types.AttributeValue, map[string]types.AttributeValue, error) {
	partitionKey := sellerPartitionKey(delivery.SellerID().String())
	claimRecord, err := newStoredRecord(
		partitionKey,
		webhookEventClaimSortKey(
			delivery.SubscriptionID().String(),
			delivery.Event().EventID.String(),
		),
		"webhookDeliveryClaim",
		webhookDeliveryClaim{DeliveryID: delivery.DeliveryID().String()},
	)
	if err != nil {
		return nil, nil, err
	}
	claimItem, err := marshalStoredRecord(claimRecord)
	if err != nil {
		return nil, nil, err
	}
	deliveryRecord, err := newStoredRecord(
		partitionKey,
		webhookDeliverySortKey(delivery.DeliveryID().String()),
		"webhookDelivery",
		delivery.Snapshot(),
	)
	if err != nil {
		return nil, nil, err
	}
	deliveryRecord.Version = delivery.Version()
	deliveryRecord.Status = string(delivery.Status())
	deliveryItem, err := marshalStoredRecord(deliveryRecord)
	return claimItem, deliveryItem, err
}
