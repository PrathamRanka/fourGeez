package payments

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	x402 "github.com/x402-foundation/x402/go/v2"
	x402types "github.com/x402-foundation/x402/go/v2/types"
)

const testVerificationTimeout = 20 * time.Millisecond

// TestX402AdapterVerifiesAndSettlesExactPayment verifies facilitator success.
func TestX402AdapterVerifiesAndSettlesExactPayment(t *testing.T) {
	t.Parallel()

	facilitator := &fakeFacilitatorClient{
		verifyResponse: &x402.VerifyResponse{
			IsValid: true,
			Payer:   "0x2222222222222222222222222222222222222222",
		},
		settleResponse: &x402.SettleResponse{
			Success:     true,
			Transaction: "0xtestnettransaction",
			Network:     x402.Network(BaseSepoliaNetwork),
			Amount:      "10000",
			Payer:       "0x2222222222222222222222222222222222222222",
		},
	}
	adapter := NewX402AdapterWithFacilitator(
		facilitator,
		testVerificationTimeout,
	)
	proof := validPaymentProof(t, validRequirements())

	verification, err := adapter.Verify(
		t.Context(),
		proof,
		validRequirements(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !verification.Valid ||
		!strings.HasPrefix(verification.PaymentIdentifier, "x402_") ||
		verification.PayerAddress != facilitator.verifyResponse.Payer {
		t.Fatalf("verification = %#v", verification)
	}

	settlement, err := adapter.Settle(
		t.Context(),
		proof,
		validRequirements(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !settlement.Settled ||
		settlement.PaymentIdentifier != verification.PaymentIdentifier ||
		settlement.PaymentReference != facilitator.settleResponse.Transaction ||
		settlement.PayerAddress != facilitator.settleResponse.Payer {
		t.Fatalf("settlement = %#v", settlement)
	}
	decoded, err := base64.StdEncoding.DecodeString(settlement.ResponseHeader)
	if err != nil {
		t.Fatal(err)
	}
	var response x402.SettleResponse
	if err := json.Unmarshal(decoded, &response); err != nil {
		t.Fatal(err)
	}
	if response.Transaction != facilitator.settleResponse.Transaction {
		t.Fatalf("transaction = %q", response.Transaction)
	}
}

// TestX402AdapterRejectsMismatchedSettlementFacts prevents false finality.
func TestX402AdapterRejectsMismatchedSettlementFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response *x402.SettleResponse
	}{
		{
			name: "wrong network",
			response: &x402.SettleResponse{
				Success:     true,
				Transaction: "0xtestnettransaction",
				Network:     "eip155:1",
				Amount:      "10000",
			},
		},
		{
			name: "wrong amount",
			response: &x402.SettleResponse{
				Success:     true,
				Transaction: "0xtestnettransaction",
				Network:     x402.Network(BaseSepoliaNetwork),
				Amount:      "9999",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			adapter := NewX402AdapterWithFacilitator(
				&fakeFacilitatorClient{settleResponse: test.response},
				testVerificationTimeout,
			)
			_, err := adapter.Settle(
				t.Context(),
				validPaymentProof(t, validRequirements()),
				validRequirements(),
			)
			if !errors.Is(err, ErrPaymentRejected) {
				t.Fatalf("Settle() error = %v, want payment rejected", err)
			}
		})
	}
}

// TestX402AdapterRejectsModifiedPaymentProof verifies exact quote binding.
func TestX402AdapterRejectsModifiedPaymentProof(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	facilitator := &fakeFacilitatorClient{
		verify: func(context.Context, []byte, []byte) (*x402.VerifyResponse, error) {
			calls.Add(1)
			return &x402.VerifyResponse{IsValid: true}, nil
		},
	}
	adapter := NewX402AdapterWithFacilitator(
		facilitator,
		testVerificationTimeout,
	)
	modified := validRequirements()
	modified.Amount = domain.MustParseAmount("10001")
	proof := validPaymentProof(t, modified)

	_, err := adapter.Verify(t.Context(), proof, validRequirements())
	if !errors.Is(err, ErrPaymentRejected) {
		t.Fatalf("Verify() error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("facilitator calls = %d", calls.Load())
	}
}

func TestX402AdapterRejectsUnapprovedNetworkOrAssetBeforeChallenge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Requirements)
	}{
		{
			name: "mainnet network",
			mutate: func(requirements *Requirements) {
				requirements.Network = "eip155:8453"
			},
		},
		{
			name: "different token",
			mutate: func(requirements *Requirements) {
				requirements.Asset = "0x1111111111111111111111111111111111111111"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requirements := validRequirements()
			test.mutate(&requirements)
			if _, err := NewX402Adapter().CreateChallenge(t.Context(), requirements); err == nil {
				t.Fatal("CreateChallenge() error = nil")
			}
		})
	}
}

