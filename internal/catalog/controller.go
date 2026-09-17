package catalog

import (
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
)

// Activate enables the seller after a signing secret has been provisioned.
func (seller *Seller) Activate(signingSecretRef string, changedAt domain.Timestamp) error {
	trimmedReference := strings.TrimSpace(signingSecretRef)
	if trimmedReference == "" || len(trimmedReference) > maximumSigningReferenceLength {
		return domain.NewValidationError(
			"signingSecretRef",
			"required",
			"must reference a provisioned seller signing secret",
		)
	}
	if changedAt.Before(seller.UpdatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}

	seller.SigningSecretRef = trimmedReference
	seller.Status = SellerStatusActive
	seller.UpdatedAt = changedAt
	seller.Version++
	return nil
}

// Suspend prevents the seller from issuing new payment challenges.
func (seller *Seller) Suspend(changedAt domain.Timestamp) error {
	if changedAt.Before(seller.UpdatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}

	seller.Status = SellerStatusSuspended
	seller.UpdatedAt = changedAt
	seller.Version++
	return nil
}

// ChangePrice updates the route price used by future purchase intents.
func (paidRoute *PaidRoute) ChangePrice(
	amount domain.Amount,
	changedAt domain.Timestamp,
) error {
	if amount.IsZero() {
		return domain.NewValidationError("amount", "positive", "must be greater than zero")
	}
	if changedAt.Before(paidRoute.UpdatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	if paidRoute.Amount == amount {
		return nil
	}

	paidRoute.Amount = amount
	paidRoute.UpdatedAt = changedAt
	paidRoute.Version++
	return nil
}
