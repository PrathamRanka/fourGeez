package transactions

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

const maximumDashboardWindow = 31 * 24 * time.Hour

// ParseSellerTransactionQuery parses and validates bounded HTTP query values.
func ParseSellerTransactionQuery(
	sellerID domain.ID,
	values url.Values,
	requireWindow bool,
) (SellerTransactionQuery, error) {
	query := SellerTransactionQuery{
		SellerID: sellerID,
		Asset:    strings.TrimSpace(values.Get("asset")),
		Network:  strings.TrimSpace(values.Get("network")),
		Cursor:   strings.TrimSpace(values.Get("cursor")),
		Limit:    defaultTransactionPageLimit,
	}
	if rawActivityMode := strings.TrimSpace(values.Get("activityMode")); rawActivityMode != "" {
		query.ActivityMode = ActivityMode(rawActivityMode)
		if query.ActivityMode != ActivityModeTest && query.ActivityMode != ActivityModeLive {
			return SellerTransactionQuery{}, domain.NewValidationError("activityMode", "supported", "must be test or live")
		}
	}
	if rawOutcome := strings.TrimSpace(values.Get("outcome")); rawOutcome != "" {
		query.Outcome = SellerOutcome(rawOutcome)
		if !validSellerOutcome(query.Outcome) {
			return SellerTransactionQuery{}, domain.NewValidationError("outcome", "supported", "must use a documented seller outcome")
		}
	}
	if rawLimit := values.Get("limit"); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil {
			return SellerTransactionQuery{}, domain.NewValidationError(
				"limit",
				"integer",
				"must be an integer",
			)
		}
		query.Limit = limit
	}
	if query.Limit < 1 || query.Limit > maximumTransactionPageLimit {
		return SellerTransactionQuery{}, domain.NewValidationError(
			"limit",
			"range",
			"must be between 1 and 100",
		)
	}
	if rawRouteID := values.Get("routeId"); rawRouteID != "" {
		routeID, err := domain.ParseID(rawRouteID, domain.RouteIDPrefix)
		if err != nil {
			return SellerTransactionQuery{}, err
		}
		query.RouteID = &routeID
	}
	if rawStatus := values.Get("status"); rawStatus != "" {
		query.Status = TransactionStatus(rawStatus)
		if !validStoredStatus(query.Status) {
			return SellerTransactionQuery{}, domain.NewValidationError(
				"status",
				"supported",
				"must use a documented transaction status",
			)
		}
	}
	from, err := parseOptionalQueryTime("from", values.Get("from"))
	if err != nil {
		return SellerTransactionQuery{}, err
	}
	to, err := parseOptionalQueryTime("to", values.Get("to"))
	if err != nil {
		return SellerTransactionQuery{}, err
	}
	query.From = from
	query.To = to
	if requireWindow && (from == nil || to == nil) {
		return SellerTransactionQuery{}, domain.NewValidationError(
			"from",
			"required_window",
			"from and to are required",
		)
	}
	if from != nil && to != nil {
		if !from.Time().Before(to.Time()) {
			return SellerTransactionQuery{}, domain.NewValidationError(
				"to",
				"range",
				"must be after from",
			)
		}
		if to.Time().Sub(from.Time()) > maximumDashboardWindow {
			return SellerTransactionQuery{}, domain.NewValidationError(
				"to",
				"range",
				"window must not exceed 31 days",
			)
		}
	}
	return query, nil
}

// Matches reports whether a transaction satisfies every query filter.
func (query SellerTransactionQuery) Matches(transaction Transaction) bool {
	if transaction.SellerID() != query.SellerID {
		return false
	}
	createdAt := transaction.CreatedAt().Time()
	if query.From != nil && createdAt.Before(query.From.Time()) {
		return false
	}
	if query.To != nil && !createdAt.Before(query.To.Time()) {
		return false
	}
	if query.RouteID != nil && transaction.RouteID() != *query.RouteID {
		return false
	}
	if query.Status != "" && transaction.Status() != query.Status {
		return false
	}
	if query.ActivityMode != "" && transaction.ActivityMode() != query.ActivityMode {
		return false
	}
	if query.Outcome != "" && transaction.SellerOutcomeAt(query.AsOf) != query.Outcome {
		return false
	}
	if query.Asset != "" && transaction.Asset() != query.Asset {
		return false
	}
	if query.Network != "" && transaction.Network() != query.Network {
		return false
	}
	return true
}

func validSellerOutcome(outcome SellerOutcome) bool {
	switch outcome {
	case SellerOutcomeAwaitingPayment, SellerOutcomeAbandoned,
		SellerOutcomePaymentRejected, SellerOutcomePaymentProcessing,
		SellerOutcomeFulfilling, SellerOutcomeFulfilled,
		SellerOutcomeFulfillmentFailed, SellerOutcomeDisputed,
		SellerOutcomeRefundRecommended, SellerOutcomeResolved:
		return true
	default:
		return false
	}
}

// parseOptionalQueryTime parses one RFC 3339 UTC filter boundary.
func parseOptionalQueryTime(fieldName string, raw string) (*domain.Timestamp, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil || parsed.Location() != time.UTC {
		return nil, domain.NewValidationError(
			fieldName,
			"rfc3339_utc",
			"must be an RFC 3339 UTC timestamp",
		)
	}
	timestamp := domain.NewTimestamp(parsed)
	return &timestamp, nil
}
