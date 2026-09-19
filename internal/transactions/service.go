package transactions

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
)

const (
	defaultTransactionPageLimit   = 25
	maximumTransactionPageLimit   = 100
	maximumPaymentReferenceLength = 512
)

// ErrSellerAccess reports a seller transaction request outside its tenancy.
var ErrSellerAccess = errors.New("seller transactions were not found")

// Service coordinates transaction and evidence read operations.
type Service struct {
	repository         ReadRepository
	evidenceRepository EvidenceRepository
	evidenceVerifier   evidence.Signer
	sellerRepository   SellerRepository
}

// NewService creates the transaction read service.
func NewService(
	repository ReadRepository,
	evidenceRepository EvidenceRepository,
	evidenceVerifier evidence.Signer,
	sellerRepository SellerRepository,
) *Service {
	return &Service{
		repository:         repository,
		evidenceRepository: evidenceRepository,
		evidenceVerifier:   evidenceVerifier,
		sellerRepository:   sellerRepository,
	}
}

// Get returns a public transaction and its evidence verification result.
func (service *Service) Get(
	ctx context.Context,
	transactionID domain.ID,
) (DetailResponse, error) {
	transaction, err := service.repository.Get(ctx, transactionID)
	if err != nil {
		return DetailResponse{}, err
	}
	return service.detail(ctx, transaction)
}

// GetForSeller returns a transaction only to its owning seller subject.
func (service *Service) GetForSeller(
	ctx context.Context,
	transactionID domain.ID,
	ownerSubject string,
) (DetailResponse, error) {
	transaction, err := service.repository.Get(ctx, transactionID)
	if err != nil {
		return DetailResponse{}, err
	}
	if service.sellerRepository == nil {
		return DetailResponse{}, ErrSellerAccess
	}
	seller, err := service.sellerRepository.GetSeller(
		ctx,
		transaction.SellerID(),
	)
	if err != nil || seller.OwnerSubject != ownerSubject {
		return DetailResponse{}, ErrSellerAccess
	}
	return service.detail(ctx, transaction)
}

// detail loads and verifies the evidence chain for a transaction.
func (service *Service) detail(
	ctx context.Context,
	transaction Transaction,
) (DetailResponse, error) {
	events, err := service.evidenceRepository.ListByTransaction(
		ctx,
		transaction.TransactionID(),
	)
	if err != nil {
		return DetailResponse{}, err
	}
	valid := true
	if err := evidence.VerifyChain(ctx, events, service.evidenceVerifier); err != nil {
		if !errors.Is(err, evidence.ErrChainInvalid) {
			return DetailResponse{}, err
		}
		valid = false
	}
	return DetailResponse{
		Transaction: transactionResponse(transaction),
		Evidence: EvidenceResponse{
			Valid:  valid,
			Events: events,
		},
	}, nil
}

// ListSeller returns one authorized page of seller transactions.
func (service *Service) ListSeller(
	ctx context.Context,
	sellerID domain.ID,
	ownerSubject string,
	limit int,
	cursor string,
) (ListResponse, error) {
	return service.ListSellerFiltered(
		ctx,
		ownerSubject,
		SellerTransactionQuery{
			SellerID: sellerID,
			Limit:    limit,
			Cursor:   cursor,
		},
	)
}

// ListSellerFiltered returns one authorized page matching bounded filters.
func (service *Service) ListSellerFiltered(
	ctx context.Context,
	ownerSubject string,
	query SellerTransactionQuery,
) (ListResponse, error) {
	if service.sellerRepository == nil {
		return ListResponse{}, ErrSellerAccess
	}
	seller, err := service.sellerRepository.GetSeller(ctx, query.SellerID)
	if err != nil || seller.OwnerSubject != ownerSubject {
		return ListResponse{}, ErrSellerAccess
	}
	if query.Limit == 0 {
		query.Limit = defaultTransactionPageLimit
	}
	if query.Limit < 1 || query.Limit > maximumTransactionPageLimit {
		return ListResponse{}, domain.NewValidationError(
			"limit",
			"range",
			"must be between 1 and 100",
		)
	}
	transactionPage, nextCursor, err := service.repository.QueryBySeller(
		ctx,
		query,
	)
	if err != nil {
		return ListResponse{}, err
	}
	items := make([]Response, len(transactionPage))
	for index, transaction := range transactionPage {
		items[index] = transactionResponse(transaction)
	}
	return ListResponse{Items: items, NextCursor: nextCursor}, nil
}

