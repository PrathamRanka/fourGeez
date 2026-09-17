package transactions

import (
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
)

// NewTransaction validates and creates a proposed transaction.
func NewTransaction(params TransactionParams) (Transaction, error) {
	validationErrors := validateTransactionParams(params)
	if len(validationErrors) > 0 {
		return Transaction{}, validationErrors
	}

	return Transaction{
		transactionID: params.TransactionID,
		intentID:      params.IntentID,
		sellerID:      params.SellerID,
		routeID:       params.RouteID,
		buyerID:       strings.TrimSpace(params.BuyerID),
		amount:        params.Amount,
		asset:         strings.TrimSpace(params.Asset),
		network:       strings.TrimSpace(params.Network),
		status:        StatusProposed,
		createdAt:     params.CreatedAt,
		updatedAt:     params.CreatedAt,
		version:       1,
	}, nil
}

// allowedTransition reports whether a state transition belongs to the lifecycle.
func allowedTransition(current, next TransactionStatus) bool {
	switch current {
	case StatusProposed:
		return next == StatusApprovalPending || next == StatusPaymentRequired
	case StatusApprovalPending:
		return next == StatusApproved
	case StatusApproved:
		return next == StatusPaymentRequired
	case StatusPaymentRequired:
		return next == StatusPaymentVerified
	case StatusPaymentVerified:
		return next == StatusForwarded
	case StatusForwarded:
		return next == StatusFulfilled || next == StatusFailed
	case StatusFulfilled, StatusFailed:
		return next == StatusDisputed
	case StatusDisputed:
		return next == StatusRefundRecommended || next == StatusResolved
	case StatusRefundRecommended:
		return next == StatusResolved
	default:
		return false
	}
}

// validateTransactionParams validates transaction creation fields.
func validateTransactionParams(params TransactionParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	validateID := func(identifier domain.ID, prefix domain.IDPrefix, field string) {
		if _, err := domain.ParseID(identifier.String(), prefix); err != nil {
			validationErrors = append(
				validationErrors,
				domain.NewValidationError(field, "format", "has an invalid domain identifier"),
			)
		}
	}

	validateID(params.TransactionID, domain.TransactionIDPrefix, "transactionId")
	validateID(params.IntentID, domain.IntentIDPrefix, "intentId")
	validateID(params.SellerID, domain.SellerIDPrefix, "sellerId")
	validateID(params.RouteID, domain.RouteIDPrefix, "routeId")
	if strings.TrimSpace(params.BuyerID) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("buyerId", "required", "is required"))
	}
	if params.Amount.IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("amount", "positive", "must be greater than zero"))
	}
	if strings.TrimSpace(params.Asset) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("asset", "required", "is required"))
	}
	if strings.TrimSpace(params.Network) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("network", "required", "is required"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	return validationErrors
}

// intPointer returns an isolated integer pointer.
func intPointer(value int) *int {
	return &value
}
