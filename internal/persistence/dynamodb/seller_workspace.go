package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/sellerworkspace"
)

const (
	sellerWorkspaceSortKey         = "WORKSPACE"
	sellerWorkspaceCreateCondition = "attribute_not_exists(PK) AND attribute_not_exists(SK)"
)

type SellerWorkspaceRepository struct {
	repositoryBase
}

func NewSellerWorkspaceRepository(client Client, tableName string) *SellerWorkspaceRepository {
	return &SellerWorkspaceRepository{repositoryBase: newRepositoryBase(client, tableName)}
}

func (repository *SellerWorkspaceRepository) Get(ctx context.Context, sellerID domain.ID) (sellerworkspace.WorkspaceState, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(sellerPartitionKey(sellerID.String()), sellerWorkspaceSortKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return sellerworkspace.WorkspaceState{}, err
	}
	var state sellerworkspace.WorkspaceState
	if err := unmarshalPayload(output.Item, &state); err != nil {
		return sellerworkspace.WorkspaceState{}, err
	}
	return state, nil
}

func (repository *SellerWorkspaceRepository) Put(ctx context.Context, state sellerworkspace.WorkspaceState, expectedVersion uint64) error {
	record, err := newStoredRecord(sellerPartitionKey(state.SellerID.String()), sellerWorkspaceSortKey, "sellerWorkspace", state)
	if err != nil {
		return err
	}
	record.Version = state.Version
	item, err := marshalStoredRecord(record)
	if err != nil {
		return err
	}
	input := &awssdk.PutItemInput{TableName: &repository.tableName, Item: item}
	if expectedVersion == 0 {
		input.ConditionExpression = stringPointer(sellerWorkspaceCreateCondition)
	} else {
		input.ConditionExpression = stringPointer("#version = :expectedVersion")
		input.ExpressionAttributeNames = map[string]string{"#version": "version"}
		input.ExpressionAttributeValues = map[string]types.AttributeValue{":expectedVersion": numberAttributeValue(expectedVersion)}
	}
	_, err = repository.client.PutItem(ctx, input)
	if isConditionalFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}
