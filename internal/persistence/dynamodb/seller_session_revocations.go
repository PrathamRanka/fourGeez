package dynamodb

import (
	"context"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const extendSessionRevocationCondition = "attribute_not_exists(PK) OR expiresAt < :expiresAt"

type sellerSessionRevocation struct {
	SessionDigest string    `json:"sessionDigest"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

// SellerSessionRevocationRepository persists hashed session-family revocations.
type SellerSessionRevocationRepository struct {
	repositoryBase
}

// NewSellerSessionRevocationRepository creates a DynamoDB revocation repository.
func NewSellerSessionRevocationRepository(client Client, tableName string) *SellerSessionRevocationRepository {
	return &SellerSessionRevocationRepository{repositoryBase: newRepositoryBase(client, tableName)}
}

// Revoke stores a revocation until the latest known token-family expiry.
func (repository *SellerSessionRevocationRepository) Revoke(ctx context.Context, digest string, expiresAt time.Time) error {
	record, err := newStoredRecord(
		sellerSessionPartitionKey(digest), sellerSessionRevocationSortKey, "sellerSessionRevocation",
		sellerSessionRevocation{SessionDigest: digest, ExpiresAt: expiresAt.UTC()},
	)
	if err != nil {
		return err
	}
	record.ExpiresAt = expiresAt.UTC().Unix()
	item, err := marshalStoredRecord(record)
	if err != nil {
		return err
	}
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                item,
		ConditionExpression: stringPointer(extendSessionRevocationCondition),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expiresAt": numberAttributeValue(uint64(expiresAt.UTC().Unix())),
		},
	})
	if isConditionalFailure(err) {
		return nil
	}
	return err
}

// IsRevoked checks a strongly consistent revocation record at the request time.
func (repository *SellerSessionRevocationRepository) IsRevoked(ctx context.Context, digest string, now time.Time) (bool, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(sellerSessionPartitionKey(digest), sellerSessionRevocationSortKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return false, err
	}
	if len(output.Item) == 0 {
		return false, nil
	}
	var revocation sellerSessionRevocation
	if err := unmarshalPayload(output.Item, &revocation); err != nil {
		return false, err
	}
	return revocation.ExpiresAt.After(now), nil
}
