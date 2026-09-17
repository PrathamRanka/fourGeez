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
		destination,
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
	var destination settlement.PaymentDestination
	if err := unmarshalPayload(output.Item, &destination); err != nil {
		return settlement.PaymentDestination{}, err
	}
	return destination, nil
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
		var destination settlement.PaymentDestination
		if err := unmarshalPayload(item, &destination); err != nil {
			return nil, err
		}
		destinations = append(destinations, destination)
	}
	return destinations, nil
}
