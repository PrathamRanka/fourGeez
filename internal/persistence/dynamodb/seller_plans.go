package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

type SellerEntitlementRepository struct{ repositoryBase }
type SellerPlanRepository = SellerEntitlementRepository

func NewSellerEntitlementRepository(client Client, tableName string) *SellerEntitlementRepository {
	return &SellerEntitlementRepository{repositoryBase: newRepositoryBase(client, tableName)}
}

func NewSellerPlanRepository(client Client, tableName string) *SellerPlanRepository {
	return NewSellerEntitlementRepository(client, tableName)
}

func (repository *SellerEntitlementRepository) Get(ctx context.Context, sellerID domain.ID) (billing.SellerEntitlement, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(sellerPartitionKey(sellerID.String()), sellerPlanSortKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return billing.SellerEntitlement{}, err
	}
	var snapshot billing.SellerEntitlementSnapshot
	if err := unmarshalPayload(output.Item, &snapshot); err != nil {
		return billing.SellerEntitlement{}, err
	}
	if snapshot.SellerID != sellerID {
		return billing.SellerEntitlement{}, persistence.ErrNotFound
	}
	return billing.RestoreSellerEntitlement(snapshot)
}

func (repository *SellerEntitlementRepository) Apply(
	ctx context.Context,
	entitlement billing.SellerEntitlement,
	reconciliation billing.EntitlementReconciliation,
	expectedVersion uint64,
) error {
	entitlementItem, err := marshalSellerEntitlement(entitlement)
	if err != nil {
		return err
	}
	reconciliationRecord, err := newStoredRecord(
		sellerPartitionKey(entitlement.SellerID().String()),
		subscriptionReconciliationSortKey(reconciliation.SourceRevision),
		"subscriptionReconciliation",
		reconciliation,
	)
	if err != nil {
		return err
	}
	reconciliationItem, err := marshalStoredRecord(reconciliationRecord)
	if err != nil {
		return err
	}
	entitlementCondition := createItemCondition
	attributeNames := map[string]string(nil)
	attributeValues := map[string]types.AttributeValue(nil)
	if expectedVersion > 0 {
		entitlementCondition = "#version = :expectedVersion"
		attributeNames = map[string]string{"#version": "version"}
		attributeValues = map[string]types.AttributeValue{":expectedVersion": numberAttributeValue(expectedVersion)}
	}
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{Put: &types.Put{TableName: &repository.tableName, Item: reconciliationItem, ConditionExpression: stringPointer(createItemCondition)}},
			{Put: &types.Put{TableName: &repository.tableName, Item: entitlementItem, ConditionExpression: &entitlementCondition, ExpressionAttributeNames: attributeNames, ExpressionAttributeValues: attributeValues}},
		},
	})
	if isTransactionFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

func marshalSellerEntitlement(entitlement billing.SellerEntitlement) (map[string]types.AttributeValue, error) {
	record, err := newStoredRecord(sellerPartitionKey(entitlement.SellerID().String()), sellerPlanSortKey, "sellerEntitlement", entitlement.Snapshot())
	if err != nil {
		return nil, err
	}
	record.Version = entitlement.Version()
	record.Status = string(entitlement.Status())
	return marshalStoredRecord(record)
}
