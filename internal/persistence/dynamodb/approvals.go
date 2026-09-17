package dynamodb

import (
	"context"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// ApprovalRepository persists approval sessions in DynamoDB.
type ApprovalRepository struct {
	repositoryBase
}

// NewApprovalRepository creates a DynamoDB-backed approval repository.
func NewApprovalRepository(client Client, tableName string) *ApprovalRepository {
	return &ApprovalRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Create inserts an approval session when its identifier is unused.
func (repository *ApprovalRepository) Create(
	ctx context.Context,
	session approvals.Session,
) error {
	return repository.writeSession(ctx, session, nil)
}

// Get loads and validates an approval-session snapshot.
func (repository *ApprovalRepository) Get(
	ctx context.Context,
	sessionID domain.ID,
) (approvals.Session, error) {
	profileOutput, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			approvalPartitionKey(sessionID.String()),
			profileSortKey,
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return approvals.Session{}, err
	}

	var snapshot approvals.Snapshot
	if err := unmarshalPayload(profileOutput.Item, &snapshot); err != nil {
		return approvals.Session{}, err
	}

	keyCondition := "PK = :partitionKey AND begins_with(SK, :sortKeyPrefix)"
	invitationOutput, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName:              &repository.tableName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":partitionKey":  stringAttributeValue(approvalPartitionKey(sessionID.String())),
			":sortKeyPrefix": stringAttributeValue("INVITE#"),
		},
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return approvals.Session{}, err
	}

	snapshot.Invitations = make([]approvals.Invitation, 0, len(invitationOutput.Items))
	for _, item := range invitationOutput.Items {
		var invitation approvals.Invitation
		if err := unmarshalPayload(item, &invitation); err != nil {
			return approvals.Session{}, err
		}
		snapshot.Invitations = append(snapshot.Invitations, invitation)
	}

	return approvals.Restore(snapshot)
}

// Update replaces an approval session when its stored version matches.
func (repository *ApprovalRepository) Update(
	ctx context.Context,
	session approvals.Session,
	expectedVersion uint64,
) error {
	return repository.writeSession(ctx, session, &expectedVersion)
}

// writeSession stores the profile and invitation child records atomically.
func (repository *ApprovalRepository) writeSession(
	ctx context.Context,
	session approvals.Session,
	expectedVersion *uint64,
) error {
	snapshot := session.Snapshot()
	invitations := snapshot.Invitations
	snapshot.Invitations = nil

	sessionRecord, err := newStoredRecord(
		approvalPartitionKey(session.SessionID().String()),
		profileSortKey,
		"approvalSession",
		snapshot,
	)
	if err != nil {
		return err
	}

	sessionRecord.Version = session.Version()
	sessionItem, err := marshalStoredRecord(sessionRecord)
	if err != nil {
		return err
	}

	profilePut := &types.Put{
		TableName: &repository.tableName,
		Item:      sessionItem,
	}
	if expectedVersion == nil {
		profilePut.ConditionExpression = stringPointer(createItemCondition)
	} else {
		profilePut.ConditionExpression = stringPointer("#version = :expectedVersion")
		profilePut.ExpressionAttributeNames = map[string]string{
			"#version": "version",
		}
		profilePut.ExpressionAttributeValues = map[string]types.AttributeValue{
			":expectedVersion": numberAttributeValue(*expectedVersion),
		}
	}

	transactItems := []types.TransactWriteItem{{Put: profilePut}}
	for _, invitation := range invitations {
		invitationItem, err := repository.marshalInvitation(session.SessionID(), invitation)
		if err != nil {
			return err
		}

		invitationPut := &types.Put{
			TableName: &repository.tableName,
			Item:      invitationItem,
		}
		if expectedVersion == nil {
			invitationPut.ConditionExpression = stringPointer(createItemCondition)
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: invitationPut})
	}

	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: transactItems,
	})
	if isTransactionFailure(err) {
		if expectedVersion == nil {
			return persistence.ErrAlreadyExists
		}
		return persistence.ErrConditionFailed
	}

	return err
}

// marshalInvitation builds one approval invitation child item.
func (repository *ApprovalRepository) marshalInvitation(
	sessionID domain.ID,
	invitation approvals.Invitation,
) (map[string]types.AttributeValue, error) {
	invitationRecord, err := newStoredRecord(
		approvalPartitionKey(sessionID.String()),
		"INVITE#"+strings.ToLower(invitation.TokenHash),
		"approvalInvitation",
		invitation,
	)
	if err != nil {
		return nil, err
	}

	return marshalStoredRecord(invitationRecord)
}
