package payments

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	// ExactScheme identifies x402 exact-price payments.
	ExactScheme = "exact"
	// BaseSepoliaNetwork identifies the supported x402 test network.
	BaseSepoliaNetwork = "eip155:84532"
	// BaseSepoliaUSDCAsset identifies USDC on Base Sepolia.
	BaseSepoliaUSDCAsset = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"

	// MockApprovedProof deterministically represents an accepted payment.
	MockApprovedProof = "mock-approved-proof"
	// MockRejectedProof deterministically represents a rejected payment.
	MockRejectedProof = "mock-rejected-proof"
	// MockTimeoutProof deterministically represents a facilitator timeout.
	MockTimeoutProof = "mock-timeout-proof"
	// MockUnavailableProof deterministically represents facilitator unavailability.
	MockUnavailableProof = "mock-unavailable-proof"
)

var (
	// ErrPaymentRejected reports a proof that cannot authorize payment.
	ErrPaymentRejected = errors.New("payment rejected")
	// ErrPaymentTimeout reports that payment verification exceeded its deadline.
	ErrPaymentTimeout = errors.New("payment verification timed out")
	// ErrPaymentUnavailable reports that the payment provider is unavailable.
	ErrPaymentUnavailable = errors.New("payment provider unavailable")
)

// Requirements contains the immutable terms required for exact payment.
type Requirements struct {
	Scheme            string
	Network           string
	Asset             string
	Amount            domain.Amount
	PayTo             string
	ResourceURL       string
	Description       string
	MIMEType          string
	MaxTimeoutSeconds int
}

// Challenge contains payment requirements and the encoded x402 header value.
type Challenge struct {
	Requirements Requirements
	Header       string
}

// VerificationResult contains the safe identity derived from a valid proof.
type VerificationResult struct {
	Valid             bool
	PaymentIdentifier string
}

// SettlementResult contains the safe result of settling a verified payment.
type SettlementResult struct {
	Settled           bool
	PaymentIdentifier string
	ResponseHeader    string
}

// Adapter isolates payment protocol implementations from AgentPay use cases.
type Adapter interface {
	CreateChallenge(context.Context, Requirements) (Challenge, error)
	Verify(context.Context, string, Requirements) (VerificationResult, error)
	Settle(context.Context, string, Requirements) (SettlementResult, error)
}
