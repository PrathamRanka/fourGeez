package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/audit"
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

func (repository *SellerEntitlementRepository) GetLaunchEntitlementOperation(
	ctx context.Context,
	sellerID domain.ID,
	operationID string,
) (billing.LaunchEntitlementOperation, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(sellerPartitionKey(sellerID.String()), launchEntitlementOperationSortKey(operationID)),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return billing.LaunchEntitlementOperation{}, err
	}
	var operation billing.LaunchEntitlementOperation
	if err := unmarshalPayload(output.Item, &operation); err != nil {
		return billing.LaunchEntitlementOperation{}, err
	}
	if operation.SellerID != sellerID || operation.OperationID != operationID {
		return billing.LaunchEntitlementOperation{}, persistence.ErrNotFound
	}
	return operation, nil
}

func (repository *SellerEntitlementRepository) Apply(
	ctx context.Context,
	entitlement billing.SellerEntitlement,
	reconciliation billing.EntitlementReconciliation,
	expectedVersion uint64,
) error {
	items, err := repository.entitlementTransactionItems(entitlement, reconciliation, expectedVersion)
	if err != nil {
		return err
	}
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{TransactItems: items})
	if isTransactionFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

// ApplyWithAudit atomically commits an operator entitlement change and its audit event.
func (repository *SellerEntitlementRepository) ApplyWithAudit(
	ctx context.Context,
	entitlement billing.SellerEntitlement,
	reconciliation billing.EntitlementReconciliation,
	expectedVersion uint64,
	event audit.Event,
	operation billing.LaunchEntitlementOperation,
) error {
	if event.SellerID() != entitlement.SellerID() {
		return domain.NewValidationError("auditEvent", "seller", "must belong to the entitlement seller")
	}
	if operation.SellerID != entitlement.SellerID() || operation.AppliedVersion != entitlement.Version() {
		return domain.NewValidationError("launchEntitlementOperation", "binding", "must belong to the entitlement and applied version")
	}
	if operation.ExpectedVersion != expectedVersion || operation.SchemaVersion != billing.LaunchEntitlementOperationSchemaVersion ||
		event.RequestID() != operation.OperationID || event.ActorID() != operation.ActorARN ||
		len(operation.RequestSHA256) != 64 || len(operation.PlanSHA256) != 64 {
		return domain.NewValidationError("launchEntitlementOperation", "binding", "must match the reviewed request and administrator audit")
	}
	items, err := repository.entitlementTransactionItems(entitlement, reconciliation, expectedVersion)
	if err != nil {
		return err
	}
	operationRecord, err := newStoredRecord(
		sellerPartitionKey(operation.SellerID.String()),
		launchEntitlementOperationSortKey(operation.OperationID),
		"launchEntitlementOperation",
		operation,
	)
	if err != nil {
		return err
	}
	operationItem, err := marshalStoredRecord(operationRecord)
	if err != nil {
		return err
	}
	items = append(items, types.TransactWriteItem{Put: &types.Put{
		TableName:           &repository.tableName,
		Item:                operationItem,
		ConditionExpression: stringPointer(createItemCondition),
	}})
	auditRecord, err := newStoredRecord(
		sellerPartitionKey(event.SellerID().String()),
		auditEventSortKey(event.OccurredAt().Time(), event.AuditEventID().String()),
		"auditEvent",
		event.Snapshot(),
	)
	if err != nil {
		return err
	}
	auditItem, err := marshalStoredRecord(auditRecord)
	if err != nil {
		return err
	}
	items = append(items, types.TransactWriteItem{Put: &types.Put{
		TableName:           &repository.tableName,
		Item:                auditItem,
		ConditionExpression: stringPointer(createItemCondition),
	}})
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{TransactItems: items})
	if isTransactionFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

func (repository *SellerEntitlementRepository) entitlementTransactionItems(
	entitlement billing.SellerEntitlement,
	reconciliation billing.EntitlementReconciliation,
	expectedVersion uint64,
) ([]types.TransactWriteItem, error) {
	entitlementItem, err := marshalSellerEntitlement(entitlement)
	if err != nil {
		return nil, err
	}
	reconciliationRecord, err := newStoredRecord(
		sellerPartitionKey(entitlement.SellerID().String()),
		subscriptionReconciliationSortKey(reconciliation.SourceRevision),
		"subscriptionReconciliation",
		reconciliation,
	)
	if err != nil {
		return nil, err
	}
	reconciliationItem, err := marshalStoredRecord(reconciliationRecord)
	if err != nil {
		return nil, err
	}
	entitlementCondition := createItemCondition
	attributeNames := map[string]string(nil)
	attributeValues := map[string]types.AttributeValue(nil)
	if expectedVersion > 0 {
		entitlementCondition = "#version = :expectedVersion"
		attributeNames = map[string]string{"#version": "version"}
		attributeValues = map[string]types.AttributeValue{":expectedVersion": numberAttributeValue(expectedVersion)}
	}
	return []types.TransactWriteItem{
		{Put: &types.Put{TableName: &repository.tableName, Item: reconciliationItem, ConditionExpression: stringPointer(createItemCondition)}},
		{Put: &types.Put{TableName: &repository.tableName, Item: entitlementItem, ConditionExpression: &entitlementCondition, ExpressionAttributeNames: attributeNames, ExpressionAttributeValues: attributeValues}},
	}, nil
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