// transactionResponse removes payment and internal persistence fields.
func transactionResponse(transaction Transaction) Response {
	reconciliation := transaction.Reconciliation()
	var publicReconciliation *Reconciliation
	if reconciliation.Stage != "" {
		publicReconciliation = &reconciliation
	}
	return Response{
		TransactionID:        transaction.TransactionID(),
		IntentID:             transaction.IntentID(),
		SellerID:             transaction.SellerID(),
		RouteID:              transaction.RouteID(),
		BuyerID:              transaction.BuyerID(),
		PurchaseSessionID:    transaction.PurchaseSessionID(),
		ProductDisplayName:   transaction.ProductDisplayName(),
		ProductSlug:          transaction.ProductSlug(),
		PaymentDestinationID: transaction.PaymentDestinationID(),
		PurchaseChannel:      transaction.PurchaseChannel(),
		PaymentRail:          transaction.PaymentRail(),
		Status:               transaction.Status(),
		Amount:               transaction.Amount(),
		Asset:                transaction.Asset(),
		Network:              transaction.Network(),
		PriceBreakdown:       transaction.PriceBreakdown(),
		CommerceLifecycle:    transaction.CommerceLifecycle(false),
		PaymentFinality:      transaction.PaymentFinality(),
		PaymentReference:     transaction.PaymentReference(),
		ReconciledAt:         transaction.ReconciledAt(),
		Reconciliation:       publicReconciliation,
		UpstreamStatus:       transaction.UpstreamStatus(),
		ResponseHash:         transaction.ResponseHash(),
		FailureCode:          transaction.FailureCode(),
		CreatedAt:            transaction.CreatedAt(),
		UpdatedAt:            transaction.UpdatedAt(),
	}
}

// NewTransaction validates and creates a proposed transaction.
func NewTransaction(params TransactionParams) (Transaction, error) {
	validationErrors := validateTransactionParams(params)
	if len(validationErrors) > 0 {
		return Transaction{}, validationErrors
	}

	purchaseChannel := params.PurchaseChannel
	if purchaseChannel == "" {
		purchaseChannel = PurchaseChannelAgent
	}
	paymentRail := params.PaymentRail
	if paymentRail == "" {
		paymentRail = PaymentRailX402
	}
	return Transaction{
		transactionID:        params.TransactionID,
		intentID:             params.IntentID,
		sellerID:             params.SellerID,
		routeID:              params.RouteID,
		buyerID:              strings.TrimSpace(params.BuyerID),
		purchaseSessionID:    strings.TrimSpace(params.PurchaseSessionID),
		productDisplayName:   strings.TrimSpace(params.ProductDisplayName),
		productSlug:          strings.TrimSpace(params.ProductSlug),
		paymentDestinationID: params.PaymentDestinationID,
		purchaseChannel:      purchaseChannel,
		paymentRail:          paymentRail,
		amount:               params.Amount,
		asset:                strings.TrimSpace(params.Asset),
		network:              strings.TrimSpace(params.Network),
		status:               StatusProposed,
		createdAt:            params.CreatedAt,
		updatedAt:            params.CreatedAt,
		version:              1,
	}, nil
}

