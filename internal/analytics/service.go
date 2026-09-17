package analytics

import (
	"sort"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const utcDateLayout = "2006-01-02"

// Service derives deterministic sales aggregates from authoritative transactions.
type Service struct{}

// NewService creates the stateless analytics service.
func NewService() *Service {
	return &Service{}
}

// AggregateSeller groups the newest unique transaction snapshots into reporting buckets.
func (*Service) AggregateSeller(
	sellerID domain.ID,
	transactionSnapshots []transactions.Transaction,
) ([]SalesAggregate, error) {
	latestTransactions, err := newestTransactions(sellerID, transactionSnapshots)
	if err != nil {
		return nil, err
	}
	aggregates := make(map[aggregateKey]SalesAggregate)
	for _, transaction := range latestTransactions {
		reconciliation := transaction.Reconciliation()
		if reconciliation.Stage == "" {
			continue
		}
		bucketDate := transaction.CreatedAt().Time().UTC().Format(utcDateLayout)
		routeIDs := []string{"", transaction.RouteID().String()}
		for _, routeID := range routeIDs {
			key := aggregateKey{
				bucketDate: bucketDate,
				asset:      transaction.Asset(),
				network:    transaction.Network(),
				routeID:    routeID,
				stage:      reconciliation.Stage,
			}
			aggregate := aggregates[key]
			if aggregate.TransactionCount == 0 {
				aggregate = SalesAggregate{
					SellerID:          sellerID,
					BucketDate:        bucketDate,
					Asset:             transaction.Asset(),
					Network:           transaction.Network(),
					RouteID:           routeID,
					Stage:             reconciliation.Stage,
					Amount:            domain.MustParseAmount("0"),
					LastTransactionAt: transaction.UpdatedAt(),
				}
			}
			aggregate.TransactionCount++
			aggregate.Amount = aggregate.Amount.Add(transaction.Amount())
			if transaction.UpdatedAt().Time().After(aggregate.LastTransactionAt.Time()) {
				aggregate.LastTransactionAt = transaction.UpdatedAt()
			}
			aggregates[key] = aggregate
		}
	}

	result := make([]SalesAggregate, 0, len(aggregates))
	for _, aggregate := range aggregates {
		result = append(result, aggregate)
	}
	sort.Slice(result, func(leftIndex int, rightIndex int) bool {
		left := result[leftIndex]
		right := result[rightIndex]
		if left.BucketDate != right.BucketDate {
			return left.BucketDate > right.BucketDate
		}
		if left.Asset != right.Asset {
			return left.Asset < right.Asset
		}
		if left.Network != right.Network {
			return left.Network < right.Network
		}
		if left.RouteID != right.RouteID {
			return left.RouteID < right.RouteID
		}
		return left.Stage < right.Stage
	})
	return result, nil
}

// newestTransactions deduplicates retries and rejects conflicting snapshots.
func newestTransactions(
	sellerID domain.ID,
	transactionSnapshots []transactions.Transaction,
) (map[domain.ID]transactions.Transaction, error) {
	latestTransactions := make(map[domain.ID]transactions.Transaction)
	for _, transaction := range transactionSnapshots {
		if transaction.SellerID() != sellerID {
			return nil, ErrSellerScope
		}
		stored, exists := latestTransactions[transaction.TransactionID()]
		if !exists {
			latestTransactions[transaction.TransactionID()] = transaction
			continue
		}
		if !sameImmutableTransaction(stored, transaction) {
			return nil, ErrProjectionConflict
		}
		if transaction.Version() > stored.Version() {
			latestTransactions[transaction.TransactionID()] = transaction
			continue
		}
		if transaction.Version() == stored.Version() &&
			!sameReconciliationFacts(stored, transaction) {
			return nil, ErrProjectionConflict
		}
	}
	return latestTransactions, nil
}

// sameImmutableTransaction verifies identity and pricing fields cannot diverge.
func sameImmutableTransaction(
	left transactions.Transaction,
	right transactions.Transaction,
) bool {
	return left.TransactionID() == right.TransactionID() &&
		left.IntentID() == right.IntentID() &&
		left.SellerID() == right.SellerID() &&
		left.RouteID() == right.RouteID() &&
		left.BuyerID() == right.BuyerID() &&
		left.Amount().Compare(right.Amount()) == 0 &&
		left.Asset() == right.Asset() &&
		left.Network() == right.Network() &&
		left.CreatedAt() == right.CreatedAt()
}

// sameReconciliationFacts verifies equal versions describe the same reporting state.
func sameReconciliationFacts(
	left transactions.Transaction,
	right transactions.Transaction,
) bool {
	return left.Status() == right.Status() &&
		left.PaymentFinality() == right.PaymentFinality() &&
		left.PaymentReference() == right.PaymentReference() &&
		left.UpdatedAt() == right.UpdatedAt()
}
