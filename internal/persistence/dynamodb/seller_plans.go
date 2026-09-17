package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// SellerPlanRepository persists one billing-plan assignment per seller.
type SellerPlanRepository struct {
	repositoryBase
}

// NewSellerPlanRepository creates a DynamoDB seller plan repository.
func NewSellerPlanRepository(
	client Client,
	tableName string,
) *SellerPlanRepository {
	return &SellerPlanRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Get loads one seller plan assignment.
func (repository *SellerPlanRepository) Get(
	ctx context.Context,
	sellerID domain.ID,
) (billing.SellerPlan, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			sellerPartitionKey(sellerID.String()),
			sellerPlanSortKey,
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return billing.SellerPlan{}, err
	}
	var snapshot billing.SellerPlanSnapshot
	if err := unmarshalPayload(output.Item, &snapshot); err != nil {
		return billing.SellerPlan{}, err
	}
	if snapshot.SellerID != sellerID {
		return billing.SellerPlan{}, persistence.ErrNotFound
	}
	return billing.RestoreSellerPlan(snapshot)
}

// CreateIfAbsent stores the default assignment with a conditional write.
func (repository *SellerPlanRepository) CreateIfAbsent(
	ctx context.Context,
	assignment billing.SellerPlan,
) (billing.SellerPlan, bool, error) {
	item, err := marshalSellerPlan(assignment)
	if err != nil {
		return billing.SellerPlan{}, false, err
	}
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                item,
		ConditionExpression: stringPointer(createItemCondition),
	})
	if err == nil {
		return assignment, true, nil
	}
	if !isConditionalFailure(err) {
		return billing.SellerPlan{}, false, err
	}
	stored, loadErr := repository.Get(ctx, assignment.SellerID())
	if loadErr != nil {
		return billing.SellerPlan{}, false, loadErr
	}
	return stored, false, nil
}

// Update replaces one assignment only when its version matches.
func (repository *SellerPlanRepository) Update(
	ctx context.Context,
	assignment billing.SellerPlan,
	expectedVersion uint64,
) error {
	item, err := marshalSellerPlan(assignment)
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

// marshalSellerPlan creates the documented seller-scoped plan item.
func marshalSellerPlan(
	assignment billing.SellerPlan,
) (map[string]types.AttributeValue, error) {
	record, err := newStoredRecord(
		sellerPartitionKey(assignment.SellerID().String()),
		sellerPlanSortKey,
		"sellerPlan",
		assignment.Snapshot(),
	)
	if err != nil {
		return nil, err
	}
	record.Version = assignment.Version()
	record.Status = string(assignment.Status())
	return marshalStoredRecord(record)
}
