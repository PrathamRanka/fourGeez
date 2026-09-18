package dynamodb

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// TransactionRepository persists payment and delivery state in DynamoDB.
type TransactionRepository struct {
	repositoryBase
}

// NewTransactionRepository creates a DynamoDB-backed transaction repository.
func NewTransactionRepository(client Client, tableName string) *TransactionRepository {
	return &TransactionRepository{
		repositoryBase: newRepositoryBase(client, tableName),
	}
}

// Create inserts a transaction and conditionally reserves its payment identifier.
func (repository *TransactionRepository) Create(
	ctx context.Context,
	transaction transactions.Transaction,
) error {
	transactionItem, err := marshalTransactionItem(transaction)
	if err != nil {
		return err
	}

	if transaction.PaymentIdentifier() == "" {
		_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
			TableName:           &repository.tableName,
			Item:                transactionItem,
			ConditionExpression: stringPointer(createItemCondition),
		})
		if isConditionalFailure(err) {
			return persistence.ErrAlreadyExists
		}
		return err
	}

	return repository.createWithPaymentClaim(ctx, transaction, transactionItem)
}

// Get loads and validates a transaction snapshot.
func (repository *TransactionRepository) Get(
	ctx context.Context,
	transactionID domain.ID,
) (transactions.Transaction, error) {
	output, err := repository.client.GetItem(ctx, &awssdk.GetItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			transactionPartitionKey(transactionID.String()),
			profileSortKey,
		),
		ConsistentRead: boolPointer(true),
	})
	if err != nil {
		return transactions.Transaction{}, err
	}

	var snapshot transactions.Snapshot
	if err := unmarshalPayload(output.Item, &snapshot); err != nil {
		return transactions.Transaction{}, err
	}

	return transactions.Restore(snapshot)
}

// ListBySeller queries the documented seller transaction index newest first.
func (repository *TransactionRepository) ListBySeller(
	ctx context.Context,
	sellerID domain.ID,
	limit int,
	cursor string,
) ([]transactions.Transaction, *string, error) {
	return repository.QueryBySeller(
		ctx,
		transactions.SellerTransactionQuery{
			SellerID: sellerID,
			Limit:    limit,
			Cursor:   cursor,
		},
	)
}

// QueryBySeller queries the seller index and applies bounded transaction filters.
func (repository *TransactionRepository) QueryBySeller(
	ctx context.Context,
	query transactions.SellerTransactionQuery,
) ([]transactions.Transaction, *string, error) {
	keyCondition := "GSI1PK = :sellerPartitionKey"
	ascending := false
	queryLimit := int32(query.Limit)
	input := &awssdk.QueryInput{
		TableName:              &repository.tableName,
		IndexName:              stringPointer("GSI1"),
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sellerPartitionKey": stringAttributeValue(
				sellerPartitionKey(query.SellerID.String()),
			),
		},
		Limit:            &queryLimit,
		ScanIndexForward: &ascending,
	}
	if query.Cursor != "" {
		cursorID, err := domain.ParseID(query.Cursor, domain.TransactionIDPrefix)
		if err != nil {
			return nil, nil, domain.NewValidationError(
				"cursor",
				"format",
				"must be a transaction identifier",
			)
		}
		cursorTransaction, err := repository.Get(ctx, cursorID)
		if err != nil || cursorTransaction.SellerID() != query.SellerID {
			return nil, nil, domain.NewValidationError(
				"cursor",
				"scope",
				"does not belong to this seller",
			)
		}
		input.ExclusiveStartKey = map[string]types.AttributeValue{
			"PK": stringAttributeValue(
				transactionPartitionKey(cursorID.String()),
			),
			"SK": stringAttributeValue(profileSortKey),
			"GSI1PK": stringAttributeValue(
				sellerPartitionKey(query.SellerID.String()),
			),
			"GSI1SK": stringAttributeValue(
				fmt.Sprintf(
					"TXN#%s#%s",
					cursorTransaction.CreatedAt().String(),
					cursorID.String(),
				),
			),
		}
	}

	output, err := repository.client.Query(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	page := make([]transactions.Transaction, 0, len(output.Items))
	var lastEvaluatedTransactionID string
	for _, item := range output.Items {
		var snapshot transactions.Snapshot
		if err := unmarshalPayload(item, &snapshot); err != nil {
			return nil, nil, err
		}
		transaction, err := transactions.Restore(snapshot)
		if err != nil {
			return nil, nil, err
		}
		lastEvaluatedTransactionID = transaction.TransactionID().String()
		if query.Matches(transaction) {
			page = append(page, transaction)
		}
	}
	var nextCursor *string
	if len(output.LastEvaluatedKey) > 0 && lastEvaluatedTransactionID != "" {
		value := lastEvaluatedTransactionID
		nextCursor = &value
	}
	return page, nextCursor, nil
}

