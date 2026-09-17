package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Client contains the DynamoDB operations used by AgentPay repositories.
type Client interface {
	PutItem(context.Context, *awssdk.PutItemInput, ...func(*awssdk.Options)) (*awssdk.PutItemOutput, error)
	GetItem(context.Context, *awssdk.GetItemInput, ...func(*awssdk.Options)) (*awssdk.GetItemOutput, error)
	UpdateItem(context.Context, *awssdk.UpdateItemInput, ...func(*awssdk.Options)) (*awssdk.UpdateItemOutput, error)
	Query(context.Context, *awssdk.QueryInput, ...func(*awssdk.Options)) (*awssdk.QueryOutput, error)
	TransactWriteItems(context.Context, *awssdk.TransactWriteItemsInput, ...func(*awssdk.Options)) (*awssdk.TransactWriteItemsOutput, error)
}

// repositoryBase stores dependencies shared by DynamoDB repositories.
type repositoryBase struct {
	client    Client
	tableName string
}

// newRepositoryBase creates shared repository dependencies.
func newRepositoryBase(client Client, tableName string) repositoryBase {
	return repositoryBase{
		client:    client,
		tableName: tableName,
	}
}
