package payments

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/gowebpki/jcs"
	x402 "github.com/x402-foundation/x402/go"
	x402http "github.com/x402-foundation/x402/go/http"
	x402types "github.com/x402-foundation/x402/go/types"
)

const (
	mockPaymentIdentifierPrefix = "mock_"
	x402PaymentIdentifierPrefix = "x402_"
	defaultFacilitatorTimeout   = 8 * time.Second
)

// MockAdapter provides deterministic payment behavior for tests and local use.
type MockAdapter struct{}

// X402Adapter creates and verifies protocol-compliant x402 v2 payments.
type X402Adapter struct {
	facilitator x402.FacilitatorClient
	timeout     time.Duration
}

// PaidRouteService resolves paid requests against immutable purchase state.
type PaidRouteService struct {
	catalogRepository  PaidRouteCatalogRepository
	intentRepository   PaidRouteIntentRepository
	approvalRepository PaidRouteApprovalRepository
	approvalSigner     *approvals.ApprovalTokenSigner
	clock              domain.Clock
	publicBaseURL      string
}

var _ Adapter = (*X402Adapter)(nil)

// NewMockAdapter creates a deterministic payment adapter.
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{}
}

// NewX402Adapter creates the official x402-backed payment adapter.
func NewX402Adapter() *X402Adapter {
	return NewX402AdapterWithFacilitator(
		x402http.NewHTTPFacilitatorClient(
			&x402http.FacilitatorConfig{
				URL:     x402http.DefaultFacilitatorURL,
				Timeout: defaultFacilitatorTimeout,
			},
		),
		defaultFacilitatorTimeout,
	)
}

// NewX402AdapterWithFacilitator creates an adapter with an explicit boundary.
func NewX402AdapterWithFacilitator(
	facilitator x402.FacilitatorClient,
	timeout time.Duration,
) *X402Adapter {
	if timeout <= 0 {
		timeout = defaultFacilitatorTimeout
	}
	return &X402Adapter{
		facilitator: facilitator,
		timeout:     timeout,
	}
}

// NewPaidRouteService creates the paid-route resolution service.
func NewPaidRouteService(
	catalogRepository PaidRouteCatalogRepository,
	intentRepository PaidRouteIntentRepository,
	approvalRepository PaidRouteApprovalRepository,
	approvalSigner *approvals.ApprovalTokenSigner,
	clock domain.Clock,
	publicBaseURL string,
) *PaidRouteService {
	return &PaidRouteService{
		catalogRepository:  catalogRepository,
		intentRepository:   intentRepository,
		approvalRepository: approvalRepository,
		approvalSigner:     approvalSigner,
		clock:              clock,
		publicBaseURL:      strings.TrimRight(publicBaseURL, "/"),
	}
}

// Resolve validates route identity, intent expiry, and approval binding.
func (service *PaidRouteService) Resolve(
	ctx context.Context,
	request PaidRouteRequest,
) (ResolvedPaidRoute, error) {
	purchaseIntent, err := service.intentRepository.Get(ctx, request.IntentID)
	if err != nil {
		return ResolvedPaidRoute{}, err
	}
	if !service.clock.Now().Before(purchaseIntent.ExpiresAt().Time()) {
		return ResolvedPaidRoute{}, ErrIntentExpired
	}
	seller, err := service.catalogRepository.ResolveSellerBySlug(
		ctx,
		request.Slug,
	)
	if err != nil {
		return ResolvedPaidRoute{}, ErrPaidRouteMismatch
	}
	route, err := service.catalogRepository.GetRoute(
		ctx,
		purchaseIntent.RouteID(),
	)
	if err != nil {
		return ResolvedPaidRoute{}, err
	}
	if !paidRouteMatches(request, seller, route, purchaseIntent) {
		return ResolvedPaidRoute{}, ErrPaidRouteMismatch
	}
	if purchaseIntent.RequiresApproval() {
		if strings.TrimSpace(request.ApprovalToken) == "" {
			return ResolvedPaidRoute{}, ErrApprovalRequired
		}
		if err := service.verifyApproval(
			ctx,
			request.ApprovalToken,
			purchaseIntent,
		); err != nil {
			return ResolvedPaidRoute{}, err
		}
	}

	return ResolvedPaidRoute{
		Seller:         seller,
		Route:          route,
		PurchaseIntent: purchaseIntent,
		Requirements: Requirements{
			Scheme:  ExactScheme,
			Network: purchaseIntent.Network(),
			Asset:   purchaseIntent.Asset(),
			Amount:  purchaseIntent.Amount(),
			PayTo:   route.PayTo,
			ResourceURL: service.publicBaseURL +
				"/pay/" + seller.Slug + route.PathPattern,
			Description:       route.Description,
			MIMEType:          route.MIMEType,
			MaxTimeoutSeconds: route.UpstreamTimeoutSeconds,
		},
	}, nil
}

