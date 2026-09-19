package dynamodb

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const (
	createItemCondition  = "attribute_not_exists(PK) AND attribute_not_exists(SK)"
	createClaimCondition = "attribute_not_exists(PK)"
)

// CatalogRepository persists sellers and paid routes in DynamoDB.
type CatalogRepository struct {
	repositoryBase
}

// NewCatalogRepository creates a DynamoDB-backed catalog repository.
func NewCatalogRepository(client Client, tableName string) *CatalogRepository {
	return &CatalogRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// CreateSeller inserts a seller and atomically reserves its public slug.
func (repository *CatalogRepository) CreateSeller(ctx context.Context, seller catalog.Seller) error {
	sellerRecord, err := newStoredRecord(
		sellerPartitionKey(seller.SellerID.String()),
		profileSortKey,
		"seller",
		seller,
	)
	if err != nil {
		return err
	}

	sellerRecord.Version = seller.Version
	sellerRecord.GSI3PK = slugPartitionKey(seller.Slug)
	sellerRecord.GSI3SK = sellerPartitionKey(seller.SellerID.String())

	sellerItem, err := marshalStoredRecord(sellerRecord)
	if err != nil {
		return err
	}

	slugClaim, err := newStoredRecord(
		slugPartitionKey(seller.Slug),
		"CLAIM",
		"slugClaim",
		seller.SellerID.String(),
	)
	if err != nil {
		return err
	}

	slugClaimItem, err := marshalStoredRecord(slugClaim)
	if err != nil {
		return err
	}
	ownerClaim, err := newStoredRecord(
		ownerSubjectPartitionKey(seller.OwnerSubject),
		ownerSubjectSellerSortKey,
		"sellerOwnerClaim",
		seller.SellerID.String(),
	)
	if err != nil {
		return err
	}
	ownerClaimItem, err := marshalStoredRecord(ownerClaim)
	if err != nil {
		return err
	}

	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Put: &types.Put{
					TableName:           &repository.tableName,
					Item:                slugClaimItem,
					ConditionExpression: stringPointer(createClaimCondition),
				},
			},
			{
				Put: &types.Put{
					TableName:           &repository.tableName,
					Item:                ownerClaimItem,
					ConditionExpression: stringPointer(createClaimCondition),
				},
			},
			{
				Put: &types.Put{
					TableName:           &repository.tableName,
					Item:                sellerItem,
					ConditionExpression: stringPointer(createItemCondition),
				},
			},
		},
	})
	if isTransactionFailure(err) {
		return persistence.ErrAlreadyExists
	}

	return err
}

// ResolveSellerByOwnerSubject loads the one seller bound to an identity subject.
func (repository *CatalogRepository) ResolveSellerByOwnerSubject(ctx context.Context, ownerSubject string) (catalog.Seller, error) {
	claimOutput, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName:      &repository.tableName,
		Key:            primaryKey(ownerSubjectPartitionKey(ownerSubject), ownerSubjectSellerSortKey),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return catalog.Seller{}, err
	}
	var rawSellerID string
	if err := unmarshalPayload(claimOutput.Item, &rawSellerID); err != nil {
		return catalog.Seller{}, err
	}
	sellerID, err := domain.ParseID(rawSellerID, domain.SellerIDPrefix)
	if err != nil {
		return catalog.Seller{}, err
	}
	return repository.GetSeller(ctx, sellerID)
}

// GetSeller loads a seller by identifier using a strongly consistent read.
func (repository *CatalogRepository) GetSeller(ctx context.Context, sellerID domain.ID) (catalog.Seller, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			sellerPartitionKey(sellerID.String()),
			profileSortKey,
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return catalog.Seller{}, err
	}

	var seller catalog.Seller
	if err := unmarshalPayload(output.Item, &seller); err != nil {
		return catalog.Seller{}, err
	}

	return seller, nil
}

