package dynamodb

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
)

const publicDirectoryProductsPartition = "DISCOVERY#PRODUCTS"

func directoryProductsPartitionKey() string { return publicDirectoryProductsPartition }

func directoryTermPartitionKey(term string) string { return "DISCOVERY#TERM#" + term }

func directoryProductSortKey(projection catalog.PublicDirectoryProjection) string {
	return projection.SortKey()
}

type directoryProjectionKey struct {
	partitionKey string
	sortKey      string
}

func directoryProjectionKeys(route catalog.PaidRoute) []directoryProjectionKey {
	projection, published := catalog.NewPublicDirectoryProjection(route)
	if !published {
		return nil
	}
	partitions := append([]string{directoryProductsPartitionKey()}, directoryTermPartitions(projection.SearchTerms)...)
	keys := make([]directoryProjectionKey, 0, len(partitions))
	for _, partition := range partitions {
		keys = append(keys, directoryProjectionKey{partitionKey: partition, sortKey: directoryProductSortKey(projection)})
	}
	return keys
}

func (repository *CatalogRepository) directoryProjectionPuts(route catalog.PaidRoute) ([]types.TransactWriteItem, error) {
	projection, published := catalog.NewPublicDirectoryProjection(route)
	if !published {
		return nil, nil
	}
	partitions := append([]string{directoryProductsPartitionKey()}, directoryTermPartitions(projection.SearchTerms)...)
	writes := make([]types.TransactWriteItem, 0, len(partitions))
	for _, partition := range partitions {
		record, err := newStoredRecord(partition, directoryProductSortKey(projection), "publicDirectoryProduct", projection)
		if err != nil {
			return nil, err
		}
		item, err := marshalStoredRecord(record)
		if err != nil {
			return nil, err
		}
		writes = append(writes, types.TransactWriteItem{Put: &types.Put{TableName: &repository.tableName, Item: item}})
	}
	return writes, nil
}

func (repository *CatalogRepository) directoryProjectionDeletes(route catalog.PaidRoute) []types.TransactWriteItem {
	keys := directoryProjectionKeys(route)
	writes := make([]types.TransactWriteItem, 0, len(keys))
	for _, key := range keys {
		writes = append(writes, types.TransactWriteItem{Delete: &types.Delete{
			TableName: &repository.tableName,
			Key: map[string]types.AttributeValue{
				"PK": stringAttributeValue(key.partitionKey),
				"SK": stringAttributeValue(key.sortKey),
			},
		}})
	}
	return writes
}

func (repository *CatalogRepository) obsoleteDirectoryProjectionDeletes(previous, current catalog.PaidRoute) []types.TransactWriteItem {
	currentKeys := make(map[directoryProjectionKey]struct{})
	for _, key := range directoryProjectionKeys(current) {
		currentKeys[key] = struct{}{}
	}
	if len(currentKeys) == 0 {
		return repository.directoryProjectionDeletes(previous)
	}
	writes := make([]types.TransactWriteItem, 0)
	for _, key := range directoryProjectionKeys(previous) {
		if _, retained := currentKeys[key]; retained {
			continue
		}
		writes = append(writes, types.TransactWriteItem{Delete: &types.Delete{
			TableName: &repository.tableName,
			Key: map[string]types.AttributeValue{
				"PK": stringAttributeValue(key.partitionKey),
				"SK": stringAttributeValue(key.sortKey),
			},
		}})
	}
	return writes
}

func directoryTermPartitions(terms []string) []string {
	partitions := make([]string, 0, len(terms))
	for _, term := range terms {
		partitions = append(partitions, directoryTermPartitionKey(term))
	}
	return partitions
}

func (repository *CatalogRepository) ListPublicDirectoryCandidates(ctx context.Context, query catalog.PublicDirectoryQuery) (catalog.PublicDirectoryCandidatePage, error) {
	partition := directoryProductsPartitionKey()
	if query.SearchTerm != "" {
		partition = directoryTermPartitionKey(query.SearchTerm)
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 24
	}
	keyCondition := "PK = :partitionKey"
	values := map[string]types.AttributeValue{":partitionKey": stringAttributeValue(partition)}
	if query.After != "" {
		keyCondition += " AND SK > :after"
		values[":after"] = stringAttributeValue(query.After)
	}
	output, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName: &repository.tableName, KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: values, Limit: int32Pointer(int32(limit)), ScanIndexForward: boolPointer(true),
	})
	if err != nil {
		return catalog.PublicDirectoryCandidatePage{}, err
	}
	items := make([]catalog.PublicDirectoryCandidate, 0, len(output.Items))
	for _, item := range output.Items {
		var projection catalog.PublicDirectoryProjection
		if err := unmarshalPayload(item, &projection); err != nil {
			return catalog.PublicDirectoryCandidatePage{}, err
		}
		sortKey, ok := item["SK"].(*types.AttributeValueMemberS)
		if !ok || sortKey.Value == "" {
			return catalog.PublicDirectoryCandidatePage{}, fmt.Errorf("public directory item is missing its sort key")
		}
		items = append(items, catalog.PublicDirectoryCandidate{Projection: projection, SortKey: sortKey.Value})
	}
	return catalog.PublicDirectoryCandidatePage{Items: items, Exhausted: len(output.LastEvaluatedKey) == 0}, nil
}

func (repository *CatalogRepository) GetPublicDirectoryRoute(
	ctx context.Context,
	sellerID domain.ID,
	routeID domain.ID,
) (catalog.PaidRoute, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			sellerPartitionKey(sellerID.String()),
			routeSortKey(routeID.String()),
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return catalog.PaidRoute{}, err
	}
	var route catalog.PaidRoute
	if err := unmarshalPayload(output.Item, &route); err != nil {
		return catalog.PaidRoute{}, err
	}
	if route.SellerID != sellerID || route.RouteID != routeID {
		return catalog.PaidRoute{}, fmt.Errorf("public directory route identity does not match its key")
	}
	return route, nil
}