// verifyApproval validates the token and its persisted approved session.
func (service *PaidRouteService) verifyApproval(
	ctx context.Context,
	token string,
	purchaseIntent intents.PurchaseIntent,
) error {
	if service.approvalSigner == nil || service.approvalRepository == nil {
		return ErrApprovalInvalid
	}
	claims, err := service.approvalSigner.VerifyForIntent(
		token,
		purchaseIntent.IntentID(),
		purchaseIntent.IntentHash(),
		domain.NewTimestamp(service.clock.Now()),
	)
	if err != nil {
		return ErrApprovalInvalid
	}
	session, err := service.approvalRepository.Get(ctx, claims.SessionID)
	if err != nil {
		return ErrApprovalInvalid
	}
	if session.Status() != approvals.SessionStatusApproved ||
		session.IntentID() != purchaseIntent.IntentID() ||
		session.IntentHash() != purchaseIntent.IntentHash() ||
		!session.MatchesApprovalToken(token) {
		return ErrApprovalInvalid
	}
	return nil
}

// paidRouteMatches checks every route dimension frozen into the intent.
func paidRouteMatches(
	request PaidRouteRequest,
	seller catalog.Seller,
	route catalog.PaidRoute,
	purchaseIntent intents.PurchaseIntent,
) bool {
	return seller.Status == catalog.SellerStatusActive &&
		route.Enabled &&
		seller.SellerID == route.SellerID &&
		seller.SellerID == purchaseIntent.SellerID() &&
		route.RouteID == purchaseIntent.RouteID() &&
		request.BuyerID == purchaseIntent.BuyerID() &&
		request.Method == route.Method &&
		string(request.Method) == string(purchaseIntent.RequestMethod()) &&
		request.ProxyPath == route.PathPattern &&
		request.ProxyPath == purchaseIntent.RequestPath()
}

// CreateChallenge creates a base64-encoded x402 v2 payment requirement.
func (adapter *X402Adapter) CreateChallenge(
	ctx context.Context,
	requirements Requirements,
) (Challenge, error) {
	if err := ctx.Err(); err != nil {
		return Challenge{}, err
	}
	if err := validateRequirements(requirements); err != nil {
		return Challenge{}, err
	}

	resourceServer := x402.Newx402ResourceServer()
	paymentRequired := resourceServer.CreatePaymentRequiredResponse(
		[]x402types.PaymentRequirements{
			{
				Scheme:            requirements.Scheme,
				Network:           requirements.Network,
				Asset:             requirements.Asset,
				Amount:            requirements.Amount.String(),
				PayTo:             requirements.PayTo,
				MaxTimeoutSeconds: requirements.MaxTimeoutSeconds,
			},
		},
		&x402types.ResourceInfo{
			URL:         requirements.ResourceURL,
			Description: requirements.Description,
			MimeType:    requirements.MIMEType,
		},
		"",
		nil,
	)
	encoded, err := json.Marshal(paymentRequired)
	if err != nil {
		return Challenge{}, err
	}

	return Challenge{
		Requirements: requirements,
		Header:       base64.StdEncoding.EncodeToString(encoded),
	}, nil
}

// Verify validates proof binding before calling the remote facilitator.
func (adapter *X402Adapter) Verify(
	ctx context.Context,
	proof string,
	requirements Requirements,
) (VerificationResult, error) {
	if err := validateRequirements(requirements); err != nil {
		return VerificationResult{}, err
	}
	payloadBytes, paymentIdentifier, err := parsePaymentProof(
		proof,
		requirements,
	)
	if err != nil {
		return VerificationResult{}, err
	}
	if adapter.facilitator == nil {
		return VerificationResult{}, ErrPaymentUnavailable
	}
	requirementsBytes, err := marshalX402Requirements(requirements)
	if err != nil {
		return VerificationResult{}, err
	}

	verificationContext, cancel := context.WithTimeout(ctx, adapter.timeout)
	defer cancel()
	response, err := adapter.facilitator.Verify(
		verificationContext,
		payloadBytes,
		requirementsBytes,
	)
	if err != nil {
		return VerificationResult{}, classifyFacilitatorError(ctx, err)
	}
	if response == nil || !response.IsValid {
		return VerificationResult{}, ErrPaymentRejected
	}

	return VerificationResult{
		Valid:             true,
		PaymentIdentifier: paymentIdentifier,
	}, nil
}

