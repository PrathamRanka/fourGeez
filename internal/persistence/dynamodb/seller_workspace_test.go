package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/sellerworkspace"
)

func TestDynamoSellerWorkspaceRepositoryUsesDocumentedKeyAndStrongRead(t *testing.T) {
	t.Parallel()
	state := testSellerWorkspaceState(t)
	client := &workspaceDynamoClient{}
	repository := NewSellerWorkspaceRepository(client, "agentpay-dev")
	if err := repository.Put(context.Background(), state, 0); err != nil {
		t.Fatal(err)
	}
	if client.putInput == nil || client.putInput.ConditionExpression == nil || *client.putInput.ConditionExpression != sellerWorkspaceCreateCondition {
		t.Fatalf("create condition = %#v", client.putInput)
	}
	client.getOutput = &awssdk.GetItemOutput{Item: client.putInput.Item}
	stored, err := repository.Get(context.Background(), state.SellerID)
	if err != nil || stored.SellerID != state.SellerID {
		t.Fatalf("Get() = %#v, %v", stored, err)
	}
	if client.getInput.ConsistentRead == nil || !*client.getInput.ConsistentRead {
		t.Fatal("workspace reads must be strongly consistent")
	}
}

func testSellerWorkspaceState(t *testing.T) sellerworkspace.WorkspaceState {
	t.Helper()
	sellerID, err := domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	if err != nil {
		t.Fatal(err)
	}
	now := domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC))
	return sellerworkspace.WorkspaceState{SellerID: sellerID, OwnerSubjectHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CreatedAt: now, UpdatedAt: now, Version: 1, Settings: sellerworkspace.SellerSettings{Version: 1, UpdatedAt: now}}
}

func TestDynamoSellerWorkspaceRepositoryMapsConditionalFailure(t *testing.T) {
	t.Parallel()
	client := &workspaceDynamoClient{putErr: &types.ConditionalCheckFailedException{Message: stringPointer("conflict")}}
	repository := NewSellerWorkspaceRepository(client, "agentpay-dev")
	if err := repository.Put(context.Background(), testSellerWorkspaceState(t), 1); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("Put() error = %v, want condition failure", err)
	}
}

type workspaceDynamoClient struct {
	putInput  *awssdk.PutItemInput
	putErr    error
	getInput  *awssdk.GetItemInput
	getOutput *awssdk.GetItemOutput
}

func (client *workspaceDynamoClient) PutItem(_ context.Context, input *awssdk.PutItemInput, _ ...func(*awssdk.Options)) (*awssdk.PutItemOutput, error) {
	client.putInput = input
	return &awssdk.PutItemOutput{}, client.putErr
}
func (client *workspaceDynamoClient) GetItem(_ context.Context, input *awssdk.GetItemInput, _ ...func(*awssdk.Options)) (*awssdk.GetItemOutput, error) {
	client.getInput = input
	if client.getOutput == nil {
		return &awssdk.GetItemOutput{}, nil
	}
	return client.getOutput, nil
}
func (*workspaceDynamoClient) UpdateItem(context.Context, *awssdk.UpdateItemInput, ...func(*awssdk.Options)) (*awssdk.UpdateItemOutput, error) {
	return &awssdk.UpdateItemOutput{}, nil
}
func (*workspaceDynamoClient) Query(context.Context, *awssdk.QueryInput, ...func(*awssdk.Options)) (*awssdk.QueryOutput, error) {
	return &awssdk.QueryOutput{}, nil
}
func (*workspaceDynamoClient) TransactWriteItems(context.Context, *awssdk.TransactWriteItemsInput, ...func(*awssdk.Options)) (*awssdk.TransactWriteItemsOutput, error) {
	return &awssdk.TransactWriteItemsOutput{}, nil
}
