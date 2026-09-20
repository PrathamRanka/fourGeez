package transactions

import (
	"context"
	"fmt"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
)

// TransactionStatus identifies a transaction lifecycle state.
type TransactionStatus string

// PaymentFinality identifies the strongest provider or network observation recorded.
type PaymentFinality string

// ReconciliationStage identifies the mutually exclusive amount bucket shown to sellers.
type ReconciliationStage string
type CommerceState string
type PaymentState string
type FulfillmentState string
type RefundState string
type RecoveryAction string
type RecoveryState string
type ActivityMode string
type SellerOutcome string

type PurchaseChannel string
type PaymentRail string

const (
	StatusProposed          TransactionStatus = "PROPOSED"
	StatusApprovalPending   TransactionStatus = "APPROVAL_PENDING"
	StatusApproved          TransactionStatus = "APPROVED"
	StatusPaymentRequired   TransactionStatus = "PAYMENT_REQUIRED"
	StatusPaymentVerified   TransactionStatus = "PAYMENT_VERIFIED"
	StatusForwarded         TransactionStatus = "FORWARDED"
	StatusFulfilled         TransactionStatus = "FULFILLED"
	StatusFailed            TransactionStatus = "FAILED"
	StatusDisputed          TransactionStatus = "DISPUTED"
	StatusRefundRecommended TransactionStatus = "REFUND_RECOMMENDED"
	StatusResolved          TransactionStatus = "RESOLVED"
)

const (
	CommerceStateAwaitingPayment       CommerceState    = "awaiting_payment"
	CommerceStatePaymentProcessing     CommerceState    = "payment_processing"
	CommerceStatePaid                  CommerceState    = "paid"
	CommerceStateFulfilling            CommerceState    = "fulfilling"
	CommerceStateFulfilled             CommerceState    = "fulfilled"
	CommerceStateFailed                CommerceState    = "failed"
	CommerceStateDisputed              CommerceState    = "disputed"
	CommerceStateRefundRecommended     CommerceState    = "refund_recommended"
	CommerceStateResolved              CommerceState    = "resolved"
	CommerceStateAbandoned             CommerceState    = "abandoned"
	PaymentStatePending                PaymentState     = "pending"
	PaymentStateConfirmed              PaymentState     = "confirmed"
	PaymentStateFinalized              PaymentState     = "finalized"
	PaymentStateFailed                 PaymentState     = "failed"
	PaymentStateExpired                PaymentState     = "expired"
	FulfillmentStateNotStarted         FulfillmentState = "not_started"
	FulfillmentStateInProgress         FulfillmentState = "in_progress"
	FulfillmentStateSucceeded          FulfillmentState = "succeeded"
	FulfillmentStateFailed             FulfillmentState = "failed"
	RefundStateNotRequested            RefundState      = "not_requested"
	RefundStateDisputed                RefundState      = "disputed"
	RefundStateRecommended             RefundState      = "recommended"
	RefundStateSellerReported          RefundState      = "seller_reported"
	RecoveryActionRetrySameRequest     RecoveryAction   = "retry_same_request"
	RecoveryActionRetrySamePayment     RecoveryAction   = "retry_same_payment"
	RecoveryActionAwaitReconciliation  RecoveryAction   = "await_reconciliation"
	RecoveryActionCreateNewIntent      RecoveryAction   = "create_new_intent"
	RecoveryActionOpenDispute          RecoveryAction   = "open_dispute"
	RecoveryActionAwaitResolution      RecoveryAction   = "await_resolution"
	RecoveryActionRecordExternalRefund RecoveryAction   = "record_external_refund"
	RecoveryActionRequestSellerReview  RecoveryAction   = "request_seller_review"
	RecoveryActionNone                 RecoveryAction   = "none"
	RecoveryStateNone                  RecoveryState    = "none"
	RecoveryStateRetryAvailable        RecoveryState    = "retry_available"
	RecoveryStateRetryInProgress       RecoveryState    = "retry_in_progress"
	RecoveryStateSellerReview          RecoveryState    = "seller_review"
	RecoveryStateDisputeAvailable      RecoveryState    = "dispute_available"
	RecoveryStateDisputeOpen           RecoveryState    = "dispute_open"
	RecoveryStateRefundRecommended     RecoveryState    = "refund_recommended"
	RecoveryStateRefundReported        RecoveryState    = "refund_reported"
	RecoveryStateResolved              RecoveryState    = "resolved"
)

