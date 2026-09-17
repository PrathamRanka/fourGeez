package payments

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"github.com/fourgeez/agentpay/internal/domain"
)

const mockPaymentIdentifierPrefix = "mock_"

// MockAdapter provides deterministic payment behavior for tests and local use.
type MockAdapter struct{}

// NewMockAdapter creates a deterministic payment adapter.
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{}
}

// CreateChallenge validates and encodes exact-payment requirements.
func (adapter *MockAdapter) CreateChallenge(
	ctx context.Context,
	requirements Requirements,
) (Challenge, error) {
	if err := ctx.Err(); err != nil {
		return Challenge{}, err
	}
	if err := validateRequirements(requirements); err != nil {
		return Challenge{}, err
	}

	payload := struct {
		Scheme            string        `json:"scheme"`
		Network           string        `json:"network"`
		Asset             string        `json:"asset"`
		Amount            domain.Amount `json:"amount"`
		PayTo             string        `json:"payTo"`
		ResourceURL       string        `json:"resource"`
		Description       string        `json:"description,omitempty"`
		MIMEType          string        `json:"mimeType,omitempty"`
		MaxTimeoutSeconds int           `json:"maxTimeoutSeconds"`
	}{
		Scheme:            requirements.Scheme,
		Network:           requirements.Network,
		Asset:             requirements.Asset,
		Amount:            requirements.Amount,
		PayTo:             requirements.PayTo,
		ResourceURL:       requirements.ResourceURL,
		Description:       requirements.Description,
		MIMEType:          requirements.MIMEType,
		MaxTimeoutSeconds: requirements.MaxTimeoutSeconds,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Challenge{}, err
	}

	return Challenge{
		Requirements: requirements,
		Header:       base64.StdEncoding.EncodeToString(encoded),
	}, nil
}

// Verify classifies mock proofs and derives a stable non-sensitive identifier.
func (adapter *MockAdapter) Verify(
	ctx context.Context,
	proof string,
	requirements Requirements,
) (VerificationResult, error) {
	if err := ctx.Err(); err != nil {
		return VerificationResult{}, err
	}
	if err := validateRequirements(requirements); err != nil {
		return VerificationResult{}, err
	}
	if err := mockProofError(proof); err != nil {
		return VerificationResult{}, err
	}

	return VerificationResult{
		Valid:             true,
		PaymentIdentifier: mockPaymentIdentifier(proof),
	}, nil
}

// Settle returns deterministic settlement metadata for an approved mock proof.
func (adapter *MockAdapter) Settle(
	ctx context.Context,
	proof string,
	requirements Requirements,
) (SettlementResult, error) {
	verification, err := adapter.Verify(ctx, proof, requirements)
	if err != nil {
		return SettlementResult{}, err
	}

	responsePayload := struct {
		Success           bool   `json:"success"`
		PaymentIdentifier string `json:"paymentIdentifier"`
	}{
		Success:           true,
		PaymentIdentifier: verification.PaymentIdentifier,
	}
	encoded, err := json.Marshal(responsePayload)
	if err != nil {
		return SettlementResult{}, err
	}

	return SettlementResult{
		Settled:           true,
		PaymentIdentifier: verification.PaymentIdentifier,
		ResponseHeader:    base64.StdEncoding.EncodeToString(encoded),
	}, nil
}

// validateRequirements rejects incomplete or non-positive exact-payment terms.
func validateRequirements(requirements Requirements) error {
	if requirements.Scheme != ExactScheme {
		return domain.NewValidationError(
			"scheme",
			"exact",
			"must use exact payment",
		)
	}
	if requirements.Network == "" {
		return domain.NewValidationError(
			"network",
			"required",
			"is required",
		)
	}
	if requirements.Asset == "" {
		return domain.NewValidationError(
			"asset",
			"required",
			"is required",
		)
	}
	if requirements.Amount.IsZero() {
		return domain.NewValidationError(
			"amount",
			"positive",
			"must be greater than zero",
		)
	}
	if requirements.PayTo == "" {
		return domain.NewValidationError(
			"payTo",
			"required",
			"is required",
		)
	}
	return nil
}

// mockProofError maps deterministic fixtures to payment failure classes.
func mockProofError(proof string) error {
	switch proof {
	case MockApprovedProof:
		return nil
	case MockTimeoutProof:
		return ErrPaymentTimeout
	case MockUnavailableProof:
		return ErrPaymentUnavailable
	default:
		return ErrPaymentRejected
	}
}

// mockPaymentIdentifier hashes proof material so callers never retain it raw.
func mockPaymentIdentifier(proof string) string {
	digest := sha256.Sum256([]byte(proof))
	return mockPaymentIdentifierPrefix + hex.EncodeToString(digest[:])
}
