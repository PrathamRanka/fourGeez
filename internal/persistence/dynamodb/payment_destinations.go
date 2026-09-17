package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/settlement"
)

// PaymentDestinationRepository persists seller payment destinations in DynamoDB.
type PaymentDestinationRepository struct {
	repositoryBase
}

// NewPaymentDestinationRepository creates a DynamoDB destination repository.
func NewPaymentDestinationRepository(
	client Client,
	tableName string,
) *PaymentDestinationRepository {
	return &PaymentDestinationRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Create inserts one destination under its owning seller partition.
func (repository *PaymentDestinationRepository) Create(
	ctx context.Context,
	destination settlement.PaymentDestination,
) error {
	record, err := newStoredRecord(
		sellerPartitionKey(destination.SellerID.String()),
		paymentDestinationSortKey(destination.DestinationID.String()),
		"paymentDestination",
		destination.Snapshot(),
	)
	if err != nil {
		return err
	}
	record.Version = destination.Version
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

// Get loads one destination by seller and destination identifier.
func (repository *PaymentDestinationRepository) Get(
	ctx context.Context,
	sellerID domain.ID,
	destinationID domain.ID,
) (settlement.PaymentDestination, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			sellerPartitionKey(sellerID.String()),
			paymentDestinationSortKey(destinationID.String()),
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return settlement.PaymentDestination{}, err
	}
	var snapshot settlement.PaymentDestinationSnapshot
	if err := unmarshalPayload(output.Item, &snapshot); err != nil {
		return settlement.PaymentDestination{}, err
	}
	return settlement.RestorePaymentDestination(snapshot)
}

// ListBySeller queries destination items without scanning the table.
func (repository *PaymentDestinationRepository) ListBySeller(
	ctx context.Context,
	sellerID domain.ID,
) ([]settlement.PaymentDestination, error) {
	keyCondition := "PK = :partitionKey AND begins_with(SK, :sortKeyPrefix)"
	output, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName:              &repository.tableName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":partitionKey": stringAttributeValue(
				sellerPartitionKey(sellerID.String()),
			),
			":sortKeyPrefix": stringAttributeValue("DESTINATION#"),
		},
	})
	if err != nil {
		return nil, err
	}
	destinations := make([]settlement.PaymentDestination, 0, len(output.Items))
	for _, item := range output.Items {
		var snapshot settlement.PaymentDestinationSnapshot
		if err := unmarshalPayload(item, &snapshot); err != nil {
			return nil, err
		}
		destination, err := settlement.RestorePaymentDestination(snapshot)
		if err != nil {
			return nil, err
		}
		destinations = append(destinations, destination)
	}
	return destinations, nil
}

// SaveChallenge replaces one destination when its stored version matches.
func (repository *PaymentDestinationRepository) SaveChallenge(
	ctx context.Context,
	destination settlement.PaymentDestination,
	expectedVersion uint64,
) error {
	item, err := repository.destinationItem(destination)
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

// Activate atomically updates the active-pair claim and destination states.
func (repository *PaymentDestinationRepository) Activate(
	ctx context.Context,
	activation settlement.PaymentDestinationActivation,
) error {
	destinationItem, err := repository.destinationItem(activation.Destination)
	if err != nil {
		return err
	}
	claimItem, err := repository.activeDestinationClaimItem(activation.Destination)
	if err != nil {
		return err
	}
	claimCondition := "attribute_not_exists(PK) AND attribute_not_exists(SK)"
	claimValues := map[string]types.AttributeValue{}
	if activation.RotatedDestination != nil {
		claimCondition = "destinationId = :expectedDestinationId"
		claimValues[":expectedDestinationId"] = stringAttributeValue(
			activation.RotatedDestination.DestinationID.String(),
		)
	}
	transactionItems := []types.TransactWriteItem{
		{
			Put: &types.Put{
				TableName:                 &repository.tableName,
				Item:                      claimItem,
				ConditionExpression:       &claimCondition,
				ExpressionAttributeValues: claimValues,
			},
		},
		{
			Put: repository.versionedDestinationPut(
				destinationItem,
				activation.ExpectedVersion,
			),
		},
	}
	if activation.RotatedDestination != nil {
		rotatedItem, marshalErr := repository.destinationItem(*activation.RotatedDestination)
		if marshalErr != nil {
			return marshalErr
		}
		transactionItems = append(
			transactionItems,
			types.TransactWriteItem{
				Put: repository.versionedDestinationPut(
					rotatedItem,
					activation.RotatedExpectedVersion,
				),
			},
		)
	}
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: transactionItems,
	})
	if isTransactionFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

// destinationItem builds the seller-scoped destination record.
func (repository *PaymentDestinationRepository) destinationItem(
	destination settlement.PaymentDestination,
) (map[string]types.AttributeValue, error) {
	record, err := newStoredRecord(
		sellerPartitionKey(destination.SellerID.String()),
		paymentDestinationSortKey(destination.DestinationID.String()),
		"paymentDestination",
		destination.Snapshot(),
	)
	if err != nil {
		return nil, err
	}
	record.Version = destination.Version
	record.Status = string(destination.Status)
	return marshalStoredRecord(record)
}

// activeDestinationClaimItem builds the unique active-pair claim record.
func (repository *PaymentDestinationRepository) activeDestinationClaimItem(
	destination settlement.PaymentDestination,
) (map[string]types.AttributeValue, error) {
	record, err := newStoredRecord(
		sellerPartitionKey(destination.SellerID.String()),
		activePaymentDestinationSortKey(destination.Asset, destination.Network),
		"activePaymentDestination",
		destination.DestinationID.String(),
	)
	if err != nil {
		return nil, err
	}
	record.DestinationID = destination.DestinationID.String()
	return marshalStoredRecord(record)
}

// versionedDestinationPut creates one optimistic destination write.
func (repository *PaymentDestinationRepository) versionedDestinationPut(
	item map[string]types.AttributeValue,
	expectedVersion uint64,
) *types.Put {
	condition := "#version = :expectedVersion"
	return &types.Put{
		TableName:           &repository.tableName,
		Item:                item,
		ConditionExpression: &condition,
		ExpressionAttributeNames: map[string]string{
			"#version": "version",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expectedVersion": numberAttributeValue(expectedVersion),
		},
	}
}
