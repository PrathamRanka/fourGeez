package dynamodb

import (
	"context"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
)

func TestCatalogRepositoryCreatesPublishedRouteWithDirectoryProjection(t *testing.T) {
	t.Parallel()
	seller, route := testDynamoDirectorySellerAndRoute(t)
	sellerRecord, err := newStoredRecord(sellerPartitionKey(seller.SellerID.String()), profileSortKey, "seller", seller)
	if err != nil {
		t.Fatal(err)
	}
	sellerItem, err := marshalStoredRecord(sellerRecord)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{getOutput: &awssdk.GetItemOutput{Item: sellerItem}}
	repository := NewCatalogRepository(client, "agentpay-dev")

	if err := repository.CreateRoute(context.Background(), route); err != nil {
		t.Fatal(err)
	}
	writes := client.transactWriteInput.TransactItems
	if len(writes) < 4 {
		t.Fatalf("transaction writes = %d, want route, slug claim, listing, and term projections", len(writes))
	}
	assertDirectoryPutExists(t, writes, directoryProductsPartitionKey())
	assertDirectoryPutExists(t, writes, directoryTermPartitionKey("research"))
}

func TestCatalogRepositoryRemovesDirectoryProjectionWhenRoutePauses(t *testing.T) {
	t.Parallel()
	_, route := testDynamoDirectorySellerAndRoute(t)
	storedRecord, err := newStoredRecord(sellerPartitionKey(route.SellerID.String()), routeSortKey(route.RouteID.String()), "paidRoute", route)
	if err != nil {
		t.Fatal(err)
	}
	storedRecord.Version = route.Version
	storedItem, err := marshalStoredRecord(storedRecord)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{queryOutput: &awssdk.QueryOutput{Items: []map[string]types.AttributeValue{storedItem}}}
	repository := NewCatalogRepository(client, "agentpay-dev")
	expectedVersion := route.Version
	if err := route.Pause(route.UpdatedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	if err := repository.UpdateRoute(context.Background(), route, expectedVersion); err != nil {
		t.Fatal(err)
	}
	writes := client.transactWriteInput.TransactItems
	assertDirectoryDeleteExists(t, writes, directoryProductsPartitionKey())
	assertDirectoryDeleteExists(t, writes, directoryTermPartitionKey("research"))
}

func TestCatalogRepositoryAddsDirectoryProjectionWhenDraftPublishes(t *testing.T) {
	t.Parallel()
	_, route := testDynamoDirectorySellerAndRoute(t)
	route.LifecycleStatus = catalog.RouteLifecycleDraft
	route.Enabled = false
	storedRecord, err := newStoredRecord(sellerPartitionKey(route.SellerID.String()), routeSortKey(route.RouteID.String()), "paidRoute", route)
	if err != nil {
		t.Fatal(err)
	}
	storedRecord.Version = route.Version
	storedItem, err := marshalStoredRecord(storedRecord)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{queryOutput: &awssdk.QueryOutput{Items: []map[string]types.AttributeValue{storedItem}}}
	repository := NewCatalogRepository(client, "agentpay-dev")
	expectedVersion := route.Version
	if err := route.Publish(route.UpdatedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	if err := repository.UpdateRoute(context.Background(), route, expectedVersion); err != nil {
		t.Fatal(err)
	}
	assertDirectoryPutExists(t, client.transactWriteInput.TransactItems, directoryProductsPartitionKey())
	assertDirectoryPutExists(t, client.transactWriteInput.TransactItems, directoryTermPartitionKey("research"))
}

func TestCatalogRepositoryQueriesDirectoryPartitionWithoutScan(t *testing.T) {
	t.Parallel()
	_, route := testDynamoDirectorySellerAndRoute(t)
	projection, ok := catalog.NewPublicDirectoryProjection(route)
	if !ok {
		t.Fatal("published route projection missing")
	}
	record, err := newStoredRecord(directoryTermPartitionKey("research"), directoryProductSortKey(projection), "publicDirectoryProduct", projection)
	if err != nil {
		t.Fatal(err)
	}
	item, err := marshalStoredRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{queryOutput: &awssdk.QueryOutput{Items: []map[string]types.AttributeValue{item}}}
	repository := NewCatalogRepository(client, "agentpay-dev")

	page, err := repository.ListPublicDirectoryCandidates(context.Background(), catalog.PublicDirectoryQuery{SearchTerm: "research", After: "PRODUCT#before", Limit: 12})
	if err != nil || len(page.Items) != 1 || page.Items[0].Projection.RouteID != route.RouteID {
		t.Fatalf("page = %#v, error = %v", page, err)
	}
	if client.queryInput.IndexName != nil || client.queryInput.KeyConditionExpression == nil || !strings.Contains(*client.queryInput.KeyConditionExpression, "PK = :partitionKey AND SK > :after") {
		t.Fatalf("query input = %#v", client.queryInput)
	}
	if client.queryInput.ScanIndexForward == nil || !*client.queryInput.ScanIndexForward {
		t.Fatalf("directory query must use ascending lexical order: %#v", client.queryInput)
	}
	if readStringAttribute(client.queryInput.ExpressionAttributeValues[":partitionKey"]) != directoryTermPartitionKey("research") {
		t.Fatalf("partition = %#v", client.queryInput.ExpressionAttributeValues)
	}
}

func TestCatalogRepositoryLoadsDirectoryRouteWithStrongSellerScopedRead(t *testing.T) {
	t.Parallel()
	_, route := testDynamoDirectorySellerAndRoute(t)
	record, err := newStoredRecord(sellerPartitionKey(route.SellerID.String()), routeSortKey(route.RouteID.String()), "paidRoute", route)
	if err != nil {
		t.Fatal(err)
	}
	item, err := marshalStoredRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{getOutput: &awssdk.GetItemOutput{Item: item}}
	repository := NewCatalogRepository(client, "agentpay-dev")

	loaded, err := repository.GetPublicDirectoryRoute(context.Background(), route.SellerID, route.RouteID)
	if err != nil || loaded.RouteID != route.RouteID {
		t.Fatalf("route = %#v, error = %v", loaded, err)
	}
	if client.getInput == nil || client.getInput.ConsistentRead == nil || !*client.getInput.ConsistentRead {
		t.Fatalf("get input = %#v", client.getInput)
	}
	if readStringAttribute(client.getInput.Key["PK"]) != sellerPartitionKey(route.SellerID.String()) ||
		readStringAttribute(client.getInput.Key["SK"]) != routeSortKey(route.RouteID.String()) {
		t.Fatalf("get key = %#v", client.getInput.Key)
	}
}

func testDynamoDirectorySellerAndRoute(t *testing.T) (catalog.Seller, catalog.PaidRoute) {
	t.Helper()
	now := domain.NewTimestamp(time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC))
	seller, err := catalog.NewSeller(catalog.SellerParams{SellerID: mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix), OwnerSubject: "owner", Slug: "northstar", Name: "Northstar", UpstreamBaseURL: "https://seller.example", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	route, err := catalog.NewPaidRoute(catalog.PaidRouteParams{RouteID: mustDynamoID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.RouteIDPrefix), SellerID: seller.SellerID, DisplayName: "Research Report", ProductSlug: "research-report", Method: catalog.RouteMethodPost, PathPattern: "/research", Description: "Source backed market brief", MIMEType: "application/json", Amount: domain.MustParseAmount("100000"), Asset: "USDC", Network: "eip155:84532", PayTo: "0x1111111111111111111111111111111111111111", UpstreamTimeoutSeconds: 10, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	return seller, route
}

func assertDirectoryPutExists(t *testing.T, writes []types.TransactWriteItem, partitionKey string) {
	t.Helper()
	for _, write := range writes {
		if write.Put != nil && readStringAttribute(write.Put.Item["PK"]) == partitionKey {
			return
		}
	}
	t.Fatalf("directory put for %q not found", partitionKey)
}

func assertDirectoryDeleteExists(t *testing.T, writes []types.TransactWriteItem, partitionKey string) {
	t.Helper()
	for _, write := range writes {
		if write.Delete != nil && readStringAttribute(write.Delete.Key["PK"]) == partitionKey {
			return
		}
	}
	t.Fatalf("directory delete for %q not found", partitionKey)
}