// allowedTransition reports whether a state transition belongs to the lifecycle.
func allowedTransition(current, next TransactionStatus) bool {
	switch current {
	case StatusProposed:
		return next == StatusApprovalPending || next == StatusPaymentRequired
	case StatusApprovalPending:
		return next == StatusApproved
	case StatusApproved:
		return next == StatusPaymentRequired
	case StatusPaymentRequired:
		return next == StatusPaymentVerified
	case StatusPaymentVerified:
		return next == StatusForwarded || next == StatusFailed
	case StatusForwarded:
		return next == StatusFulfilled || next == StatusFailed
	case StatusFulfilled, StatusFailed:
		return next == StatusDisputed
	case StatusDisputed:
		return next == StatusRefundRecommended || next == StatusResolved
	case StatusRefundRecommended:
		return next == StatusResolved
	default:
		return false
	}
}

// validateTransactionParams validates transaction creation fields.
func validateTransactionParams(params TransactionParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	validateID := func(identifier domain.ID, prefix domain.IDPrefix, field string) {
		if _, err := domain.ParseID(identifier.String(), prefix); err != nil {
			validationErrors = append(
				validationErrors,
				domain.NewValidationError(field, "format", "has an invalid domain identifier"),
			)
		}
	}

	validateID(params.TransactionID, domain.TransactionIDPrefix, "transactionId")
	validateID(params.IntentID, domain.IntentIDPrefix, "intentId")
	validateID(params.SellerID, domain.SellerIDPrefix, "sellerId")
	validateID(params.RouteID, domain.RouteIDPrefix, "routeId")
	if strings.TrimSpace(params.BuyerID) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("buyerId", "required", "is required"))
	}
	if params.PurchaseChannel != "" && params.PurchaseChannel != PurchaseChannelAgent && params.PurchaseChannel != PurchaseChannelBrowser {
		validationErrors = append(validationErrors, domain.NewValidationError("purchaseChannel", "supported", "must use a supported purchase channel"))
	}
	if params.PaymentRail != "" && params.PaymentRail != PaymentRailX402 {
		validationErrors = append(validationErrors, domain.NewValidationError("paymentRail", "supported", "must use x402"))
	}
	if params.PurchaseChannel == PurchaseChannelBrowser && strings.TrimSpace(params.PurchaseSessionID) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("purchaseSessionId", "required", "is required for browser purchases"))
	}
	commerceSnapshotFields := 0
	if strings.TrimSpace(params.ProductDisplayName) != "" {
		commerceSnapshotFields++
	}
	if strings.TrimSpace(params.ProductSlug) != "" {
		commerceSnapshotFields++
	}
	if params.PaymentDestinationID.String() != "" {
		commerceSnapshotFields++
	}
	if commerceSnapshotFields != 0 && commerceSnapshotFields != 3 {
		validationErrors = append(validationErrors, domain.NewValidationError("commerceSnapshot", "complete", "product and payment destination snapshots must be supplied together"))
	}
	if params.PaymentDestinationID.String() != "" && params.PaymentDestinationID.Prefix() != domain.PaymentDestinationIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("paymentDestinationId", "format", "must be a payment destination identifier"))
	}
	if params.Amount.IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("amount", "positive", "must be greater than zero"))
	}
	if strings.TrimSpace(params.Asset) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("asset", "required", "is required"))
	}
	if strings.TrimSpace(params.Network) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("network", "required", "is required"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	return validationErrors
}

// intPointer returns an isolated integer pointer.
func intPointer(value int) *int {
	return &value
}

// RequireApproval moves a proposed transaction into approval pending.
func (transaction *Transaction) RequireApproval(at domain.Timestamp) error {
	return transaction.transition(StatusApprovalPending, at)
}

// MarkApproved records successful approval.
func (transaction *Transaction) MarkApproved(at domain.Timestamp) error {
	return transaction.transition(StatusApproved, at)
}

// RequirePayment marks the transaction ready for a payment challenge.
func (transaction *Transaction) RequirePayment(at domain.Timestamp) error {
	return transaction.transition(StatusPaymentRequired, at)
}