const (
	ActivityModeTest ActivityMode = "test"
	ActivityModeLive ActivityMode = "live"

	SellerOutcomeAwaitingPayment   SellerOutcome = "awaiting_payment"
	SellerOutcomeAbandoned         SellerOutcome = "abandoned"
	SellerOutcomePaymentRejected   SellerOutcome = "payment_rejected"
	SellerOutcomePaymentProcessing SellerOutcome = "payment_processing"
	SellerOutcomeFulfilling        SellerOutcome = "fulfilling"
	SellerOutcomeFulfilled         SellerOutcome = "fulfilled"
	SellerOutcomeFulfillmentFailed SellerOutcome = "fulfillment_failed"
	SellerOutcomeDisputed          SellerOutcome = "disputed"
	SellerOutcomeRefundRecommended SellerOutcome = "refund_recommended"
	SellerOutcomeResolved          SellerOutcome = "resolved"
)

const (
	PurchaseChannelAgent   PurchaseChannel = "agent"
	PurchaseChannelBrowser PurchaseChannel = "browser"
	PaymentRailX402        PaymentRail     = "x402"
)

const (
	PaymentFinalityConfirmed PaymentFinality = "confirmed"
	PaymentFinalityFinalized PaymentFinality = "finalized"
	PaymentFinalityFailed    PaymentFinality = "failed"
)

const (
	ReconciliationStageChallenged ReconciliationStage = "challenged"
	ReconciliationStageVerified   ReconciliationStage = "verified"
	ReconciliationStageFinalized  ReconciliationStage = "finalized"
	ReconciliationStageFulfilled  ReconciliationStage = "fulfilled"
	ReconciliationStageFailed     ReconciliationStage = "failed"
	ReconciliationStageDisputed   ReconciliationStage = "disputed"
)

// TransactionParams contains the values required to create a transaction.
type TransactionParams struct {
	TransactionID        domain.ID
	IntentID             domain.ID
	SellerID             domain.ID
	RouteID              domain.ID
	BuyerID              string
	PurchaseSessionID    string
	ProductDisplayName   string
	ProductSlug          string
	PaymentDestinationID domain.ID
	PurchaseChannel      PurchaseChannel
	PaymentRail          PaymentRail
	ActivityMode         ActivityMode
	CheckoutExpiresAt    domain.Timestamp
	Amount               domain.Amount
	Asset                string
	Network              string
	CreatedAt            domain.Timestamp
}

// ResponseSummary contains allowlisted seller response metadata.
type ResponseSummary struct {
	ContentType   string `json:"contentType,omitempty"`
	ContentLength int64  `json:"contentLength"`
}

// Transaction records payment, forwarding, delivery, and dispute state.
type Transaction struct {
	transactionID           domain.ID
	intentID                domain.ID
	sellerID                domain.ID
	routeID                 domain.ID
	buyerID                 string
	purchaseSessionID       string
	productDisplayName      string
	productSlug             string
	paymentDestinationID    domain.ID
	purchaseChannel         PurchaseChannel
	paymentRail             PaymentRail
	activityMode            ActivityMode
	checkoutExpiresAt       domain.Timestamp
	amount                  domain.Amount
	asset                   string
	network                 string
	status                  TransactionStatus
	paymentIdentifier       string
	paymentProofHash        intents.SHA256Digest
	paymentReference        string
	paymentFinality         PaymentFinality
	reconciledAt            *domain.Timestamp
	upstreamStatus          *int
	responseHash            *intents.SHA256Digest
	responseSummary         *ResponseSummary
	failureCode             string
	recoveryRequestBodyHash intents.SHA256Digest
	fulfillmentAttempts     uint32
	retrySafe               bool
	createdAt               domain.Timestamp
	updatedAt               domain.Timestamp
	version                 uint64
}

