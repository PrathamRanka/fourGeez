package dynamodb

import (
	"context"
	"errors"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/operations"
)

const quotaIncrementCondition = "attribute_not_exists(#count) OR #count < :limit"

// QuotaCounterRepository persists atomic monthly quota counters and source claims.
type QuotaCounterRepository struct {
	repositoryBase
}

// NewQuotaCounterRepository creates a DynamoDB quota repository.
func NewQuotaCounterRepository(
	client Client,
	tableName string,
) *QuotaCounterRepository {
	return &QuotaCounterRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Increment consumes one unit with a conditional atomic update.
func (repository *QuotaCounterRepository) Increment(
	ctx context.Context,
	request operations.CounterRequest,
) error {
	_, err := repository.client.UpdateItem(ctx, repository.updateInput(request))
	var conditionFailure *types.ConditionalCheckFailedException
	if errors.As(err, &conditionFailure) {
		return domain.ErrRateLimitExceeded
	}
	return err
}

// IncrementUnique atomically claims a source and consumes one quota unit.
func (repository *QuotaCounterRepository) IncrementUnique(
	ctx context.Context,
	request operations.CounterRequest,
	source string,
) error {
	period := request.PeriodStart.Time().UTC().Format("2006-01")
	partitionKey := sellerPartitionKey(request.SellerID.String())
	claimKey := quotaClaimSortKey(period, string(request.QuotaName), source)
	_, err := repository.client.TransactWriteItems(
		ctx,
		&awssdk.TransactWriteItemsInput{
			TransactItems: []types.TransactWriteItem{
				{
					Put: &types.Put{
						TableName: &repository.tableName,
						Item: map[string]types.AttributeValue{
							"PK":         stringAttributeValue(partitionKey),
							"SK":         stringAttributeValue(claimKey),
							"entityType": stringAttributeValue("quotaClaim"),
						},
						ConditionExpression: stringPointer(createItemCondition),
					},
				},
				{
					Update: repository.transactionUpdate(request),
				},
			},
		},
	)
	if err == nil {
		return nil
	}
	if !isTransactionFailure(err) {
		return err
	}
	replayed, loadErr := repository.claimExists(ctx, partitionKey, claimKey)
	if loadErr != nil {
		return loadErr
	}
	if replayed {
		return nil
	}
	return domain.ErrRateLimitExceeded
}

// updateInput builds the standalone atomic quota update.
func (repository *QuotaCounterRepository) updateInput(
	request operations.CounterRequest,
) *awssdk.UpdateItemInput {
	update := repository.transactionUpdate(request)
	return &awssdk.UpdateItemInput{
		TableName:                 update.TableName,
		Key:                       update.Key,
		UpdateExpression:          update.UpdateExpression,
		ConditionExpression:       update.ConditionExpression,
		ExpressionAttributeNames:  update.ExpressionAttributeNames,
		ExpressionAttributeValues: update.ExpressionAttributeValues,
	}
}

// transactionUpdate builds the counter update shared by both write paths.
func (repository *QuotaCounterRepository) transactionUpdate(
	request operations.CounterRequest,
) *types.Update {
	period := request.PeriodStart.Time().UTC().Format("2006-01")
	updateExpression := "SET #count = if_not_exists(#count, :zero) + :one, #limit = :limit, periodStart = :periodStart, periodEnd = :periodEnd, updatedAt = :updatedAt, entityType = :entityType"
	conditionExpression := quotaIncrementCondition
	return &types.Update{
		TableName: &repository.tableName,
		Key: primaryKey(
			sellerPartitionKey(request.SellerID.String()),
			quotaCounterSortKey(period, string(request.QuotaName)),
		),
		UpdateExpression:    &updateExpression,
		ConditionExpression: &conditionExpression,
		ExpressionAttributeNames: map[string]string{
			"#count": "count",
			"#limit": "limit",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero":        numberAttributeValue(0),
			":one":         numberAttributeValue(1),
			":limit":       numberAttributeValue(request.Limit),
			":periodStart": stringAttributeValue(request.PeriodStart.String()),
			":periodEnd":   stringAttributeValue(request.PeriodEnd.String()),
			":updatedAt":   stringAttributeValue(request.UpdatedAt.String()),
			":entityType":  stringAttributeValue("quotaCounter"),
		},
	}
}

// claimExists reports whether a failed transaction was an idempotent replay.
func (repository *QuotaCounterRepository) claimExists(
	ctx context.Context,
	partitionKey string,
	claimKey string,
) (bool, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(partitionKey, claimKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return false, err
	}
	return len(output.Item) > 0, nil
}
