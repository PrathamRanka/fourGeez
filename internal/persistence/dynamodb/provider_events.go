package dynamodb

import (
	"context"
	"strconv"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

type ProviderEventRepository struct{ repositoryBase }

func NewProviderEventRepository(client Client, tableName string) *ProviderEventRepository {
	return &ProviderEventRepository{repositoryBase: newRepositoryBase(client, tableName)}
}

func (repository *ProviderEventRepository) Store(
	ctx context.Context,
	event billing.SubscriptionProviderEvent,
) (billing.ProviderEventStoreResult, error) {
	item, err := marshalProviderEvent(event)
	if err != nil {
		return 0, err
	}
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName: &repository.tableName, Item: item,
		ConditionExpression: stringPointer(createItemCondition),
	})
	if err == nil {
		return billing.ProviderEventStoreResultInserted, nil
	}
	if !isConditionalFailure(err) {
		return 0, err
	}
	stored, err := repository.Get(ctx, event.EventID)
	if err != nil {
		return 0, err
	}
	if stored.PayloadHash == event.PayloadHash {
		return billing.ProviderEventStoreResultDuplicate, nil
	}
	stored.ProcessingState = billing.ProviderEventStateQuarantined
	stored.ConflictPayloadHash = event.PayloadHash
	if err := repository.replaceWithHash(ctx, stored, stored.PayloadHash); err != nil {
		return 0, err
	}
	return billing.ProviderEventStoreResultConflict, nil
}

func (repository *ProviderEventRepository) replaceWithHash(
	ctx context.Context,
	event billing.SubscriptionProviderEvent,
	expectedHash string,
) error {
	item, err := marshalProviderEvent(event)
	if err != nil {
		return err
	}
	condition := "eventHash = :expectedHash"
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName: &repository.tableName, Item: item, ConditionExpression: &condition,
		ExpressionAttributeValues: map[string]types.AttributeValue{":expectedHash": stringAttributeValue(expectedHash)},
	})
	if isConditionalFailure(err) {
		return billing.ErrProviderEventPayloadConflict
	}
	return err
}

func (repository *ProviderEventRepository) Get(
	ctx context.Context,
	eventID string,
) (billing.SubscriptionProviderEvent, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(providerEventPartitionKey(eventID), providerEventInboxSortKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return billing.SubscriptionProviderEvent{}, err
	}
	var event billing.SubscriptionProviderEvent
	if err := unmarshalPayload(output.Item, &event); err != nil {
		return billing.SubscriptionProviderEvent{}, err
	}
	if event.EventID != eventID {
		return billing.SubscriptionProviderEvent{}, billing.ErrProviderEventNotFound
	}
	return event, nil
}

func (repository *ProviderEventRepository) MarkProcessing(
	ctx context.Context,
	eventID string,
	_ domain.Timestamp,
) error {
	event, err := repository.Get(ctx, eventID)
	if err != nil {
		return err
	}
	if event.ProcessingState == billing.ProviderEventStateQuarantined {
		return billing.ErrProviderEventQuarantined
	}
	if event.ProcessingState != billing.ProviderEventStateReceived && event.ProcessingState != billing.ProviderEventStateFailed {
		return billing.ErrProviderEventAlreadyProcessing
	}
	event.ProcessingState = billing.ProviderEventStateProcessing
	event.AttemptCount++
	return repository.replaceWithStates(ctx, event, []billing.ProviderEventProcessingState{billing.ProviderEventStateReceived, billing.ProviderEventStateFailed}, event.PayloadHash)
}

func (repository *ProviderEventRepository) MarkFailed(ctx context.Context, eventID string, failedAt domain.Timestamp) error {
	event, err := repository.Get(ctx, eventID)
	if err != nil {
		return err
	}
	if event.ProcessingState == billing.ProviderEventStateQuarantined {
		return billing.ErrProviderEventQuarantined
	}
	expectedState := event.ProcessingState
	event.ProcessingState = billing.ProviderEventStateFailed
	event.ProcessedAt = &failedAt
	return repository.replaceWithStates(ctx, event, []billing.ProviderEventProcessingState{expectedState}, event.PayloadHash)
}

func (repository *ProviderEventRepository) MarkApplied(
	ctx context.Context,
	eventID string,
	sellerID domain.ID,
	entitlementVersion uint64,
	appliedAt domain.Timestamp,
) error {
	event, err := repository.Get(ctx, eventID)
	if err != nil {
		return err
	}
	if event.ProcessingState == billing.ProviderEventStateQuarantined {
		return billing.ErrProviderEventQuarantined
	}
	event.SellerID = sellerID
	event.ProcessingState = billing.ProviderEventStateApplied
	event.AppliedEntitlementVersion = entitlementVersion
	event.ProcessedAt = &appliedAt
	return repository.replaceWithStates(ctx, event, []billing.ProviderEventProcessingState{billing.ProviderEventStateProcessing}, event.PayloadHash)
}

func (repository *ProviderEventRepository) replaceWithStates(
	ctx context.Context,
	event billing.SubscriptionProviderEvent,
	expectedStates []billing.ProviderEventProcessingState,
	expectedHash string,
) error {
	item, err := marshalProviderEvent(event)
	if err != nil {
		return err
	}
	condition := "(#status = :expectedStatus0"
	attributeValues := map[string]types.AttributeValue{
		":expectedStatus0": stringAttributeValue(string(expectedStates[0])),
		":expectedHash":    stringAttributeValue(expectedHash),
	}
	for index := 1; index < len(expectedStates); index++ {
		name := ":expectedStatus" + strconv.Itoa(index)
		condition += " OR #status = " + name
		attributeValues[name] = stringAttributeValue(string(expectedStates[index]))
	}
	condition += ") AND eventHash = :expectedHash"
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName: &repository.tableName, Item: item, ConditionExpression: &condition,
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: attributeValues,
	})
	if isConditionalFailure(err) {
		return billing.ErrProviderEventAlreadyProcessing
	}
	return err
}

func marshalProviderEvent(event billing.SubscriptionProviderEvent) (map[string]types.AttributeValue, error) {
	record, err := newStoredRecord(providerEventPartitionKey(event.EventID), providerEventInboxSortKey, "subscriptionProviderEvent", event)
	if err != nil {
		return nil, err
	}
	record.Status = string(event.ProcessingState)
	record.EventHash = event.PayloadHash
	return marshalStoredRecord(record)
}