// Response is the public transaction representation without payment secrets.
type Response struct {
	TransactionID        domain.ID                   `json:"transactionId"`
	IntentID             domain.ID                   `json:"intentId"`
	SellerID             domain.ID                   `json:"sellerId"`
	RouteID              domain.ID                   `json:"routeId"`
	BuyerID              string                      `json:"buyerId"`
	PurchaseSessionID    string                      `json:"purchaseSessionId,omitempty"`
	ProductDisplayName   string                      `json:"productDisplayName,omitempty"`
	ProductSlug          string                      `json:"productSlug,omitempty"`
	PaymentDestinationID domain.ID                   `json:"paymentDestinationId,omitempty"`
	PurchaseChannel      PurchaseChannel             `json:"purchaseChannel"`
	PaymentRail          PaymentRail                 `json:"paymentRail"`
	ActivityMode         ActivityMode                `json:"activityMode"`
	CheckoutExpiresAt    domain.Timestamp            `json:"checkoutExpiresAt"`
	SellerOutcome        SellerOutcome               `json:"sellerOutcome"`
	Status               TransactionStatus           `json:"status"`
	Amount               domain.Amount               `json:"amount"`
	Asset                string                      `json:"asset"`
	Network              string                      `json:"network"`
	PriceBreakdown       intents.ExactPriceBreakdown `json:"priceBreakdown"`
	CommerceLifecycle    CommerceLifecycleProjection `json:"commerceLifecycle"`
	PaymentFinality      PaymentFinality             `json:"paymentFinality,omitempty"`
	PaymentReference     string                      `json:"paymentReference,omitempty"`
	ReconciledAt         *domain.Timestamp           `json:"reconciledAt,omitempty"`
	Reconciliation       *Reconciliation             `json:"reconciliation,omitempty"`
	UpstreamStatus       *int                        `json:"upstreamStatus,omitempty"`
	ResponseHash         *intents.SHA256Digest       `json:"responseHash,omitempty"`
	FailureCode          string                      `json:"failureCode,omitempty"`
	FulfillmentAttempts  uint32                      `json:"fulfillmentAttempts"`
	CreatedAt            domain.Timestamp            `json:"createdAt"`
	UpdatedAt            domain.Timestamp            `json:"updatedAt"`
}

type CommerceLifecycleProjection struct {
	ExternalReference string           `json:"externalReference"`
	CommerceState     CommerceState    `json:"commerceState"`
	PaymentState      PaymentState     `json:"paymentState"`
	FulfillmentState  FulfillmentState `json:"fulfillmentState"`
	RefundState       RefundState      `json:"refundState"`
	RecoveryAction    RecoveryAction   `json:"recoveryAction"`
	RecoveryState     RecoveryState    `json:"recoveryState"`
}

// Reconciliation contains the amount and safe reference for one reporting stage.
type Reconciliation struct {
	Stage            ReconciliationStage `json:"stage"`
	Amount           domain.Amount       `json:"amount"`
	Asset            string              `json:"asset"`
	Network          string              `json:"network"`
	PaymentReference string              `json:"paymentReference,omitempty"`
	ReconciledAt     *domain.Timestamp   `json:"reconciledAt,omitempty"`
}

// EvidenceResponse contains a chain verification result and its events.
type EvidenceResponse struct {
	Valid  bool             `json:"valid"`
	Events []evidence.Event `json:"events"`
}

// DetailResponse combines one transaction with its evidence chain.
type DetailResponse struct {
	Transaction Response         `json:"transaction"`
	Evidence    EvidenceResponse `json:"evidence"`
}

// ListResponse is one cursor-paginated seller transaction page.
type ListResponse struct {
	Items      []Response `json:"items"`
	NextCursor *string    `json:"nextCursor,omitempty"`
}

// SellerTransactionQuery contains bounded seller transaction filters.
type SellerTransactionQuery struct {
	SellerID     domain.ID
	From         *domain.Timestamp
	To           *domain.Timestamp
	RouteID      *domain.ID
	Status       TransactionStatus
	ActivityMode ActivityMode
	Outcome      SellerOutcome
	Asset        string
	Network      string
	AsOf         domain.Timestamp
	Limit        int
	Cursor       string
}

