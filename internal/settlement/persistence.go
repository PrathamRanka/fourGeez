package settlement

import (
	"encoding/hex"
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
)

// PaymentDestinationSnapshot is the persistence-only representation of a destination.
type PaymentDestinationSnapshot struct {
	DestinationID      domain.ID                `json:"destinationId"`
	SellerID           domain.ID                `json:"sellerId"`
	Asset              string                   `json:"asset"`
	Network            string                   `json:"network"`
	Address            string                   `json:"address"`
	Status             PaymentDestinationStatus `json:"status"`
	ChallengeHash      string                   `json:"challengeHash,omitempty"`
	ChallengeExpiresAt *domain.Timestamp        `json:"challengeExpiresAt,omitempty"`
	VerifiedAt         *domain.Timestamp        `json:"verifiedAt,omitempty"`
	CreatedAt          domain.Timestamp         `json:"createdAt"`
	UpdatedAt          domain.Timestamp         `json:"updatedAt"`
	Version            uint64                   `json:"version"`
}

// Snapshot returns all persisted fields, including the non-public challenge hash.
func (destination PaymentDestination) Snapshot() PaymentDestinationSnapshot {
	return PaymentDestinationSnapshot{
		DestinationID:      destination.DestinationID,
		SellerID:           destination.SellerID,
		Asset:              destination.Asset,
		Network:            destination.Network,
		Address:            destination.Address,
		Status:             destination.Status,
		ChallengeHash:      destination.ChallengeHash,
		ChallengeExpiresAt: cloneTimestamp(destination.ChallengeExpiresAt),
		VerifiedAt:         cloneTimestamp(destination.VerifiedAt),
		CreatedAt:          destination.CreatedAt,
		UpdatedAt:          destination.UpdatedAt,
		Version:            destination.Version,
	}
}

// RestorePaymentDestination validates and restores one persisted destination snapshot.
func RestorePaymentDestination(
	snapshot PaymentDestinationSnapshot,
) (PaymentDestination, error) {
	destination, err := NewPaymentDestination(PaymentDestinationParams{
		DestinationID: snapshot.DestinationID,
		SellerID:      snapshot.SellerID,
		Asset:         snapshot.Asset,
		Network:       snapshot.Network,
		Address:       snapshot.Address,
		CreatedAt:     snapshot.CreatedAt,
	})
	if err != nil {
		return PaymentDestination{}, err
	}
	if !validPaymentDestinationStatus(snapshot.Status) ||
		snapshot.UpdatedAt.Time().IsZero() ||
		snapshot.Version == 0 {
		return PaymentDestination{}, errors.New("invalid payment destination snapshot")
	}
	if snapshot.ChallengeHash != "" {
		digest, decodeErr := hex.DecodeString(snapshot.ChallengeHash)
		if decodeErr != nil || len(digest) != 32 || snapshot.ChallengeExpiresAt == nil {
			return PaymentDestination{}, errors.New("invalid payment destination challenge snapshot")
		}
	} else if snapshot.ChallengeExpiresAt != nil {
		return PaymentDestination{}, errors.New("invalid payment destination challenge expiry")
	}
	destination.Status = snapshot.Status
	destination.ChallengeHash = snapshot.ChallengeHash
	destination.ChallengeExpiresAt = cloneTimestamp(snapshot.ChallengeExpiresAt)
	destination.VerifiedAt = cloneTimestamp(snapshot.VerifiedAt)
	destination.UpdatedAt = snapshot.UpdatedAt
	destination.Version = snapshot.Version
	return destination, nil
}

// validPaymentDestinationStatus recognizes every documented destination state.
func validPaymentDestinationStatus(status PaymentDestinationStatus) bool {
	switch status {
	case PaymentDestinationStatusPendingVerification,
		PaymentDestinationStatusActive,
		PaymentDestinationStatusDisabled,
		PaymentDestinationStatusRotated:
		return true
	default:
		return false
	}
}

// cloneTimestamp prevents persistence snapshots from sharing mutable pointers.
func cloneTimestamp(value *domain.Timestamp) *domain.Timestamp {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
