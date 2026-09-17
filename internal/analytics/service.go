package analytics

import (
	"context"
	"sort"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const utcDateLayout = "2006-01-02"

const (
	dashboardSummaryPageSize = 100
	dashboardSummaryLimit    = 1000
)

// Service derives deterministic sales aggregates from authoritative transactions.
type Service struct{}

// NewService creates the stateless analytics service.
func NewService() *Service {
	return &Service{}
}

// DashboardService loads bounded transaction pages and builds seller summaries.
type DashboardService struct {
	transactionReader TransactionPageReader
	aggregator        *Service
}

// NewDashboardService creates the dashboard summary use case.
func NewDashboardService(
	transactionReader TransactionPageReader,
	aggregator *Service,
) *DashboardService {
	return &DashboardService{
		transactionReader: transactionReader,
		aggregator:        aggregator,
	}
}

// Summary loads a bounded window and returns deterministic aggregate rows.
func (service *DashboardService) Summary(
	ctx context.Context,
	ownerSubject string,
	query transactions.SellerTransactionQuery,
) (DashboardSummaryResponse, error) {
	query.Limit = dashboardSummaryPageSize
	query.Cursor = ""
	responses := make([]transactions.Response, 0)
	for {
		page, err := service.transactionReader.ListSellerFiltered(
			ctx,
			ownerSubject,
			query,
		)
		if err != nil {
			return DashboardSummaryResponse{}, err
		}
		responses = append(responses, page.Items...)
		if len(responses) > dashboardSummaryLimit {
			return DashboardSummaryResponse{}, ErrSummaryLimit
		}
		if page.NextCursor == nil {
			break
		}
		query.Cursor = *page.NextCursor
	}
	aggregates, err := service.aggregator.AggregateResponses(query.SellerID, responses)
	if err != nil {
		return DashboardSummaryResponse{}, err
	}
	return DashboardSummaryResponse{
		TransactionCount: len(responses),
		Aggregates:       aggregates,
	}, nil
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

// AggregateResponses groups public transaction rows returned by a bounded seller query.
func (*Service) AggregateResponses(
	sellerID domain.ID,
	responses []transactions.Response,
) ([]SalesAggregate, error) {
	aggregates := make(map[aggregateKey]SalesAggregate)
	for _, response := range responses {
		if response.SellerID != sellerID {
			return nil, ErrSellerScope
		}
		if response.Reconciliation == nil || response.Reconciliation.Stage == "" {
			continue
		}
		bucketDate := response.CreatedAt.Time().UTC().Format(utcDateLayout)
		for _, routeID := range []string{"", response.RouteID.String()} {
			key := aggregateKey{
				bucketDate: bucketDate,
				asset:      response.Asset,
				network:    response.Network,
				routeID:    routeID,
				stage:      response.Reconciliation.Stage,
			}
			aggregates[key] = addAggregate(
				aggregates[key],
				sellerID,
				bucketDate,
				response.Asset,
				response.Network,
				routeID,
				response.Reconciliation.Stage,
				response.Amount,
				response.UpdatedAt,
			)
		}
	}
	return sortedAggregates(aggregates), nil
}

// addAggregate adds one transaction to an exact reporting bucket.
func addAggregate(
	aggregate SalesAggregate,
	sellerID domain.ID,
	bucketDate string,
	asset string,
	network string,
	routeID string,
	stage transactions.ReconciliationStage,
	amount domain.Amount,
	updatedAt domain.Timestamp,
) SalesAggregate {
	if aggregate.TransactionCount == 0 {
		aggregate = SalesAggregate{
			SellerID:          sellerID,
			BucketDate:        bucketDate,
			Asset:             asset,
			Network:           network,
			RouteID:           routeID,
			Stage:             stage,
			Amount:            domain.MustParseAmount("0"),
			LastTransactionAt: updatedAt,
		}
	}
	aggregate.TransactionCount++
	aggregate.Amount = aggregate.Amount.Add(amount)
	if updatedAt.Time().After(aggregate.LastTransactionAt.Time()) {
		aggregate.LastTransactionAt = updatedAt
	}
	return aggregate
}

// sortedAggregates returns stable dashboard ordering.
func sortedAggregates(aggregates map[aggregateKey]SalesAggregate) []SalesAggregate {
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
	return result
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
