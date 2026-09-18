package dynamodb

import (
	"context"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
)

type ProjectKeyExchangeRateLimiter struct {
	repositoryBase
	clock   domain.Clock
	maximum uint64
	window  time.Duration
}

func NewProjectKeyExchangeRateLimiter(client Client, tableName string, clock domain.Clock, maximum uint64, window time.Duration) *ProjectKeyExchangeRateLimiter {
	return &ProjectKeyExchangeRateLimiter{repositoryBase: newRepositoryBase(client, tableName), clock: clock, maximum: maximum, window: window}
}

func (limiter *ProjectKeyExchangeRateLimiter) AllowProjectKeyExchange(
	ctx context.Context,
	credentialID domain.ID,
) error {
	now := limiter.clock.Now().UTC()
	windowStart := now.Truncate(limiter.window)
	condition := "attribute_not_exists(#count) OR #count < :limit"
	update := "ADD #count :one SET expiresAt = :expiresAt"
	_, err := limiter.client.UpdateItem(ctx, &awssdk.UpdateItemInput{
		TableName: &limiter.tableName,
		Key: primaryKey(
			credentialPartitionKey(credentialID.String()),
			projectKeyExchangeRateLimitSortKey(windowStart.Format(time.RFC3339)),
		),
		ConditionExpression:      &condition,
		UpdateExpression:         &update,
		ExpressionAttributeNames: map[string]string{"#count": "count"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one":       numberAttributeValue(1),
			":limit":     numberAttributeValue(limiter.maximum),
			":expiresAt": numberAttributeValue(uint64(windowStart.Add(limiter.window).Unix())),
		},
	})
	if isConditionalFailure(err) {
		return domain.ErrRateLimitExceeded
	}
	return err
}