func TestX402AdapterNormalizesSymbolicUSDCToBaseSepoliaContract(t *testing.T) {
	t.Parallel()

	requirements := validRequirements()
	requirements.Asset = "USDC"
	challenge, err := NewX402Adapter().CreateChallenge(t.Context(), requirements)
	if err != nil {
		t.Fatalf("CreateChallenge() error = %v", err)
	}
	if challenge.Requirements.Asset != BaseSepoliaUSDCAsset {
		t.Fatalf("challenge asset = %q", challenge.Requirements.Asset)
	}

	paymentRequiredBytes, err := base64.StdEncoding.DecodeString(challenge.Header)
	if err != nil {
		t.Fatal(err)
	}
	paymentRequired, err := x402types.ToPaymentRequired(paymentRequiredBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(paymentRequired.Accepts) != 1 || paymentRequired.Accepts[0].Asset != BaseSepoliaUSDCAsset {
		t.Fatalf("payment requirements = %#v", paymentRequired.Accepts)
	}
}

// TestX402AdapterRejectsEveryModifiedFrozenPaymentTerm verifies that the
// client cannot substitute any exact-payment or resource binding before the
// facilitator is called.
func TestX402AdapterRejectsEveryModifiedFrozenPaymentTerm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*x402types.PaymentPayload)
		wantError error
	}{
		{name: "scheme", mutate: func(payload *x402types.PaymentPayload) { payload.Accepted.Scheme = "upto" }, wantError: ErrPaymentCapabilityUnsupported},
		{name: "network", mutate: func(payload *x402types.PaymentPayload) { payload.Accepted.Network = "eip155:1" }, wantError: ErrPaymentCapabilityUnsupported},
		{name: "asset", mutate: func(payload *x402types.PaymentPayload) {
			payload.Accepted.Asset = "0x2222222222222222222222222222222222222222"
		}, wantError: ErrPaymentCapabilityUnsupported},
		{name: "amount", mutate: func(payload *x402types.PaymentPayload) { payload.Accepted.Amount = "9999" }, wantError: ErrPaymentRejected},
		{name: "destination", mutate: func(payload *x402types.PaymentPayload) {
			payload.Accepted.PayTo = "0x3333333333333333333333333333333333333333"
		}, wantError: ErrPaymentRejected},
		{name: "timeout", mutate: func(payload *x402types.PaymentPayload) { payload.Accepted.MaxTimeoutSeconds++ }, wantError: ErrPaymentRejected},
		{name: "missing resource", mutate: func(payload *x402types.PaymentPayload) { payload.Resource = nil }, wantError: ErrPaymentRejected},
		{name: "resource URL", mutate: func(payload *x402types.PaymentPayload) { payload.Resource.URL = "https://api.example/pay/demo/other" }, wantError: ErrPaymentRejected},
		{name: "resource description", mutate: func(payload *x402types.PaymentPayload) { payload.Resource.Description = "Different product" }, wantError: ErrPaymentRejected},
		{name: "resource MIME type", mutate: func(payload *x402types.PaymentPayload) { payload.Resource.MimeType = "text/plain" }, wantError: ErrPaymentRejected},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32
			facilitator := &fakeFacilitatorClient{
				verify: func(context.Context, []byte, []byte) (*x402.VerifyResponse, error) {
					calls.Add(1)
					return &x402.VerifyResponse{IsValid: true}, nil
				},
			}
			adapter := NewX402AdapterWithFacilitator(facilitator, testVerificationTimeout)
			proof := paymentProofWithMutation(t, validRequirements(), test.mutate)

			_, err := adapter.Verify(t.Context(), proof, validRequirements())
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Verify() error = %v, want %v", err, test.wantError)
			}
			if calls.Load() != 0 {
				t.Fatalf("facilitator calls = %d, want 0", calls.Load())
			}
		})
	}
}

