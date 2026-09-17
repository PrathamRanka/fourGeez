package settlement

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
)

var (
	ErrOwnershipChallengeExpired    = errors.New("ownership challenge has expired")
	ErrOwnershipChallengeMismatch   = errors.New("ownership challenge does not match")
	ErrOwnershipProofInvalid        = errors.New("ownership proof is invalid")
	ErrOwnershipStateInvalid        = errors.New("payment destination cannot be verified in its current state")
	ErrRotationConfirmationRequired = errors.New(
		"rotation confirmation is required for the active payment destination",
	)
)

// PaymentDestinationStatus represents whether a destination may receive new payments.
type PaymentDestinationStatus string

const (
	PaymentDestinationStatusPendingVerification PaymentDestinationStatus = "pending_verification"
	PaymentDestinationStatusActive              PaymentDestinationStatus = "active"
	PaymentDestinationStatusDisabled            PaymentDestinationStatus = "disabled"
	PaymentDestinationStatusRotated             PaymentDestinationStatus = "rotated"
)

// PaymentDestinationParams contains the values required to create a destination.
type PaymentDestinationParams struct {
	DestinationID domain.ID
	SellerID      domain.ID
	Asset         string
	Network       string
	Address       string
	CreatedAt     domain.Timestamp
}

// PaymentDestination stores public payout configuration without wallet secrets.
type PaymentDestination struct {
	DestinationID      domain.ID                `json:"destinationId"`
	SellerID           domain.ID                `json:"sellerId"`
	Asset              string                   `json:"asset"`
	Network            string                   `json:"network"`
	Address            string                   `json:"address"`
	Status             PaymentDestinationStatus `json:"status"`
	ChallengeHash      string                   `json:"-"`
	ChallengeExpiresAt *domain.Timestamp        `json:"challengeExpiresAt,omitempty"`
	VerifiedAt         *domain.Timestamp        `json:"verifiedAt,omitempty"`
	CreatedAt          domain.Timestamp         `json:"createdAt"`
	UpdatedAt          domain.Timestamp         `json:"updatedAt"`
	Version            uint64                   `json:"version"`
}

// CreatePaymentDestinationRequest contains seller-supplied public destination fields.
type CreatePaymentDestinationRequest struct {
	Asset   string `json:"asset"`
	Network string `json:"network"`
	Address string `json:"address"`
}

// PaymentDestinationListResponse contains a deterministic seller-scoped list.
type PaymentDestinationListResponse struct {
	Items []PaymentDestination `json:"items"`
}

// OwnershipChallengeResponse returns one raw challenge without persisting it.
type OwnershipChallengeResponse struct {
	Challenge string           `json:"challenge"`
	ExpiresAt domain.Timestamp `json:"expiresAt"`
}

// VerifyOwnershipRequest contains one signed challenge and rotation consent.
type VerifyOwnershipRequest struct {
	Challenge       string `json:"challenge"`
	Signature       string `json:"signature"`
	ConfirmRotation bool   `json:"confirmRotation"`
}

// PaymentDestinationActivation describes one atomic activation and optional rotation.
type PaymentDestinationActivation struct {
	Destination            PaymentDestination
	ExpectedVersion        uint64
	RotatedDestination     *PaymentDestination
	RotatedExpectedVersion uint64
}

// Repository persists payment destinations without crossing feature ownership.
type Repository interface {
	Create(context.Context, PaymentDestination) error
	Get(context.Context, domain.ID, domain.ID) (PaymentDestination, error)
	ListBySeller(context.Context, domain.ID) ([]PaymentDestination, error)
	SaveChallenge(context.Context, PaymentDestination, uint64) error
	Activate(context.Context, PaymentDestinationActivation) error
}

// SellerAuthorizer verifies that an authenticated subject owns a seller.
type SellerAuthorizer interface {
	AuthorizeSeller(context.Context, string, domain.ID) error
}

// OwnershipNonceGenerator creates unpredictable one-time challenge entropy.
type OwnershipNonceGenerator interface {
	NewNonce() (string, error)
}

// OwnershipVerifier validates a proof without receiving or retaining wallet secrets.
type OwnershipVerifier interface {
	VerifyOwnership(context.Context, PaymentDestination, string, string) (bool, error)
}