// ResolveSellerBySlug loads the seller projected into the storefront slug index.
func (repository *CatalogRepository) ResolveSellerBySlug(ctx context.Context, slug string) (catalog.Seller, error) {
	indexName := "GSI3"
	keyCondition := "GSI3PK = :partitionKey"

	output, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName:              &repository.tableName,
		IndexName:              &indexName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":partitionKey": stringAttributeValue(slugPartitionKey(slug)),
		},
		Limit: int32Pointer(1),
	})
	if err != nil {
		return catalog.Seller{}, err
	}
	if len(output.Items) == 0 {
		return catalog.Seller{}, persistence.ErrNotFound
	}

	var seller catalog.Seller
	if err := unmarshalPayload(output.Items[0], &seller); err != nil {
		return catalog.Seller{}, err
	}

	return seller, nil
}

// UpdateSeller replaces a seller only when its stored version matches.
func (repository *CatalogRepository) UpdateSeller(
	ctx context.Context,
	seller catalog.Seller,
	expectedVersion uint64,
) error {
	storedSeller, err := repository.GetSeller(ctx, seller.SellerID)
	if err != nil {
		return err
	}
	if storedSeller.Slug != seller.Slug {
		return persistence.ErrConditionFailed
	}

	sellerRecord, err := newStoredRecord(
		sellerPartitionKey(seller.SellerID.String()),
		profileSortKey,
		"seller",
		seller,
	)
	if err != nil {
		return err
	}

	sellerRecord.Version = seller.Version
	sellerRecord.GSI3PK = slugPartitionKey(seller.Slug)
	sellerRecord.GSI3SK = sellerPartitionKey(seller.SellerID.String())

	sellerItem, err := marshalStoredRecord(sellerRecord)
	if err != nil {
		return err
	}

	return repository.putWithExpectedVersion(ctx, sellerItem, expectedVersion)
}

// CreateRoute inserts a seller-owned paid route when the key is unused.
func (repository *CatalogRepository) CreateRoute(ctx context.Context, route catalog.PaidRoute) error {
	if _, err := repository.GetSeller(ctx, route.SellerID); err != nil {
		return err
	}

	routeRecord, err := newStoredRecord(
		sellerPartitionKey(route.SellerID.String()),
		routeSortKey(route.RouteID.String()),
		"paidRoute",
		route,
	)
	if err != nil {
		return err
	}

	routeRecord.Version = route.Version
	routeRecord.GSI4PK = routeSortKey(route.RouteID.String())
	routeRecord.GSI4SK = sellerPartitionKey(route.SellerID.String())
	routeItem, err := marshalStoredRecord(routeRecord)
	if err != nil {
		return err
	}

	productSlugClaim, err := newStoredRecord(
		sellerPartitionKey(route.SellerID.String()),
		productSlugClaimSortKey(route.ProductSlug),
		"productSlugClaim",
		route.RouteID.String(),
	)
	if err != nil {
		return err
	}
	productSlugClaimItem, err := marshalStoredRecord(productSlugClaim)
	if err != nil {
		return err
	}

	writes := []types.TransactWriteItem{
		{Put: &types.Put{
			TableName:           &repository.tableName,
			Item:                productSlugClaimItem,
			ConditionExpression: stringPointer(createItemCondition),
		}},
		{Put: &types.Put{
			TableName:           &repository.tableName,
			Item:                routeItem,
			ConditionExpression: stringPointer(createItemCondition),
		}},
	}
	directoryWrites, err := repository.directoryProjectionPuts(route)
	if err != nil {
		return err
	}
	writes = append(writes, directoryWrites...)
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: writes,
	})
	if isTransactionFailure(err) {
		return persistence.ErrAlreadyExists
	}

	return err
}

// GetRoute loads a paid route from its owning seller partition.
func (repository *CatalogRepository) GetRoute(
	ctx context.Context,
	routeID domain.ID,
) (catalog.PaidRoute, error) {
	indexName := "GSI4"
	keyCondition := "GSI4PK = :partitionKey"
	output, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName:              &repository.tableName,
		IndexName:              &indexName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":partitionKey": stringAttributeValue(routeSortKey(routeID.String())),
		},
		Limit: int32Pointer(1),
	})
	if err != nil {
		return catalog.PaidRoute{}, err
	}
	if len(output.Items) == 0 {
		return catalog.PaidRoute{}, persistence.ErrNotFound
	}

	var route catalog.PaidRoute
	if err := unmarshalPayload(output.Items[0], &route); err != nil {
		return catalog.PaidRoute{}, err
	}

	return route, nil
}

