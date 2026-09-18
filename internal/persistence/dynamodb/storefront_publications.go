package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/storefront"
)

const storefrontPublicationSortKey = "STOREFRONT_PUBLICATION"

type StorefrontPublicationRepository struct{ repositoryBase }

func NewStorefrontPublicationRepository(client Client, tableName string) *StorefrontPublicationRepository {
	return &StorefrontPublicationRepository{repositoryBase: newRepositoryBase(client, tableName)}
}

func (repository *StorefrontPublicationRepository) Get(ctx context.Context, sellerID domain.ID) (storefront.PublicationState, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{TableName: &repository.tableName, Key: primaryKey(sellerPartitionKey(sellerID.String()), storefrontPublicationSortKey), ConsistentRead: boolPointer(true)})
	if err != nil {
		return storefront.PublicationState{}, err
	}
	var state storefront.PublicationState
	if err := unmarshalPayload(output.Item, &state); err != nil {
		return storefront.PublicationState{}, err
	}
	return state, nil
}

func (repository *StorefrontPublicationRepository) Put(ctx context.Context, state storefront.PublicationState, expectedVersion uint64) error {
	record, err := newStoredRecord(sellerPartitionKey(state.SellerID.String()), storefrontPublicationSortKey, "storefrontPublication", state)
	if err != nil {
		return err
	}
	record.Version = state.Version
	record.PublicationRevision = state.PublicationRevision
	item, err := marshalStoredRecord(record)
	if err != nil {
		return err
	}
	input := &awssdk.PutItemInput{TableName: &repository.tableName, Item: item}
	if expectedVersion == 0 {
		input.ConditionExpression = stringPointer(createItemCondition)
	} else {
		input.ConditionExpression = stringPointer("#version = :expectedVersion AND #revision < :nextRevision")
		input.ExpressionAttributeNames = map[string]string{"#version": "version", "#revision": "publicationRevision"}
		input.ExpressionAttributeValues = map[string]types.AttributeValue{":expectedVersion": numberAttributeValue(expectedVersion), ":nextRevision": numberAttributeValue(state.PublicationRevision)}
	}
	_, err = repository.client.PutItem(ctx, input)
	if isConditionalFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}