// Settle submits an exact verified proof and encodes the x402 response header.
func (adapter *X402Adapter) Settle(
	ctx context.Context,
	proof string,
	requirements Requirements,
) (SettlementResult, error) {
	if err := validateRequirements(requirements); err != nil {
		return SettlementResult{}, err
	}
	payloadBytes, paymentIdentifier, err := parsePaymentProof(
		proof,
		requirements,
	)
	if err != nil {
		return SettlementResult{}, err
	}
	if adapter.facilitator == nil {
		return SettlementResult{}, ErrPaymentUnavailable
	}
	requirementsBytes, err := marshalX402Requirements(requirements)
	if err != nil {
		return SettlementResult{}, err
	}

	settlementContext, cancel := context.WithTimeout(ctx, adapter.timeout)
	defer cancel()
	response, err := adapter.facilitator.Settle(
		settlementContext,
		payloadBytes,
		requirementsBytes,
	)
	if err != nil {
		return SettlementResult{}, classifyFacilitatorError(ctx, err)
	}
	if response == nil || !response.Success || response.Transaction == "" {
		return SettlementResult{}, ErrPaymentRejected
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return SettlementResult{}, err
	}

	return SettlementResult{
		Settled:           true,
		PaymentIdentifier: paymentIdentifier,
		ResponseHeader:    base64.StdEncoding.EncodeToString(encoded),
	}, nil
}

// IsRetryable reports whether a payment failure can be safely retried.
func IsRetryable(err error) bool {
	return errors.Is(err, ErrPaymentTimeout) ||
		errors.Is(err, ErrPaymentUnavailable)
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

// parsePaymentProof decodes and binds a v2 proof to the frozen exact quote.
func parsePaymentProof(
	proof string,
	requirements Requirements,
) ([]byte, string, error) {
	if strings.TrimSpace(proof) == "" {
		return nil, "", ErrPaymentRejected
	}
	payloadBytes, err := base64.StdEncoding.DecodeString(proof)
	if err != nil {
		return nil, "", ErrPaymentRejected
	}
	payload, err := x402types.ToPaymentPayload(payloadBytes)
	if err != nil || payload.X402Version != 2 {
		return nil, "", ErrPaymentRejected
	}
	if !matchesRequirements(payload.Accepted, requirements) {
		return nil, "", ErrPaymentRejected
	}
	canonicalPayload, err := jcs.Transform(payloadBytes)
	if err != nil {
		return nil, "", ErrPaymentRejected
	}

	digest := sha256.Sum256(canonicalPayload)
	paymentIdentifier := x402PaymentIdentifierPrefix +
		hex.EncodeToString(digest[:])
	return payloadBytes, paymentIdentifier, nil
}

// matchesRequirements enforces exact amount and immutable payment terms.
func matchesRequirements(
	accepted x402types.PaymentRequirements,
	requirements Requirements,
) bool {
	return accepted.Scheme == requirements.Scheme &&
		accepted.Network == requirements.Network &&
		accepted.Asset == requirements.Asset &&
		accepted.Amount == requirements.Amount.String() &&
		accepted.PayTo == requirements.PayTo &&
		accepted.MaxTimeoutSeconds == requirements.MaxTimeoutSeconds
}

// marshalX402Requirements serializes the frozen quote for the facilitator.
func marshalX402Requirements(requirements Requirements) ([]byte, error) {
	return json.Marshal(x402types.PaymentRequirements{
		Scheme:            requirements.Scheme,
		Network:           requirements.Network,
		Asset:             requirements.Asset,
		Amount:            requirements.Amount.String(),
		PayTo:             requirements.PayTo,
		MaxTimeoutSeconds: requirements.MaxTimeoutSeconds,
	})
}

// classifyFacilitatorError separates retryable transport failures from rejection.
func classifyFacilitatorError(parent context.Context, err error) error {
	if errors.Is(parent.Err(), context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrPaymentTimeout
	}
	var verifyError *x402.VerifyError
	if errors.As(err, &verifyError) {
		return ErrPaymentRejected
	}
	var settleError *x402.SettleError
	if errors.As(err, &settleError) {
		return ErrPaymentRejected
	}
	return ErrPaymentUnavailable
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
