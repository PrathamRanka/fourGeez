package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const browserPurchaseCreationScopePrefix = "browser-purchase:"

type BrowserPurchaseSessionRepository struct{ repositoryBase }

type browserPurchaseCreationRecord struct {
	RequestHash       string                            `json:"requestHash"`
	PurchaseSessionID browserpurchase.PurchaseSessionID `json:"purchaseSessionId"`
}

type browserPurchaseGrantRecord struct {
	PurchaseSessionID browserpurchase.PurchaseSessionID `json:"purchaseSessionId"`
}

func NewBrowserPurchaseSessionRepository(client Client, tableName string) *BrowserPurchaseSessionRepository {
	return &BrowserPurchaseSessionRepository{repositoryBase: newRepositoryBase(client, tableName)}
}

func (repository *BrowserPurchaseSessionRepository) FindCreation(ctx context.Context, scope string, key domain.IdempotencyKey, requestHash string) (browserpurchase.BrowserPurchaseSession, bool, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(idempotencyPartitionKey(browserPurchaseCreationScopePrefix+scope), string(key)),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return browserpurchase.BrowserPurchaseSession{}, false, err
	}
	if len(output.Item) == 0 {
		return browserpurchase.BrowserPurchaseSession{}, false, nil
	}
	var creation browserPurchaseCreationRecord
	if err := unmarshalPayload(output.Item, &creation); err != nil {
		return browserpurchase.BrowserPurchaseSession{}, false, err
	}
	if creation.RequestHash != requestHash {
		return browserpurchase.BrowserPurchaseSession{}, false, browserpurchase.ErrIdempotencyConflict
	}
	session, err := repository.Get(ctx, creation.PurchaseSessionID)
	return session, err == nil, err
}

func (repository *BrowserPurchaseSessionRepository) Create(ctx context.Context, session browserpurchase.BrowserPurchaseSession, scope string, key domain.IdempotencyKey, requestHash string) (browserpurchase.BrowserPurchaseSession, bool, error) {
	creationRecord, err := newStoredRecord(
		idempotencyPartitionKey(browserPurchaseCreationScopePrefix+scope), string(key),
		"browserPurchaseCreation", browserPurchaseCreationRecord{RequestHash: requestHash, PurchaseSessionID: session.PurchaseSessionID},
	)
	if err != nil {
		return browserpurchase.BrowserPurchaseSession{}, false, err
	}
	creationRecord.ExpiresAt = session.AccessExpiresAt.Time().Unix()
	grantRecord, err := newStoredRecord(
		browserGrantPartitionKey(session.BrowserGrantHash), browserGrantPurchaseSortKey(session.PurchaseSessionID.String()),
		"browserPurchaseGrant", browserPurchaseGrantRecord{PurchaseSessionID: session.PurchaseSessionID},
	)
	if err != nil {
		return browserpurchase.BrowserPurchaseSession{}, false, err
	}
	grantRecord.ExpiresAt = session.AccessExpiresAt.Time().Unix()
	sessionRecord, err := browserPurchaseSessionRecord(session)
	if err != nil {
		return browserpurchase.BrowserPurchaseSession{}, false, err
	}
	creationItem, err := marshalStoredRecord(creationRecord)
	if err != nil {
		return browserpurchase.BrowserPurchaseSession{}, false, err
	}
	grantItem, err := marshalStoredRecord(grantRecord)
	if err != nil {
		return browserpurchase.BrowserPurchaseSession{}, false, err
	}
	sessionItem, err := marshalStoredRecord(sessionRecord)
	if err != nil {
		return browserpurchase.BrowserPurchaseSession{}, false, err
	}
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{
		{Put: &types.Put{TableName: &repository.tableName, Item: creationItem, ConditionExpression: stringPointer(createItemCondition)}},
		{Put: &types.Put{TableName: &repository.tableName, Item: grantItem, ConditionExpression: stringPointer(createItemCondition)}},
		{Put: &types.Put{TableName: &repository.tableName, Item: sessionItem, ConditionExpression: stringPointer(createItemCondition)}},
	}})
	if err == nil {
		return session, false, nil
	}
	if !isTransactionFailure(err) {
		return browserpurchase.BrowserPurchaseSession{}, false, err
	}
	existing, found, findErr := repository.FindCreation(ctx, scope, key, requestHash)
	if findErr != nil {
		return browserpurchase.BrowserPurchaseSession{}, false, findErr
	}
	if found {
		return existing, true, nil
	}
	return browserpurchase.BrowserPurchaseSession{}, false, persistence.ErrAlreadyExists
}