// UpdateRoute replaces a paid route only when its stored version matches.
func (repository *CatalogRepository) UpdateRoute(
	ctx context.Context,
	route catalog.PaidRoute,
	expectedVersion uint64,
) error {
	storedRoute, err := repository.GetRoute(ctx, route.RouteID)
	if err != nil {
		return err
	}
	if storedRoute.SellerID != route.SellerID {
		return persistence.ErrConditionFailed
	}
	if storedRoute.PathPattern != route.PathPattern ||
		(storedRoute.ProductSlug != "" && storedRoute.ProductSlug != route.ProductSlug) ||
		(storedRoute.DisplayName != "" && storedRoute.DisplayName != route.DisplayName) {
		return persistence.ErrConditionFailed
	}

	routeRecord, err := newStoredRecord(
		sellerPartitionKey(route.SellerID.String()),
		routeSortKey(route.RouteID.String()),
		"paidRoute",
		route,
	)
	if err != nil {
		return err
	}

	routeRecord.Version = route.Version
	routeRecord.GSI4PK = routeSortKey(route.RouteID.String())
	routeRecord.GSI4SK = sellerPartitionKey(route.SellerID.String())
	routeItem, err := marshalStoredRecord(routeRecord)
	if err != nil {
		return err
	}

	versionCondition := "#version = :expectedVersion"
	writes := make([]types.TransactWriteItem, 0, 20)
	if storedRoute.ProductSlug == "" {
		productSlugClaim, claimErr := newStoredRecord(sellerPartitionKey(route.SellerID.String()), productSlugClaimSortKey(route.ProductSlug), "productSlugClaim", route.RouteID.String())
		if claimErr != nil {
			return claimErr
		}
		productSlugClaimItem, claimErr := marshalStoredRecord(productSlugClaim)
		if claimErr != nil {
			return claimErr
		}
		writes = append(writes, types.TransactWriteItem{Put: &types.Put{TableName: &repository.tableName, Item: productSlugClaimItem, ConditionExpression: stringPointer(createItemCondition)}})
	}
	writes = append(writes,
		types.TransactWriteItem{Put: &types.Put{
			TableName:                &repository.tableName,
			Item:                     routeItem,
			ConditionExpression:      &versionCondition,
			ExpressionAttributeNames: map[string]string{"#version": "version"},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":expectedVersion": numberAttributeValue(expectedVersion),
			},
		}},
	)
	writes = append(writes, repository.obsoleteDirectoryProjectionDeletes(storedRoute, route)...)
	if _, published := catalog.NewPublicDirectoryProjection(route); published {
		directoryWrites, projectionErr := repository.directoryProjectionPuts(route)
		if projectionErr != nil {
			return projectionErr
		}
		writes = append(writes, directoryWrites...)
	}
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: writes,
	})
	if isTransactionFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

// ListRoutesBySeller returns all route items in a seller partition.
func (repository *CatalogRepository) ListRoutesBySeller(
	ctx context.Context,
	sellerID domain.ID,
) ([]catalog.PaidRoute, error) {
	keyCondition := "PK = :partitionKey AND begins_with(SK, :sortKeyPrefix)"
	output, err := repository.client.Query(ctx, &awssdk.QueryInput{
		TableName:              &repository.tableName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":partitionKey":  stringAttributeValue(sellerPartitionKey(sellerID.String())),
			":sortKeyPrefix": stringAttributeValue("ROUTE#"),
		},
	})
	if err != nil {
		return nil, err
	}

	routes := make([]catalog.PaidRoute, 0, len(output.Items))
	for _, item := range output.Items {
		var route catalog.PaidRoute
		if err := unmarshalPayload(item, &route); err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}

	return routes, nil
}

// putWithExpectedVersion performs an optimistic-concurrency replacement.
func (repository *CatalogRepository) putWithExpectedVersion(
	ctx context.Context,
	item map[string]types.AttributeValue,
	expectedVersion uint64,
) error {
	condition := "#version = :expectedVersion"
	_, err := repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                item,
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