// VerifyPayment records a verified payment identifier and proof hash.
func (transaction *Transaction) VerifyPayment(
	paymentIdentifier string,
	proofHash intents.SHA256Digest,
	at domain.Timestamp,
) error {
	if strings.TrimSpace(paymentIdentifier) == "" {
		return domain.NewValidationError("paymentIdentifier", "required", "is required")
	}
	if _, err := intents.ParseSHA256Digest(proofHash.String()); err != nil {
		return domain.NewValidationError(
			"paymentProofHash",
			"format",
			"must be a SHA-256 digest",
		)
	}
	if err := transaction.transition(StatusPaymentVerified, at); err != nil {
		return err
	}
	transaction.paymentIdentifier = strings.TrimSpace(paymentIdentifier)
	transaction.paymentProofHash = proofHash
	transaction.paymentFinality = PaymentFinalityConfirmed
	reconciledAt := at
	transaction.reconciledAt = &reconciledAt
	return nil
}

// FinalizePayment records a successful settlement observation without changing delivery state.
func (transaction *Transaction) FinalizePayment(
	paymentIdentifier string,
	paymentReference string,
	at domain.Timestamp,
) error {
	if strings.TrimSpace(paymentIdentifier) != transaction.paymentIdentifier {
		return domain.NewValidationError(
			"paymentIdentifier",
			"match",
			"must match the verified payment identifier",
		)
	}
	if err := validatePaymentReference(paymentReference, true); err != nil {
		return err
	}
	if transaction.status != StatusPaymentVerified ||
		transaction.paymentFinality != PaymentFinalityConfirmed {
		return InvalidTransitionError{
			From: transaction.status,
			To:   StatusPaymentVerified,
		}
	}
	if err := transaction.validateMutationTime(at); err != nil {
		return err
	}
	transaction.paymentReference = strings.TrimSpace(paymentReference)
	transaction.paymentFinality = PaymentFinalityFinalized
	reconciledAt := at
	transaction.reconciledAt = &reconciledAt
	transaction.updatedAt = at
	transaction.version++
	return nil
}

// FailPayment records a definitive settlement rejection before seller forwarding.
func (transaction *Transaction) FailPayment(
	failureCode string,
	paymentReference string,
	at domain.Timestamp,
) error {
	if strings.TrimSpace(failureCode) == "" {
		return domain.NewValidationError("failureCode", "required", "is required")
	}
	if err := validatePaymentReference(paymentReference, false); err != nil {
		return err
	}
	if transaction.status != StatusPaymentVerified ||
		transaction.paymentFinality != PaymentFinalityConfirmed {
		return InvalidTransitionError{From: transaction.status, To: StatusFailed}
	}
	if err := transaction.transition(StatusFailed, at); err != nil {
		return err
	}
	transaction.failureCode = strings.TrimSpace(failureCode)
	transaction.paymentReference = strings.TrimSpace(paymentReference)
	transaction.paymentFinality = PaymentFinalityFailed
	reconciledAt := at
	transaction.reconciledAt = &reconciledAt
	return nil
}

// Reconciliation returns one deterministic seller reporting bucket for this transaction.
func (transaction Transaction) Reconciliation() Reconciliation {
	stage := ReconciliationStage("")
	switch transaction.status {
	case StatusPaymentRequired:
		stage = ReconciliationStageChallenged
	case StatusPaymentVerified, StatusForwarded:
		if transaction.paymentFinality == PaymentFinalityFinalized {
			stage = ReconciliationStageFinalized
		} else {
			stage = ReconciliationStageVerified
		}
	case StatusFulfilled:
		stage = ReconciliationStageFulfilled
	case StatusFailed:
		stage = ReconciliationStageFailed
	case StatusDisputed, StatusRefundRecommended, StatusResolved:
		stage = ReconciliationStageDisputed
	}
	return Reconciliation{
		Stage:            stage,
		Amount:           transaction.amount,
		Asset:            transaction.asset,
		Network:          transaction.network,
		PaymentReference: transaction.paymentReference,
		ReconciledAt:     transaction.ReconciledAt(),
	}
}

