package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/fourgeez/agentpay/internal/disputes"
)

const manualRefundRecordSortKey = "REFUND_RECORD"

type ManualRefundRecordRepository struct{ repositoryBase }

func NewManualRefundRecordRepository(client Client, tableName string) *ManualRefundRecordRepository {
	return &ManualRefundRecordRepository{repositoryBase: newRepositoryBase(client, tableName)}
}

func (repository *ManualRefundRecordRepository) SaveIfAbsent(ctx context.Context, record disputes.ManualRefundRecord) (disputes.ManualRefundRecord, bool, error) {
	storedRecord, err := newStoredRecord(disputePartitionKey(record.DisputeID.String()), manualRefundRecordSortKey, "manualRefundRecord", record)
	if err != nil {
		return disputes.ManualRefundRecord{}, false, err
	}
	item, err := marshalStoredRecord(storedRecord)
	if err != nil {
		return disputes.ManualRefundRecord{}, false, err
	}
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{TableName: &repository.tableName, Item: item, ConditionExpression: stringPointer(createItemCondition)})
	if err == nil {
		return record, true, nil
	}
	if !isConditionalFailure(err) {
		return disputes.ManualRefundRecord{}, false, err
	}
	output, getErr := repository.client.GetItem(ctx, &awssdk.GetItemInput{TableName: &repository.tableName, Key: primaryKey(disputePartitionKey(record.DisputeID.String()), manualRefundRecordSortKey), ConsistentRead: boolPointer(true)})
	if getErr != nil {
		return disputes.ManualRefundRecord{}, false, getErr
	}
	var existing disputes.ManualRefundRecord
	if getErr := unmarshalPayload(output.Item, &existing); getErr != nil {
		return disputes.ManualRefundRecord{}, false, getErr
	}
	if existing != record {
		return disputes.ManualRefundRecord{}, false, disputes.ErrRemediationConflict
	}
	return existing, false, nil
}

var _ disputes.ManualRefundRecordRepository = (*ManualRefundRecordRepository)(nil)
