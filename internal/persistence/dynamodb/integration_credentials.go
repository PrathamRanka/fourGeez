package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// IntegrationCredentialRepository persists seller-scoped credentials.
type IntegrationCredentialRepository struct {
	repositoryBase
}

type credentialLookup struct {
	CredentialID domain.ID `json:"credentialId"`
	SellerID     domain.ID `json:"sellerId"`
}

// NewIntegrationCredentialRepository creates a DynamoDB credential repository.
func NewIntegrationCredentialRepository(
	client Client,
	tableName string,
) *IntegrationCredentialRepository {
	return &IntegrationCredentialRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Create stores one credential when its seller-scoped key is unused.
func (repository *IntegrationCredentialRepository) Create(
	ctx context.Context,
	credential integrations.Credential,
) error {
	credentialRecord, err := newStoredRecord(
		sellerPartitionKey(credential.SellerID().String()),
		credentialSortKey(credential.CredentialID().String()),
		"integrationCredential",
		credential.Snapshot(),
	)
	if err != nil {
		return err
	}
	credentialRecord.Version = credential.Version()
	credentialItem, err := marshalStoredRecord(credentialRecord)
	if err != nil {
		return err
	}

	lookupItem, err := repository.marshalCredentialLookup(credential)
	if err != nil {
		return err
	}
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{Put: &types.Put{TableName: &repository.tableName, Item: lookupItem, ConditionExpression: stringPointer(createItemCondition)}},
			{Put: &types.Put{TableName: &repository.tableName, Item: credentialItem, ConditionExpression: stringPointer(createItemCondition)}},
		},
	})
	if isTransactionFailure(err) {
		return persistence.ErrAlreadyExists
	}
	return err
}

// Get loads one credential by its seller-scoped primary key.
func (repository *IntegrationCredentialRepository) Get(
	ctx context.Context,
	sellerID domain.ID,
	credentialID domain.ID,
) (integrations.Credential, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			sellerPartitionKey(sellerID.String()),
			credentialSortKey(credentialID.String()),
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return integrations.Credential{}, err
	}

	var snapshot integrations.Snapshot
	if err := unmarshalPayload(output.Item, &snapshot); err != nil {
		return integrations.Credential{}, err
	}
	if snapshot.SellerID != sellerID || snapshot.CredentialID != credentialID {
		return integrations.Credential{}, persistence.ErrNotFound
	}
	return integrations.RestoreCredential(snapshot), nil
}

// GetByID resolves the public credential lookup before loading seller state.
func (repository *IntegrationCredentialRepository) GetByID(
	ctx context.Context,
	credentialID domain.ID,
) (integrations.Credential, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(credentialPartitionKey(credentialID.String()), credentialLookupSortKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return integrations.Credential{}, err
	}
	var lookup credentialLookup
	if err := unmarshalPayload(output.Item, &lookup); err != nil {
		return integrations.Credential{}, err
	}
	if lookup.CredentialID != credentialID || lookup.SellerID.Prefix() != domain.SellerIDPrefix {
		return integrations.Credential{}, persistence.ErrNotFound
	}
	return repository.Get(ctx, lookup.SellerID, credentialID)
}

// ListBySeller queries all credentials without scanning the table.
func (repository *IntegrationCredentialRepository) ListBySeller(
	ctx context.Context,
	sellerID domain.ID,
) ([]integrations.Credential, error) {
	keyCondition := "PK = :partitionKey AND begins_with(SK, :sortKeyPrefix)"
	output, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName:              &repository.tableName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":partitionKey": stringAttributeValue(
				sellerPartitionKey(sellerID.String()),
			),
			":sortKeyPrefix": stringAttributeValue("CREDENTIAL#"),
		},
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return nil, err
	}

	credentials := make([]integrations.Credential, 0, len(output.Items))
	for _, credentialItem := range output.Items {
		var snapshot integrations.Snapshot
		if err := unmarshalPayload(credentialItem, &snapshot); err != nil {
			return nil, err
		}
		credentials = append(
			credentials,
			integrations.RestoreCredential(snapshot),
		)
	}
	return credentials, nil
}