// Update replaces a transaction when its stored version matches.
func (repository *TransactionRepository) Update(
	ctx context.Context,
	transaction transactions.Transaction,
	expectedVersion uint64,
) error {
	storedTransaction, err := repository.Get(ctx, transaction.TransactionID())
	if err != nil {
		return err
	}
	if storedTransaction.Version() != expectedVersion {
		return persistence.ErrConditionFailed
	}

	transactionItem, err := marshalTransactionItem(transaction)
	if err != nil {
		return err
	}

	storedPaymentIdentifier := storedTransaction.PaymentIdentifier()
	nextPaymentIdentifier := transaction.PaymentIdentifier()
	if storedPaymentIdentifier != "" && storedPaymentIdentifier != nextPaymentIdentifier {
		return persistence.ErrPaymentIdentifierConflict
	}
	if storedPaymentIdentifier == "" && nextPaymentIdentifier != "" {
		return repository.updateWithPaymentClaim(
			ctx,
			transaction,
			transactionItem,
			expectedVersion,
		)
	}

	condition := "#version = :expectedVersion"
	_, err = repository.client.PutItem(ctx, &awssdk.PutItemInput{
		TableName:           &repository.tableName,
		Item:                transactionItem,
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

// ClaimForwarding atomically moves one payment-verified transaction to forwarded.
func (repository *TransactionRepository) ClaimForwarding(
	ctx context.Context,
	transactionID domain.ID,
	expectedVersion uint64,
	forwardedAt domain.Timestamp,
) (transactions.Transaction, bool, error) {
	transaction, err := repository.Get(ctx, transactionID)
	if err != nil {
		return transactions.Transaction{}, false, err
	}
	if transaction.Version() != expectedVersion {
		return transaction, false, nil
	}
	if transaction.Status() != transactions.StatusPaymentVerified {
		return transaction, false, nil
	}
	if transaction.PaymentFinality() != transactions.PaymentFinalityFinalized {
		return transaction, false, nil
	}

	if err := transaction.MarkForwarded(forwardedAt); err != nil {
		return transactions.Transaction{}, false, err
	}

	payload, err := marshalPayload(transaction.Snapshot())
	if err != nil {
		return transactions.Transaction{}, false, err
	}

	condition := "#status = :verified AND #paymentFinality = :finalized AND #version = :expected"
	update := "SET payload = :payload, #status = :forwarded, #version = :next"
	_, err = repository.client.UpdateItem(ctx, &awssdk.UpdateItemInput{
		TableName: &repository.tableName,
		Key: primaryKey(
			transactionPartitionKey(transactionID.String()),
			profileSortKey,
		),
		ConditionExpression: &condition,
		UpdateExpression:    &update,
		ExpressionAttributeNames: map[string]string{
			"#status":          "status",
			"#paymentFinality": "paymentFinality",
			"#version":         "version",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":verified":  stringAttributeValue(string(transactions.StatusPaymentVerified)),
			":finalized": stringAttributeValue(string(transactions.PaymentFinalityFinalized)),
			":forwarded": stringAttributeValue(string(transactions.StatusForwarded)),
			":expected":  numberAttributeValue(expectedVersion),
			":next":      numberAttributeValue(transaction.Version()),
			":payload":   &types.AttributeValueMemberB{Value: payload},
		},
	})
	if isConditionalFailure(err) {
		return transaction, false, nil
	}
	if err != nil {
		return transactions.Transaction{}, false, err
	}

	return transaction, true, nil
}

// createWithPaymentClaim atomically creates a transaction and payment claim.
func (repository *TransactionRepository) createWithPaymentClaim(
	ctx context.Context,
	transaction transactions.Transaction,
	transactionItem map[string]types.AttributeValue,
) error {
	paymentClaimItem, err := marshalPaymentClaim(transaction)
	if err != nil {
		return err
	}

	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Put: &types.Put{
					TableName:           &repository.tableName,
					Item:                paymentClaimItem,
					ConditionExpression: stringPointer(createClaimCondition),
				},
			},
			{
				Put: &types.Put{
					TableName:           &repository.tableName,
					Item:                transactionItem,
					ConditionExpression: stringPointer(createItemCondition),
				},
			},
		},
	})
	if isTransactionFailure(err) {
		return persistence.ErrPaymentIdentifierConflict
	}

	return err
}

