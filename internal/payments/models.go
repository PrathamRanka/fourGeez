package payments

import (
	"context"
	"errors"
	"time"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/transactions"
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
	// MockPayerAddress is the deterministic local browser-wallet identity.
	MockPayerAddress = "0x2222222222222222222222222222222222222222"
)

var (
	// ErrPaymentRejected reports a proof that cannot authorize payment.
	ErrPaymentRejected = errors.New("payment rejected")
	// ErrPaymentTimeout reports that payment verification exceeded its deadline.
	ErrPaymentTimeout = errors.New("payment verification timed out")
	// ErrPaymentUnavailable reports that the payment provider is unavailable.
	ErrPaymentUnavailable = errors.New("payment provider unavailable")
	// ErrPaidRouteMismatch reports a paid request outside its frozen intent.
	ErrPaidRouteMismatch = errors.New("paid route does not match purchase intent")
	// ErrIntentExpired reports a purchase intent that can no longer execute.
	ErrIntentExpired = errors.New("purchase intent has expired")
	// ErrApprovalRequired reports a missing approval for a protected intent.
	ErrApprovalRequired = errors.New("purchase approval is required")
	// ErrApprovalInvalid reports an invalid or stale approval token.
	ErrApprovalInvalid = errors.New("purchase approval is invalid")
	// ErrPaymentReplay reports a proof or intent already claimed elsewhere.
	ErrPaymentReplay = errors.New("payment proof has already been used")
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
	PayerAddress      string
}

// SettlementResult contains the safe result of settling a verified payment.
type SettlementResult struct {
	Settled           bool
	PaymentIdentifier string
	PaymentReference  string
	ResponseHeader    string
	PayerAddress      string
}

// Adapter isolates payment protocol implementations from AgentPay use cases.
type Adapter interface {
	CreateChallenge(context.Context, Requirements) (Challenge, error)
	Verify(context.Context, string, Requirements) (VerificationResult, error)
	Settle(context.Context, string, Requirements) (SettlementResult, error)
}

// CommerceOperation identifies a transaction-critical entitlement boundary.
type CommerceOperation string

const (
	CommerceOperationChallenge    CommerceOperation = "challenge"
	CommerceOperationVerification CommerceOperation = "verification"
	CommerceOperationSettlement   CommerceOperation = "settlement"
	// CommerceOperationExecution is intentionally not called after finality:
	// a payment finalized before ordinary cancellation remains fulfillable.
	CommerceOperationExecution CommerceOperation = "execution"
)

// CommerceAuthorizer performs a fresh authoritative seller-entitlement check.
type CommerceAuthorizer interface {
	AuthorizeCommerce(context.Context, domain.ID, CommerceOperation) error
}

// BrowserPurchaseLifecycle binds the one browser transaction and extends its
// read/remediation authority only after payment finality.
type BrowserPurchaseLifecycle interface {
	ClaimTransaction(context.Context, browserpurchase.PurchaseSessionID, domain.ID) (browserpurchase.BrowserPurchaseSession, error)
	Complete(context.Context, browserpurchase.PurchaseSessionID, string, string, domain.ID, time.Time) error
}

// PaidRouteRequest identifies the frozen purchase requested by an agent.
type PaidRouteRequest struct {
	Slug          string
	Method        catalog.RouteMethod
	ProxyPath     string
	IntentID      domain.ID
	BuyerID       string
	ApprovalToken string
}

// ResolvedPaidRoute contains trusted values required for payment and forwarding.
type ResolvedPaidRoute struct {
	Seller         catalog.Seller
	Route          catalog.PaidRoute
	PurchaseIntent intents.PurchaseIntent
	Requirements   Requirements
}

// CheckoutRequest contains one authenticated paid-route attempt.
type CheckoutRequest struct {
	PaidRouteRequest
	PaymentProof string
	Body         []byte
	ContentType  string
}

// CheckoutResult contains either a challenge or a delivered seller response.
type CheckoutResult struct {
	TransactionID    domain.ID
	Challenge        *Challenge
	Response         *proxy.ForwardResponse
	SettlementHeader string
}

// PaidRouteCatalogRepository resolves sellers and their configured routes.
type PaidRouteCatalogRepository interface {
	ResolveSellerBySlug(context.Context, string) (catalog.Seller, error)
	GetRoute(context.Context, domain.ID) (catalog.PaidRoute, error)
}

// PaidRouteAuthorizer reloads the current product and verified destination.
type PaidRouteAuthorizer interface {
	AuthorizePaidRoute(context.Context, domain.ID) (catalog.Seller, catalog.PaidRoute, settlement.PaymentDestination, error)
}

// PaidRouteIntentRepository loads immutable purchase intents.
type PaidRouteIntentRepository interface {
	Get(context.Context, domain.ID) (intents.PurchaseIntent, error)
}

// PaidRouteApprovalRepository loads the session named by an approval token.
type PaidRouteApprovalRepository interface {
	Get(context.Context, domain.ID) (approvals.Session, error)
}

// CheckoutTransactionRepository persists transaction lifecycle mutations.
type CheckoutTransactionRepository interface {
	Create(context.Context, transactions.Transaction) error
	Get(context.Context, domain.ID) (transactions.Transaction, error)
	Update(context.Context, transactions.Transaction, uint64) error
}

// PaymentEvidenceRecorder records safe payment lifecycle facts.
type PaymentEvidenceRecorder interface {
	RecordPaymentChallenge(
		context.Context,
		domain.ID,
		evidence.PaymentChallengeFacts,
	) error
	RecordPaymentVerification(
		context.Context,
		domain.ID,
		evidence.PaymentVerificationFacts,
	) error
}

// PaidExecutor performs the already-verified seller invocation.
type PaidExecutor interface {
	Execute(context.Context, proxy.ExecutionRequest) (proxy.ForwardResponse, error)
}
