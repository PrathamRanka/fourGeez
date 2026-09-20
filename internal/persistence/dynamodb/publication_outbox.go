package dynamodb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/publicationops"
)

func (repository repositoryBase) publicationOutboxPut(
	eventType publicationops.EventType,
	sellerID string,
	aggregateID string,
	aggregateVersion uint64,
	occurredAt domain.Timestamp,
) (types.TransactWriteItem, error) {
	eventID := publicationOutboxEventID(eventType, sellerID, aggregateID, aggregateVersion)
	event := publicationops.Event{
		SchemaVersion: publicationops.SchemaVersionV1, EventID: eventID, EventType: eventType,
		SellerID: sellerID, AggregateID: aggregateID, AggregateVersion: aggregateVersion, OccurredAt: occurredAt,
	}
	record, err := newStoredRecord(
		sellerPartitionKey(sellerID),
		publicationOutboxSortKey(eventType, aggregateID, aggregateVersion),
		"publicationOutbox",
		event,
	)
	if err != nil {
		return types.TransactWriteItem{}, err
	}
	item, err := marshalStoredRecord(record)
	if err != nil {
		return types.TransactWriteItem{}, err
	}
	return types.TransactWriteItem{Put: &types.Put{
		TableName:           &repository.tableName,
		Item:                item,
		ConditionExpression: stringPointer(createItemCondition),
	}}, nil
}

func publicationOutboxSortKey(eventType publicationops.EventType, aggregateID string, aggregateVersion uint64) string {
	return fmt.Sprintf("PUBLICATION_OUTBOX#%s#%s#%020d", eventType, aggregateID, aggregateVersion)
}

func publicationOutboxEventID(eventType publicationops.EventType, sellerID, aggregateID string, aggregateVersion uint64) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"agentpay.publication-outbox.v1\x00%s\x00%s\x00%s\x00%d",
		eventType,
		sellerID,
		aggregateID,
		aggregateVersion,
	)))
	return hex.EncodeToString(digest[:])
}
