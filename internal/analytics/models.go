package analytics

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/transactions"
)

var (
	ErrSellerScope        = errors.New("transaction does not belong to the requested seller")
	ErrProjectionConflict = errors.New("transaction snapshots conflict at the same version")
	ErrSummaryLimit       = errors.New("dashboard summary transaction limit exceeded")
)

// SalesAggregate is one seller, day, asset, network, route, and stage bucket.
type SalesAggregate struct {
	SellerID          domain.ID                        `json:"sellerId"`
	BucketDate        string                           `json:"bucketDate"`
	Asset             string                           `json:"asset"`
	Network           string                           `json:"network"`
	RouteID           string                           `json:"routeId,omitempty"`
	Stage             transactions.ReconciliationStage `json:"stage"`
	TransactionCount  uint64                           `json:"transactionCount"`
	Amount            domain.Amount                    `json:"amount"`
	LastTransactionAt domain.Timestamp                 `json:"lastTransactionAt"`
}

// DashboardSummaryResponse contains one bounded seller reporting window.
type DashboardSummaryResponse struct {
	TransactionCount int              `json:"transactionCount"`
	Aggregates       []SalesAggregate `json:"aggregates"`
}

// TransactionPageReader loads authorized filtered transaction pages.
type TransactionPageReader interface {
	ListSellerFiltered(
		context.Context,
		string,
		transactions.SellerTransactionQuery,
	) (transactions.ListResponse, error)
}

type aggregateKey struct {
	bucketDate string
	asset      string
	network    string
	routeID    string
	stage      transactions.ReconciliationStage
}