func (repository *BrowserPurchaseSessionRepository) Get(ctx context.Context, purchaseSessionID browserpurchase.PurchaseSessionID) (browserpurchase.BrowserPurchaseSession, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(purchaseSessionPartitionKey(purchaseSessionID.String()), profileSortKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return browserpurchase.BrowserPurchaseSession{}, err
	}
	var session browserpurchase.BrowserPurchaseSession
	if err := unmarshalPayload(output.Item, &session); err != nil {
		return browserpurchase.BrowserPurchaseSession{}, err
	}
	return session, nil
}

func (repository *BrowserPurchaseSessionRepository) GetByGrantHash(ctx context.Context, grantHash string) (browserpurchase.BrowserPurchaseSession, error) {
	partitionKey := browserGrantPartitionKey(grantHash)
	prefix := "PURCHASE#"
	output, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName:              &repository.tableName,
		KeyConditionExpression: stringPointer("PK = :pk AND begins_with(SK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": stringAttributeValue(partitionKey), ":prefix": stringAttributeValue(prefix),
		},
		ConsistentRead: boolPointer(true), Limit: int32Pointer(1),
	})
	if err != nil {
		return browserpurchase.BrowserPurchaseSession{}, err
	}
	if len(output.Items) != 1 {
		return browserpurchase.BrowserPurchaseSession{}, persistence.ErrNotFound
	}
	var claim browserPurchaseGrantRecord
	if err := unmarshalPayload(output.Items[0], &claim); err != nil {
		return browserpurchase.BrowserPurchaseSession{}, err
	}
	return repository.Get(ctx, claim.PurchaseSessionID)
}

func (repository *BrowserPurchaseSessionRepository) ClaimTransaction(ctx context.Context, purchaseSessionID browserpurchase.PurchaseSessionID, transactionID domain.ID, updatedAt domain.Timestamp) (browserpurchase.BrowserPurchaseSession, error) {
	session, err := repository.Get(ctx, purchaseSessionID)
	if err != nil {
		return browserpurchase.BrowserPurchaseSession{}, err
	}
	if session.TransactionID != nil {
		if *session.TransactionID == transactionID {
			return session, nil
		}
		return browserpurchase.BrowserPurchaseSession{}, persistence.ErrConditionFailed
	}
	if session.Status != browserpurchase.StatusActive {
		return browserpurchase.BrowserPurchaseSession{}, persistence.ErrConditionFailed
	}
	session.TransactionID = &transactionID
	session.UpdatedAt = updatedAt
	if err := repository.putSession(ctx, session, "#status = :active AND attribute_not_exists(transactionId)", map[string]string{"#status": "status"}, map[string]types.AttributeValue{":active": stringAttributeValue(string(browserpurchase.StatusActive))}); err != nil {
		return browserpurchase.BrowserPurchaseSession{}, err
	}
	return session, nil
}

func (repository *BrowserPurchaseSessionRepository) Complete(ctx context.Context, session browserpurchase.BrowserPurchaseSession) error {
	if session.Status != browserpurchase.StatusCompleted || session.TransactionID == nil || session.WalletBindingHash == nil {
		return persistence.ErrConditionFailed
	}
	return repository.putSession(ctx, session, "#status = :active AND transactionId = :transactionId", map[string]string{"#status": "status"}, map[string]types.AttributeValue{
		":active": stringAttributeValue(string(browserpurchase.StatusActive)), ":transactionId": stringAttributeValue(session.TransactionID.String()),
	})
}

func (repository *BrowserPurchaseSessionRepository) CreateChallenge(ctx context.Context, challenge browserpurchase.BrowserPurchaseRecoveryChallengeRecord) error {
	record, err := newStoredRecord(
		purchaseSessionPartitionKey(challenge.PurchaseSessionID.String()), browserRecoverySortKey(challenge.ChallengeID.String()),
		"browserPurchaseRecoveryChallenge", challenge,
	)
	if err != nil {
		return err
	}
	record.ExpiresAt = challenge.ExpiresAt.Time().Unix()
	item, err := marshalStoredRecord(record)
	if err != nil {
		return err
	}
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{TableName: &repository.tableName, Item: item, ConditionExpression: stringPointer(createItemCondition)})
	if isConditionalFailure(err) {
		return persistence.ErrAlreadyExists
	}
	return err
}

