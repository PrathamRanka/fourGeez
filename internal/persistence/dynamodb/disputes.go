package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// DisputeRepository persists deterministic dispute classifications in DynamoDB.
type DisputeRepository struct {
	repositoryBase
}

// NewDisputeRepository creates a DynamoDB-backed dispute repository.
func NewDisputeRepository(client Client, tableName string) *DisputeRepository {
	return &DisputeRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Create inserts a dispute when its identifier is unused.
func (repository *DisputeRepository) Create(
	ctx context.Context,
	dispute disputes.Dispute,
) error {
	disputeRecord, err := newStoredRecord(
		disputePartitionKey(dispute.DisputeID.String()),
		profileSortKey,
		"dispute",
		dispute,
	)
	if err != nil {
		return err
	}

	disputeItem, err := marshalStoredRecord(disputeRecord)
	if err != nil {
		return err
	}

	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                disputeItem,
		ConditionExpression: stringPointer(createItemCondition),
	})
	if isConditionalFailure(err) {
		return persistence.ErrAlreadyExists
	}

	return err
}

// Get loads a dispute by identifier.
func (repository *DisputeRepository) Get(
	ctx context.Context,
	disputeID domain.ID,
) (disputes.Dispute, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			disputePartitionKey(disputeID.String()),
			profileSortKey,
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return disputes.Dispute{}, err
	}

	var dispute disputes.Dispute
	if err := unmarshalPayload(output.Item, &dispute); err != nil {
		return disputes.Dispute{}, err
	}

	return dispute, nil
}