// validatePaymentReference rejects missing or unsafe settlement references.
func validatePaymentReference(paymentReference string, required bool) error {
	trimmed := strings.TrimSpace(paymentReference)
	if required && trimmed == "" {
		return domain.NewValidationError("paymentReference", "required", "is required")
	}
	if len(trimmed) > maximumPaymentReferenceLength {
		return domain.NewValidationError(
			"paymentReference",
			"length",
			"exceeds the maximum length",
		)
	}
	if strings.IndexFunc(trimmed, unicode.IsControl) >= 0 {
		return domain.NewValidationError(
			"paymentReference",
			"format",
			"must not contain control characters",
		)
	}
	return nil
}

// MarkForwarded claims the transaction for one seller invocation.
func (transaction *Transaction) MarkForwarded(at domain.Timestamp) error {
	if transaction.status != StatusPaymentVerified ||
		transaction.paymentFinality != PaymentFinalityFinalized {
		return InvalidTransitionError{From: transaction.status, To: StatusForwarded}
	}
	return transaction.transition(StatusForwarded, at)
}

// MarkFulfilled records successful seller delivery.
func (transaction *Transaction) MarkFulfilled(
	status int,
	responseHash intents.SHA256Digest,
	summary ResponseSummary,
	at domain.Timestamp,
) error {
	if status < 200 || status > 299 {
		return domain.NewValidationError(
			"upstreamStatus",
			"success",
			"must be a successful HTTP status",
		)
	}
	if _, err := intents.ParseSHA256Digest(responseHash.String()); err != nil {
		return domain.NewValidationError(
			"responseHash",
			"format",
			"must be a SHA-256 digest",
		)
	}
	if summary.ContentLength < 0 {
		return domain.NewValidationError(
			"responseSummary.contentLength",
			"non_negative",
			"must not be negative",
		)
	}
	if err := transaction.transition(StatusFulfilled, at); err != nil {
		return err
	}
	transaction.upstreamStatus = intPointer(status)
	responseHashCopy := responseHash
	transaction.responseHash = &responseHashCopy
	summaryCopy := summary
	transaction.responseSummary = &summaryCopy
	return nil
}

// MarkFailed records a stable delivery failure.
func (transaction *Transaction) MarkFailed(
	failureCode string,
	status *int,
	responseHash *intents.SHA256Digest,
	at domain.Timestamp,
) error {
	if strings.TrimSpace(failureCode) == "" {
		return domain.NewValidationError("failureCode", "required", "is required")
	}
	if responseHash != nil {
		if _, err := intents.ParseSHA256Digest(responseHash.String()); err != nil {
			return domain.NewValidationError(
				"responseHash",
				"format",
				"must be a SHA-256 digest",
			)
		}
	}
	if err := transaction.transition(StatusFailed, at); err != nil {
		return err
	}
	transaction.failureCode = strings.TrimSpace(failureCode)
	if status != nil {
		transaction.upstreamStatus = intPointer(*status)
	}
	if responseHash != nil {
		responseHashCopy := *responseHash
		transaction.responseHash = &responseHashCopy
	}
	return nil
}

// OpenDispute marks a completed transaction as disputed.
func (transaction *Transaction) OpenDispute(at domain.Timestamp) error {
	return transaction.transition(StatusDisputed, at)
}

// RecommendRefund records a deterministic refund recommendation.
func (transaction *Transaction) RecommendRefund(at domain.Timestamp) error {
	return transaction.transition(StatusRefundRecommended, at)
}

// Resolve closes a disputed transaction.
func (transaction *Transaction) Resolve(at domain.Timestamp) error {
	return transaction.transition(StatusResolved, at)
}

// transition applies one guarded transaction state change.
func (transaction *Transaction) transition(
	next TransactionStatus,
	at domain.Timestamp,
) error {
	if err := transaction.validateMutationTime(at); err != nil {
		return err
	}
	if !allowedTransition(transaction.status, next) {
		return InvalidTransitionError{From: transaction.status, To: next}
	}
	transaction.status = next
	transaction.updatedAt = at
	transaction.version++
	return nil
}

// validateMutationTime prevents state or reconciliation timestamps from moving backward.
func (transaction Transaction) validateMutationTime(at domain.Timestamp) error {
	if at.Before(transaction.updatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	return nil
}