// ReadRepository loads transaction read models and seller pages.
type ReadRepository interface {
	Get(ctx context.Context, transactionID domain.ID) (Transaction, error)
	ListBySeller(
		ctx context.Context,
		sellerID domain.ID,
		limit int,
		cursor string,
	) ([]Transaction, *string, error)
	QueryBySeller(
		ctx context.Context,
		query SellerTransactionQuery,
	) ([]Transaction, *string, error)
}

// EvidenceRepository loads the append-only chain for one transaction.
type EvidenceRepository interface {
	ListByTransaction(
		ctx context.Context,
		transactionID domain.ID,
	) ([]evidence.Event, error)
}

// SellerRepository loads seller ownership for tenant authorization.
type SellerRepository interface {
	GetSeller(ctx context.Context, sellerID domain.ID) (catalog.Seller, error)
}

// InvalidTransitionError reports an unsupported transaction state change.
type InvalidTransitionError struct {
	From TransactionStatus
	To   TransactionStatus
}

// Error describes the rejected transaction transition.
func (transitionError InvalidTransitionError) Error() string {
	return fmt.Sprintf(
		"transaction cannot transition from %s to %s",
		transitionError.From,
		transitionError.To,
	)
}

// TransactionID returns the transaction identifier.
func (transaction Transaction) TransactionID() domain.ID {
	return transaction.transactionID
}

// IntentID returns the bound purchase-intent identifier.
func (transaction Transaction) IntentID() domain.ID {
	return transaction.intentID
}

// SellerID returns the owning seller identifier.
func (transaction Transaction) SellerID() domain.ID {
	return transaction.sellerID
}

// RouteID returns the paid-route identifier.
func (transaction Transaction) RouteID() domain.ID {
	return transaction.routeID
}

// BuyerID returns the buyer identity.
func (transaction Transaction) BuyerID() string {
	return transaction.buyerID
}

func (transaction Transaction) PurchaseSessionID() string  { return transaction.purchaseSessionID }
func (transaction Transaction) ProductDisplayName() string { return transaction.productDisplayName }
func (transaction Transaction) ProductSlug() string        { return transaction.productSlug }
func (transaction Transaction) PaymentDestinationID() domain.ID {
	return transaction.paymentDestinationID
}
func (transaction Transaction) PurchaseChannel() PurchaseChannel { return transaction.purchaseChannel }
func (transaction Transaction) PaymentRail() PaymentRail         { return transaction.paymentRail }
func (transaction Transaction) ActivityMode() ActivityMode       { return transaction.activityMode }
func (transaction Transaction) CheckoutExpiresAt() domain.Timestamp {
	return transaction.checkoutExpiresAt
}

// Amount returns the exact transaction amount.
func (transaction Transaction) Amount() domain.Amount {
	return transaction.amount
}

// Asset returns the payment asset.
func (transaction Transaction) Asset() string {
	return transaction.asset
}

// Network returns the payment network.
func (transaction Transaction) Network() string {
	return transaction.network
}

// Status returns the current transaction state.
func (transaction Transaction) Status() TransactionStatus {
	return transaction.status
}

// PaymentIdentifier returns the replay-protection identifier.
func (transaction Transaction) PaymentIdentifier() string {
	return transaction.paymentIdentifier
}

// PaymentProofHash returns the non-sensitive proof digest.
func (transaction Transaction) PaymentProofHash() intents.SHA256Digest {
	return transaction.paymentProofHash
}

// PaymentReference returns the safe facilitator or network settlement reference.
func (transaction Transaction) PaymentReference() string {
	return transaction.paymentReference
}

// PaymentFinality returns the latest persisted payment observation.
func (transaction Transaction) PaymentFinality() PaymentFinality {
	return transaction.paymentFinality
}

// ReconciledAt returns a copy of the latest successful reconciliation timestamp.
func (transaction Transaction) ReconciledAt() *domain.Timestamp {
	if transaction.reconciledAt == nil {
		return nil
	}
	reconciledAt := *transaction.reconciledAt
	return &reconciledAt
}

