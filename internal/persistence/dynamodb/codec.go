package dynamodb

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// storedRecord is the shared single-table DynamoDB envelope.
type storedRecord struct {
	PartitionKey string `dynamodbav:"PK"`
	SortKey      string `dynamodbav:"SK"`
	Entity       string `dynamodbav:"entity"`
	Payload      []byte `dynamodbav:"payload"`
	Version      uint64 `dynamodbav:"version,omitempty"`
	Status       string `dynamodbav:"status,omitempty"`
	EventHash    string `dynamodbav:"eventHash,omitempty"`
	GSI1PK       string `dynamodbav:"GSI1PK,omitempty"`
	GSI1SK       string `dynamodbav:"GSI1SK,omitempty"`
	GSI2PK       string `dynamodbav:"GSI2PK,omitempty"`
	GSI2SK       string `dynamodbav:"GSI2SK,omitempty"`
	GSI3PK       string `dynamodbav:"GSI3PK,omitempty"`
	GSI3SK       string `dynamodbav:"GSI3SK,omitempty"`
}

// newStoredRecord serializes a domain snapshot into the shared envelope.
func newStoredRecord(partitionKey, sortKey, entity string, payload any) (storedRecord, error) {
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return storedRecord{}, err
	}

	return storedRecord{
		PartitionKey: partitionKey,
		SortKey:      sortKey,
		Entity:       entity,
		Payload:      encodedPayload,
	}, nil
}

// marshalStoredRecord converts the shared envelope into DynamoDB attributes.
func marshalStoredRecord(record storedRecord) (map[string]types.AttributeValue, error) {
	return attributevalue.MarshalMap(record)
}

// unmarshalPayload decodes the domain snapshot from a DynamoDB item.
func unmarshalPayload(item map[string]types.AttributeValue, destination any) error {
	if len(item) == 0 {
		return persistence.ErrNotFound
	}

	var record storedRecord
	if err := attributevalue.UnmarshalMap(item, &record); err != nil {
		return err
	}

	return json.Unmarshal(record.Payload, destination)
}

// marshalPayload serializes a domain snapshot for an update expression.
func marshalPayload(value any) ([]byte, error) {
	record, err := newStoredRecord("", "", "", value)
	if err != nil {
		return nil, err
	}

	return record.Payload, nil
}

// primaryKey builds a DynamoDB composite primary key.
func primaryKey(partitionKey, sortKey string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: partitionKey},
		"SK": &types.AttributeValueMemberS{Value: sortKey},
	}
}

// stringAttributeValue creates a DynamoDB string attribute.
func stringAttributeValue(value string) types.AttributeValue {
	return &types.AttributeValueMemberS{Value: value}
}

// numberAttributeValue creates a DynamoDB unsigned-number attribute.
func numberAttributeValue(value uint64) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatUint(value, 10)}
}

// isConditionalFailure identifies a failed single-item condition.
func isConditionalFailure(err error) bool {
	var conditionalError *types.ConditionalCheckFailedException
	return errors.As(err, &conditionalError)
}

// isTransactionFailure identifies a cancelled transactional write.
func isTransactionFailure(err error) bool {
	var transactionError *types.TransactionCanceledException
	return errors.As(err, &transactionError)
}

// stringPointer returns a pointer used by AWS SDK request structures.
func stringPointer(value string) *string {
	return &value
}

// boolPointer returns a pointer used by AWS SDK request structures.
func boolPointer(value bool) *bool {
	return &value
}

// int32Pointer returns a pointer used by AWS SDK request structures.
func int32Pointer(value int32) *int32 {
	return &value
}
