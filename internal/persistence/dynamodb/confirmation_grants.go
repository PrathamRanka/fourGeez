package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

type ConfirmationGrantRepository struct{ repositoryBase }

type confirmationGrantBinding struct {
	ConfirmationGrantID domain.ID `json:"confirmationGrantId"`
	BindingHash         string    `json:"bindingHash"`
}

func NewConfirmationGrantRepository(client Client, tableName string) *ConfirmationGrantRepository {
	return &ConfirmationGrantRepository{repositoryBase: newRepositoryBase(client, tableName)}
}

func (repository *ConfirmationGrantRepository) CreateReplacing(ctx context.Context, grant authorization.ConfirmationGrant) error {
	grantRecord, err := newStoredRecord(
		mcpConfirmationPartitionKey(grant.ConfirmationGrantID.String()), profileSortKey,
		"mcpConfirmationGrant", grant,
	)
	if err != nil {
		return err
	}
	grantRecord.Version = grant.Version
	grantItem, err := marshalStoredRecord(grantRecord)
	if err != nil {
		return err
	}
	bindingRecord, err := newStoredRecord(
		sellerPartitionKey(grant.SellerID.String()), mcpConfirmationBindingSortKey(grant.BindingHash),
		"mcpConfirmationBinding", confirmationGrantBinding{ConfirmationGrantID: grant.ConfirmationGrantID, BindingHash: grant.BindingHash},
	)
	if err != nil {
		return err
	}
	bindingRecord.ConfirmationGrantID = grant.ConfirmationGrantID.String()
	bindingItem, err := marshalStoredRecord(bindingRecord)
	if err != nil {
		return err
	}
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{
		{Put: &types.Put{TableName: &repository.tableName, Item: grantItem, ConditionExpression: stringPointer(createItemCondition)}},
		{Put: &types.Put{TableName: &repository.tableName, Item: bindingItem}},
	}})
	if isTransactionFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

func (repository *ConfirmationGrantRepository) Get(ctx context.Context, grantID domain.ID) (authorization.ConfirmationGrant, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(mcpConfirmationPartitionKey(grantID.String()), profileSortKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return authorization.ConfirmationGrant{}, err
	}
	if len(output.Item) == 0 {
		return authorization.ConfirmationGrant{}, persistence.ErrNotFound
	}
	var grant authorization.ConfirmationGrant
	if err := unmarshalPayload(output.Item, &grant); err != nil {
		return authorization.ConfirmationGrant{}, err
	}
	return grant, nil
}

func (repository *ConfirmationGrantRepository) Consume(
	ctx context.Context,
	grant authorization.ConfirmationGrant,
	expectedVersion uint64,
) error {
	grantRecord, err := newStoredRecord(
		mcpConfirmationPartitionKey(grant.ConfirmationGrantID.String()), profileSortKey,
		"mcpConfirmationGrant", grant,
	)
	if err != nil {
		return err
	}
	grantRecord.Version = grant.Version
	grantItem, err := marshalStoredRecord(grantRecord)
	if err != nil {
		return err
	}
	condition := "#version = :expectedVersion"
	bindingCondition := "confirmationGrantId = :confirmationGrantId"
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{
		{ConditionCheck: &types.ConditionCheck{
			TableName:           &repository.tableName,
			Key:                 primaryKey(sellerPartitionKey(grant.SellerID.String()), mcpConfirmationBindingSortKey(grant.BindingHash)),
			ConditionExpression: &bindingCondition,
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":confirmationGrantId": stringAttributeValue(grant.ConfirmationGrantID.String()),
			},
		}},
		{Put: &types.Put{
			TableName: &repository.tableName, Item: grantItem, ConditionExpression: &condition,
			ExpressionAttributeNames:  map[string]string{"#version": "version"},
			ExpressionAttributeValues: map[string]types.AttributeValue{":expectedVersion": numberAttributeValue(expectedVersion)},
		}},
	}})
	if isTransactionFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

var _ authorization.ConfirmationGrantRepository = (*ConfirmationGrantRepository)(nil)