// UpstreamStatus returns a copy of the seller HTTP status.
func (transaction Transaction) UpstreamStatus() *int {
	if transaction.upstreamStatus == nil {
		return nil
	}
	return intPointer(*transaction.upstreamStatus)
}

// ResponseHash returns a copy of the captured response digest.
func (transaction Transaction) ResponseHash() *intents.SHA256Digest {
	if transaction.responseHash == nil {
		return nil
	}
	digest := *transaction.responseHash
	return &digest
}

// ResponseSummary returns a copy of allowlisted response metadata.
func (transaction Transaction) ResponseSummary() *ResponseSummary {
	if transaction.responseSummary == nil {
		return nil
	}
	summary := *transaction.responseSummary
	return &summary
}

// FailureCode returns the stable transaction failure code.
func (transaction Transaction) FailureCode() string {
	return transaction.failureCode
}

func (transaction Transaction) RecoveryRequestBodyHash() intents.SHA256Digest {
	return transaction.recoveryRequestBodyHash
}

func (transaction Transaction) FulfillmentAttempts() uint32 { return transaction.fulfillmentAttempts }
func (transaction Transaction) RetrySafe() bool             { return transaction.retrySafe }

// CreatedAt returns the creation timestamp.
func (transaction Transaction) CreatedAt() domain.Timestamp {
	return transaction.createdAt
}

// UpdatedAt returns the latest state-change timestamp.
func (transaction Transaction) UpdatedAt() domain.Timestamp {
	return transaction.updatedAt
}

// Version returns the optimistic-concurrency version.
func (transaction Transaction) Version() uint64 {
	return transaction.version
}

func (transaction Transaction) PriceBreakdown() intents.ExactPriceBreakdown {
	return intents.ExactPriceBreakdown{
		Calculation: intents.PriceCalculationFixedSingleProduct,
		Quantity:    1,
		UnitAmount:  transaction.amount,
		Subtotal:    transaction.amount,
		Adjustments: domain.MustParseAmount("0"),
		Total:       transaction.amount,
		Asset:       transaction.asset,
		Network:     transaction.network,
	}
}

func (transaction Transaction) CommerceLifecycle(sellerReportedRefund bool) CommerceLifecycleProjection {
	return transaction.CommerceLifecycleAt(transaction.updatedAt, sellerReportedRefund)
}

