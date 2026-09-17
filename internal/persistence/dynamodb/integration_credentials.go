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

	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                credentialItem,
		ConditionExpression: stringPointer(createItemCondition),
	})
	if isConditionalFailure(err) {
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