func (repository *BrowserPurchaseSessionRepository) GetChallenge(ctx context.Context, purchaseSessionID browserpurchase.PurchaseSessionID, challengeID browserpurchase.RecoveryChallengeID) (browserpurchase.BrowserPurchaseRecoveryChallengeRecord, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(purchaseSessionPartitionKey(purchaseSessionID.String()), browserRecoverySortKey(challengeID.String())),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return browserpurchase.BrowserPurchaseRecoveryChallengeRecord{}, err
	}
	var challenge browserpurchase.BrowserPurchaseRecoveryChallengeRecord
	if err := unmarshalPayload(output.Item, &challenge); err != nil {
		return browserpurchase.BrowserPurchaseRecoveryChallengeRecord{}, err
	}
	return challenge, nil
}

func (repository *BrowserPurchaseSessionRepository) Recover(ctx context.Context, session browserpurchase.BrowserPurchaseSession, previousGrantHash string, challenge browserpurchase.BrowserPurchaseRecoveryChallengeRecord) error {
	if challenge.UsedAt == nil || session.Status != browserpurchase.StatusCompleted {
		return persistence.ErrConditionFailed
	}
	sessionRecord, err := browserPurchaseSessionRecord(session)
	if err != nil {
		return err
	}
	sessionItem, err := marshalStoredRecord(sessionRecord)
	if err != nil {
		return err
	}
	challengeRecord, err := newStoredRecord(
		purchaseSessionPartitionKey(session.PurchaseSessionID.String()), browserRecoverySortKey(challenge.ChallengeID.String()),
		"browserPurchaseRecoveryChallenge", challenge,
	)
	if err != nil {
		return err
	}
	challengeRecord.ExpiresAt = challenge.ExpiresAt.Time().Unix()
	challengeRecord.UsedAt = challenge.UsedAt.String()
	challengeItem, err := marshalStoredRecord(challengeRecord)
	if err != nil {
		return err
	}
	grantRecord, err := newStoredRecord(
		browserGrantPartitionKey(session.BrowserGrantHash), browserGrantPurchaseSortKey(session.PurchaseSessionID.String()),
		"browserPurchaseGrant", browserPurchaseGrantRecord{PurchaseSessionID: session.PurchaseSessionID},
	)
	if err != nil {
		return err
	}
	grantRecord.ExpiresAt = session.AccessExpiresAt.Time().Unix()
	grantItem, err := marshalStoredRecord(grantRecord)
	if err != nil {
		return err
	}
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{
		{Delete: &types.Delete{TableName: &repository.tableName, Key: primaryKey(browserGrantPartitionKey(previousGrantHash), browserGrantPurchaseSortKey(session.PurchaseSessionID.String())), ConditionExpression: stringPointer("attribute_exists(PK) AND attribute_exists(SK)")}},
		{Put: &types.Put{TableName: &repository.tableName, Item: grantItem, ConditionExpression: stringPointer(createItemCondition)}},
		{Put: &types.Put{TableName: &repository.tableName, Item: sessionItem, ConditionExpression: stringPointer("#status = :completed AND browserGrantHash = :previousGrantHash"), ExpressionAttributeNames: map[string]string{"#status": "status"}, ExpressionAttributeValues: map[string]types.AttributeValue{":completed": stringAttributeValue(string(browserpurchase.StatusCompleted)), ":previousGrantHash": stringAttributeValue(previousGrantHash)}}},
		{Put: &types.Put{TableName: &repository.tableName, Item: challengeItem, ConditionExpression: stringPointer("attribute_not_exists(usedAt)")}},
	}})
	if isTransactionFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

func browserPurchaseSessionRecord(session browserpurchase.BrowserPurchaseSession) (storedRecord, error) {
	record, err := newStoredRecord(purchaseSessionPartitionKey(session.PurchaseSessionID.String()), profileSortKey, "browserPurchaseSession", session)
	if err != nil {
		return storedRecord{}, err
	}
	record.Status = string(session.Status)
	record.BrowserGrantHash = session.BrowserGrantHash
	if session.TransactionID != nil {
		record.TransactionID = session.TransactionID.String()
	}
	record.ExpiresAt = session.AccessExpiresAt.Time().Unix()
	return record, nil
}

func (repository *BrowserPurchaseSessionRepository) putSession(ctx context.Context, session browserpurchase.BrowserPurchaseSession, condition string, names map[string]string, values map[string]types.AttributeValue) error {
	record, err := browserPurchaseSessionRecord(session)
	if err != nil {
		return err
	}
	item, err := marshalStoredRecord(record)
	if err != nil {
		return err
	}
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName: &repository.tableName, Item: item, ConditionExpression: stringPointer(condition),
		ExpressionAttributeNames: names, ExpressionAttributeValues: values,
	})
	if isConditionalFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

var _ browserpurchase.Repository = (*BrowserPurchaseSessionRepository)(nil)