func (transaction Transaction) CommerceLifecycleAt(at domain.Timestamp, sellerReportedRefund bool) CommerceLifecycleProjection {
	projection := CommerceLifecycleProjection{
		ExternalReference: transaction.transactionID.String(),
		CommerceState:     CommerceStateAwaitingPayment,
		PaymentState:      PaymentStatePending,
		FulfillmentState:  FulfillmentStateNotStarted,
		RefundState:       RefundStateNotRequested,
		RecoveryAction:    RecoveryActionRetrySameRequest,
		RecoveryState:     RecoveryStateNone,
	}
	if transaction.status == StatusPaymentRequired &&
		transaction.paymentFinality == "" &&
		!at.Time().Before(transaction.checkoutExpiresAt.Time()) {
		projection.CommerceState = CommerceStateAbandoned
		projection.PaymentState = PaymentStateExpired
		projection.RecoveryAction = RecoveryActionCreateNewIntent
		return projection
	}
	if transaction.paymentFinality == PaymentFinalityConfirmed {
		projection.CommerceState = CommerceStatePaymentProcessing
		projection.PaymentState = PaymentStateConfirmed
		projection.RecoveryAction = RecoveryActionAwaitReconciliation
	} else if transaction.paymentFinality == PaymentFinalityFinalized {
		projection.CommerceState = CommerceStatePaid
		projection.PaymentState = PaymentStateFinalized
		if transaction.recoveryRequestBodyHash.String() != "" {
			projection.RecoveryState = RecoveryStateRetryInProgress
		}
	} else if transaction.paymentFinality == PaymentFinalityFailed {
		projection.CommerceState = CommerceStateFailed
		projection.PaymentState = PaymentStateFailed
		projection.RecoveryAction = RecoveryActionCreateNewIntent
	}
	if transaction.failureCode != "" && transaction.status != StatusPaymentVerified {
		projection.FulfillmentState = FulfillmentStateFailed
	} else if transaction.upstreamStatus != nil && *transaction.upstreamStatus >= 200 && *transaction.upstreamStatus <= 299 {
		projection.FulfillmentState = FulfillmentStateSucceeded
	}
	switch transaction.status {
	case StatusForwarded:
		projection.CommerceState = CommerceStateFulfilling
		projection.FulfillmentState = FulfillmentStateInProgress
		if transaction.fulfillmentAttempts > 1 {
			projection.RecoveryState = RecoveryStateRetryInProgress
		}
	case StatusFulfilled:
		projection.CommerceState = CommerceStateFulfilled
		projection.FulfillmentState = FulfillmentStateSucceeded
		projection.RecoveryAction = RecoveryActionNone
		if transaction.fulfillmentAttempts > 1 {
			projection.RecoveryState = RecoveryStateResolved
		}
	case StatusFailed:
		projection.CommerceState = CommerceStateFailed
		projection.FulfillmentState = FulfillmentStateFailed
		if transaction.paymentFinality == PaymentFinalityFinalized {
			switch {
			case transaction.CanRetryFulfillment():
				projection.RecoveryState = RecoveryStateRetryAvailable
				projection.RecoveryAction = RecoveryActionRetrySamePayment
			case transaction.failureCode == "upstream_timeout" || transaction.failureCode == "upstream_unavailable":
				projection.RecoveryState = RecoveryStateSellerReview
				projection.RecoveryAction = RecoveryActionRequestSellerReview
			default:
				projection.RecoveryState = RecoveryStateDisputeAvailable
				projection.RecoveryAction = RecoveryActionOpenDispute
			}
		}
	case StatusDisputed:
		projection.CommerceState = CommerceStateDisputed
		projection.RefundState = RefundStateDisputed
		projection.RecoveryAction = RecoveryActionAwaitResolution
		projection.RecoveryState = RecoveryStateDisputeOpen
	case StatusRefundRecommended:
		projection.CommerceState = CommerceStateRefundRecommended
		projection.RefundState = RefundStateRecommended
		projection.RecoveryAction = RecoveryActionRecordExternalRefund
		projection.RecoveryState = RecoveryStateRefundRecommended
	case StatusResolved:
		projection.CommerceState = CommerceStateResolved
		projection.RefundState = RefundStateDisputed
		projection.RecoveryAction = RecoveryActionNone
		projection.RecoveryState = RecoveryStateResolved
	}
	if sellerReportedRefund {
		projection.RefundState = RefundStateSellerReported
		projection.RecoveryAction = RecoveryActionNone
		projection.RecoveryState = RecoveryStateRefundReported
	}
	return projection
}

func (transaction Transaction) CanRetryFulfillment() bool {
	return transaction.status == StatusFailed &&
		transaction.paymentFinality == PaymentFinalityFinalized &&
		transaction.retrySafe && transaction.fulfillmentAttempts == 1
}

func (transaction Transaction) SellerOutcomeAt(at domain.Timestamp) SellerOutcome {
	projection := transaction.CommerceLifecycleAt(at, false)
	switch projection.CommerceState {
	case CommerceStateAbandoned:
		return SellerOutcomeAbandoned
	case CommerceStatePaymentProcessing, CommerceStatePaid:
		return SellerOutcomePaymentProcessing
	case CommerceStateFulfilling:
		return SellerOutcomeFulfilling
	case CommerceStateFulfilled:
		return SellerOutcomeFulfilled
	case CommerceStateDisputed:
		return SellerOutcomeDisputed
	case CommerceStateRefundRecommended:
		return SellerOutcomeRefundRecommended
	case CommerceStateResolved:
		return SellerOutcomeResolved
	case CommerceStateFailed:
		if projection.PaymentState == PaymentStateFailed {
			return SellerOutcomePaymentRejected
		}
		return SellerOutcomeFulfillmentFailed
	default:
		return SellerOutcomeAwaitingPayment
	}
}