// Update replaces one credential only when its stored version matches.
func (repository *IntegrationCredentialRepository) Update(
	ctx context.Context,
	credential integrations.Credential,
	expectedVersion uint64,
) error {
	credentialRecord, err := newStoredRecord(
		sellerPartitionKey(credential.SellerID().String()),
		credentialSortKey(credential.CredentialID().String()),
		"integrationCredential",
		credential.Snapshot(),
	)
	if err != nil {
		return err
	}
	credentialRecord.Version = credential.Version()
	credentialItem, err := marshalStoredRecord(credentialRecord)
	if err != nil {
		return err
	}

	condition := "#version = :expectedVersion"
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                credentialItem,
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

// Rotate atomically revokes a predecessor and creates its lookup and successor.
func (repository *IntegrationCredentialRepository) Rotate(
	ctx context.Context,
	predecessor integrations.Credential,
	successor integrations.Credential,
	expectedVersion uint64,
	replay integrations.RotationReplay,
) error {
	predecessorItem, err := repository.marshalCredential(predecessor)
	if err != nil {
		return err
	}
	successorItem, err := repository.marshalCredential(successor)
	if err != nil {
		return err
	}
	lookupItem, err := repository.marshalCredentialLookup(successor)
	if err != nil {
		return err
	}
	replayRecord, err := newStoredRecord(
		idempotencyPartitionKey(replay.Scope),
		string(replay.Key),
		"credentialRotationReplay",
		replay,
	)
	if err != nil {
		return err
	}
	replayItem, err := marshalStoredRecord(replayRecord)
	if err != nil {
		return err
	}
	versionCondition := "#version = :expectedVersion"
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{Put: &types.Put{
				TableName: &repository.tableName, Item: predecessorItem, ConditionExpression: &versionCondition,
				ExpressionAttributeNames:  map[string]string{"#version": "version"},
				ExpressionAttributeValues: map[string]types.AttributeValue{":expectedVersion": numberAttributeValue(expectedVersion)},
			}},
			{Put: &types.Put{TableName: &repository.tableName, Item: lookupItem, ConditionExpression: stringPointer(createItemCondition)}},
			{Put: &types.Put{TableName: &repository.tableName, Item: successorItem, ConditionExpression: stringPointer(createItemCondition)}},
			{Put: &types.Put{TableName: &repository.tableName, Item: replayItem, ConditionExpression: stringPointer(createItemCondition)}},
		},
	})
	if isTransactionFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

func (repository *IntegrationCredentialRepository) LoadRotationReplay(
	ctx context.Context,
	scope string,
	key domain.IdempotencyKey,
) (integrations.RotationReplay, bool, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			idempotencyPartitionKey(scope),
			string(key),
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return integrations.RotationReplay{}, false, err
	}
	if len(output.Item) == 0 {
		return integrations.RotationReplay{}, false, nil
	}
	var replay integrations.RotationReplay
	if err := unmarshalPayload(output.Item, &replay); err != nil {
		return integrations.RotationReplay{}, false, err
	}
	return replay, true, nil
}

func (repository *IntegrationCredentialRepository) marshalCredential(credential integrations.Credential) (map[string]types.AttributeValue, error) {
	record, err := newStoredRecord(
		sellerPartitionKey(credential.SellerID().String()),
		credentialSortKey(credential.CredentialID().String()),
		"integrationCredential",
		credential.Snapshot(),
	)
	if err != nil {
		return nil, err
	}
	record.Version = credential.Version()
	return marshalStoredRecord(record)
}

func (repository *IntegrationCredentialRepository) marshalCredentialLookup(credential integrations.Credential) (map[string]types.AttributeValue, error) {
	record, err := newStoredRecord(
		credentialPartitionKey(credential.CredentialID().String()),
		credentialLookupSortKey,
		"integrationCredentialLookup",
		credentialLookup{CredentialID: credential.CredentialID(), SellerID: credential.SellerID()},
	)
	if err != nil {
		return nil, err
	}
	return marshalStoredRecord(record)
}