// TestX402PaymentIdentifierSurvivesJSONFormatting prevents replay by re-encoding.
func TestX402PaymentIdentifierSurvivesJSONFormatting(t *testing.T) {
	t.Parallel()

	facilitator := &fakeFacilitatorClient{
		verifyResponse: &x402.VerifyResponse{IsValid: true},
	}
	adapter := NewX402AdapterWithFacilitator(
		facilitator,
		testVerificationTimeout,
	)
	compactProof := validPaymentProof(t, validRequirements())
	compactJSON, err := base64.StdEncoding.DecodeString(compactProof)
	if err != nil {
		t.Fatal(err)
	}
	var payload x402types.PaymentPayload
	if err := json.Unmarshal(compactJSON, &payload); err != nil {
		t.Fatal(err)
	}
	indentedJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	indentedProof := base64.StdEncoding.EncodeToString(indentedJSON)

	compact, err := adapter.Verify(
		t.Context(),
		compactProof,
		validRequirements(),
	)
	if err != nil {
		t.Fatal(err)
	}
	indented, err := adapter.Verify(
		t.Context(),
		indentedProof,
		validRequirements(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if compact.PaymentIdentifier != indented.PaymentIdentifier {
		t.Fatalf(
			"payment identifiers = %q and %q",
			compact.PaymentIdentifier,
			indented.PaymentIdentifier,
		)
	}
}

// TestX402AdapterClassifiesVerificationFailures verifies retry decisions.
func TestX402AdapterClassifiesVerificationFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		verify        facilitatorVerifyFunc
		wantError     error
		wantRetryable bool
	}{
		{
			name: "rejected",
			verify: func(context.Context, []byte, []byte) (*x402.VerifyResponse, error) {
				return nil, x402.NewVerifyError(
					x402.ErrCodeSignatureInvalid,
					"",
					"invalid signature",
				)
			},
			wantError: ErrPaymentRejected,
		},
		{
			name: "expired authorization",
			verify: func(context.Context, []byte, []byte) (*x402.VerifyResponse, error) {
				return nil, x402.NewVerifyError(
					x402.ErrCodePaymentExpired,
					"",
					"authorization expired",
				)
			},
			wantError: ErrPaymentRejected,
		},
		{
			name: "nonce rejected",
			verify: func(context.Context, []byte, []byte) (*x402.VerifyResponse, error) {
				return nil, x402.NewVerifyError(
					x402.ErrCodeInvalidPayment,
					"",
					"nonce already used",
				)
			},
			wantError: ErrPaymentRejected,
		},
		{
			name: "invalid response",
			verify: func(context.Context, []byte, []byte) (*x402.VerifyResponse, error) {
				return &x402.VerifyResponse{IsValid: false}, nil
			},
			wantError: ErrPaymentRejected,
		},
		{
			name: "timeout",
			verify: func(ctx context.Context, _ []byte, _ []byte) (*x402.VerifyResponse, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
			wantError:     ErrPaymentTimeout,
			wantRetryable: true,
		},
		{
			name: "unavailable",
			verify: func(context.Context, []byte, []byte) (*x402.VerifyResponse, error) {
				return nil, errors.New("facilitator unavailable")
			},
			wantError:     ErrPaymentUnavailable,
			wantRetryable: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			adapter := NewX402AdapterWithFacilitator(
				&fakeFacilitatorClient{verify: test.verify},
				testVerificationTimeout,
			)
			_, err := adapter.Verify(
				t.Context(),
				validPaymentProof(t, validRequirements()),
				validRequirements(),
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Verify() error = %v, want %v", err, test.wantError)
			}
			if IsRetryable(err) != test.wantRetryable {
				t.Fatalf("IsRetryable() = %v", IsRetryable(err))
			}
		})
	}
}

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
		settlement.PaymentReference == "" ||
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
		{name: "missing resource", mutate: func(requirements *Requirements) {
			requirements.ResourceURL = ""
		}},
		{name: "missing MIME type", mutate: func(requirements *Requirements) {
			requirements.MIMEType = ""
		}},
		{name: "invalid timeout", mutate: func(requirements *Requirements) {
			requirements.MaxTimeoutSeconds = 0
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

type facilitatorVerifyFunc func(
	context.Context,
	[]byte,
	[]byte,
) (*x402.VerifyResponse, error)

type fakeFacilitatorClient struct {
	verifyResponse *x402.VerifyResponse
	settleResponse *x402.SettleResponse
	verify         facilitatorVerifyFunc
}

// Verify returns the configured facilitator verification result.
func (client *fakeFacilitatorClient) Verify(
	ctx context.Context,
	payload []byte,
	requirements []byte,
) (*x402.VerifyResponse, error) {
	if client.verify != nil {
		return client.verify(ctx, payload, requirements)
	}
	return client.verifyResponse, nil
}

// Settle returns the configured facilitator settlement result.
func (client *fakeFacilitatorClient) Settle(
	context.Context,
	[]byte,
	[]byte,
) (*x402.SettleResponse, error) {
	return client.settleResponse, nil
}

// GetSupported returns the one payment kind used by the test facilitator.
func (client *fakeFacilitatorClient) GetSupported(
	context.Context,
) (x402.SupportedResponse, error) {
	return x402.SupportedResponse{
		Kinds: []x402.SupportedKind{
			{
				X402Version: 2,
				Scheme:      ExactScheme,
				Network:     BaseSepoliaNetwork,
			},
		},
	}, nil
}

// validPaymentProof encodes an x402 v2 proof bound to the supplied requirements.
func validPaymentProof(t *testing.T, requirements Requirements) string {
	t.Helper()

	payload := x402types.PaymentPayload{
		X402Version: 2,
		Payload: map[string]interface{}{
			"authorization": map[string]interface{}{
				"nonce": "test-nonce",
			},
			"signature": "0xtest-signature",
		},
		Accepted: x402types.PaymentRequirements{
			Scheme:            requirements.Scheme,
			Network:           requirements.Network,
			Asset:             requirements.Asset,
			Amount:            requirements.Amount.String(),
			PayTo:             requirements.PayTo,
			MaxTimeoutSeconds: requirements.MaxTimeoutSeconds,
		},
		Resource: &x402types.ResourceInfo{
			URL:         requirements.ResourceURL,
			Description: requirements.Description,
			MimeType:    requirements.MIMEType,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(encoded)
}

func paymentProofWithMutation(
	t *testing.T,
	requirements Requirements,
	mutate func(*x402types.PaymentPayload),
) string {
	t.Helper()
	encoded, err := base64.StdEncoding.DecodeString(validPaymentProof(t, requirements))
	if err != nil {
		t.Fatal(err)
	}
	var payload x402types.PaymentPayload
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	mutate(&payload)
	encoded, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(encoded)
}