// updateWithPaymentClaim atomically reserves a payment and updates its transaction.
func (repository *TransactionRepository) updateWithPaymentClaim(
	ctx context.Context,
	transaction transactions.Transaction,
	transactionItem map[string]types.AttributeValue,
	expectedVersion uint64,
) error {
	paymentClaimItem, err := marshalPaymentClaim(transaction)
	if err != nil {
		return err
	}

	versionCondition := "#version = :expectedVersion"
	_, err = repository.client.TransactWriteItems(ctx, &awssdk.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Put: &types.Put{
					TableName:           &repository.tableName,
					Item:                paymentClaimItem,
					ConditionExpression: stringPointer(createClaimCondition),
				},
			},
			{
				Put: &types.Put{
					TableName:           &repository.tableName,
					Item:                transactionItem,
					ConditionExpression: &versionCondition,
					ExpressionAttributeNames: map[string]string{
						"#version": "version",
					},
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":expectedVersion": numberAttributeValue(expectedVersion),
					},
				},
			},
		},
	})
	if isTransactionFailure(err) {
		return persistence.ErrPaymentIdentifierConflict
	}

	return err
}

// marshalTransactionItem builds the transaction record and required indexes.
func marshalTransactionItem(
	transaction transactions.Transaction,
) (map[string]types.AttributeValue, error) {
	transactionRecord, err := newStoredRecord(
		transactionPartitionKey(transaction.TransactionID().String()),
		profileSortKey,
		"transaction",
		transaction.Snapshot(),
	)
	if err != nil {
		return nil, err
	}

	transactionRecord.Version = transaction.Version()
	transactionRecord.Status = string(transaction.Status())
	transactionRecord.PaymentFinality = string(transaction.PaymentFinality())
	transactionRecord.GSI1PK = sellerPartitionKey(transaction.SellerID().String())
	transactionRecord.GSI1SK = fmt.Sprintf(
		"TXN#%s#%s",
		transaction.CreatedAt().String(),
		transaction.TransactionID().String(),
	)
	if transaction.PaymentIdentifier() != "" {
		transactionRecord.GSI2PK = paymentPartitionKey(transaction.PaymentIdentifier())
		transactionRecord.GSI2SK = "TXN#" + transaction.TransactionID().String()
	}

	return marshalStoredRecord(transactionRecord)
}

// marshalPaymentClaim builds the payment-identifier uniqueness record.
func marshalPaymentClaim(
	transaction transactions.Transaction,
) (map[string]types.AttributeValue, error) {
	paymentClaim, err := newStoredRecord(
		paymentPartitionKey(transaction.PaymentIdentifier()),
		"CLAIM",
		"paymentClaim",
		transaction.TransactionID().String(),
	)
	if err != nil {
		return nil, err
	}

	return marshalStoredRecord(paymentClaim)
}
