package payments

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/domain"
	x402types "github.com/x402-foundation/x402/go/types"
)

// TestX402AdapterCreatesV2TestnetChallenge verifies the official SDK payload.
func TestX402AdapterCreatesV2TestnetChallenge(t *testing.T) {
	t.Parallel()

	requirements := validRequirements()
	challenge, err := NewX402Adapter().CreateChallenge(
		t.Context(),
		requirements,
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := base64.StdEncoding.DecodeString(challenge.Header)
	if err != nil {
		t.Fatal(err)
	}
	paymentRequired, err := x402types.ToPaymentRequired(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if paymentRequired.X402Version != 2 {
		t.Fatalf("x402Version = %d", paymentRequired.X402Version)
	}
	if paymentRequired.Resource == nil ||
		paymentRequired.Resource.URL != requirements.ResourceURL ||
		paymentRequired.Resource.Description != requirements.Description ||
		paymentRequired.Resource.MimeType != requirements.MIMEType {
		t.Fatalf("resource = %#v", paymentRequired.Resource)
	}
	if len(paymentRequired.Accepts) != 1 {
		t.Fatalf("accepts length = %d", len(paymentRequired.Accepts))
	}
	accepted := paymentRequired.Accepts[0]
	if accepted.Scheme != requirements.Scheme ||
		accepted.Network != requirements.Network ||
		accepted.Asset != requirements.Asset ||
		accepted.Amount != requirements.Amount.String() ||
		accepted.PayTo != requirements.PayTo ||
		accepted.MaxTimeoutSeconds != requirements.MaxTimeoutSeconds {
		t.Fatalf("accepted = %#v", accepted)
	}
}

// TestMockAdapterCreatesDeterministicExactChallenge verifies mock challenge data.
func TestMockAdapterCreatesDeterministicExactChallenge(t *testing.T) {
	t.Parallel()

	adapter := NewMockAdapter()
	requirements := validRequirements()
	first, err := adapter.CreateChallenge(t.Context(), requirements)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.CreateChallenge(t.Context(), requirements)
	if err != nil {
		t.Fatal(err)
	}
	if first.Header == "" || first.Header != second.Header {
		t.Fatalf("challenge headers = %q and %q", first.Header, second.Header)
	}
	if first.Requirements.Amount != requirements.Amount ||
		first.Requirements.PayTo != requirements.PayTo {
		t.Fatalf("challenge = %#v", first)
	}
}

// TestMockAdapterVerifiesAndSettlesApprovedProof verifies the success fixture.
func TestMockAdapterVerifiesAndSettlesApprovedProof(t *testing.T) {
	t.Parallel()

	adapter := NewMockAdapter()
	requirements := validRequirements()
	verification, err := adapter.Verify(
		t.Context(),
		MockApprovedProof,
		requirements,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !verification.Valid ||
		!strings.HasPrefix(verification.PaymentIdentifier, "mock_") {
		t.Fatalf("verification = %#v", verification)
	}
	settlement, err := adapter.Settle(
		t.Context(),
		MockApprovedProof,
		requirements,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !settlement.Settled ||
		settlement.PaymentIdentifier != verification.PaymentIdentifier ||
		settlement.ResponseHeader == "" {
		t.Fatalf("settlement = %#v", settlement)
	}
}

// TestMockAdapterReturnsDeterministicFailures verifies mock failure classes.
func TestMockAdapterReturnsDeterministicFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		proof     string
		wantError error
	}{
		{name: "missing", proof: "", wantError: ErrPaymentRejected},
		{name: "rejected", proof: MockRejectedProof, wantError: ErrPaymentRejected},
		{name: "timeout", proof: MockTimeoutProof, wantError: ErrPaymentTimeout},
		{name: "unavailable", proof: MockUnavailableProof, wantError: ErrPaymentUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewMockAdapter().Verify(
				context.Background(),
				test.proof,
				validRequirements(),
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Verify() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

// TestMockAdapterRejectsInvalidRequirements verifies exact-payment validation.
func TestMockAdapterRejectsInvalidRequirements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Requirements)
	}{
		{name: "zero amount", mutate: func(requirements *Requirements) {
			requirements.Amount = domain.MustParseAmount("0")
		}},
		{name: "missing asset", mutate: func(requirements *Requirements) {
			requirements.Asset = ""
		}},
		{name: "missing network", mutate: func(requirements *Requirements) {
			requirements.Network = ""
		}},
		{name: "missing recipient", mutate: func(requirements *Requirements) {
			requirements.PayTo = ""
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requirements := validRequirements()
			test.mutate(&requirements)
			if _, err := NewMockAdapter().CreateChallenge(
				t.Context(),
				requirements,
			); err == nil {
				t.Fatal("CreateChallenge() accepted invalid requirements")
			}
		})
	}
}

// validRequirements returns a complete exact-payment fixture.
func validRequirements() Requirements {
	return Requirements{
		Scheme:            ExactScheme,
		Network:           BaseSepoliaNetwork,
		Asset:             BaseSepoliaUSDCAsset,
		Amount:            domain.MustParseAmount("10000"),
		PayTo:             "0x1111111111111111111111111111111111111111",
		ResourceURL:       "https://api.example/pay/demo/weather",
		Description:       "Weather report",
		MIMEType:          "application/json",
		MaxTimeoutSeconds: 60,
	}
}
