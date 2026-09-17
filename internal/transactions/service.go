package transactions

import (
	"context"
	"errors"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
)

const (
	defaultTransactionPageLimit = 25
	maximumTransactionPageLimit = 100
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
	if service.sellerRepository == nil {
		return ListResponse{}, ErrSellerAccess
	}
	seller, err := service.sellerRepository.GetSeller(ctx, sellerID)
	if err != nil || seller.OwnerSubject != ownerSubject {
		return ListResponse{}, ErrSellerAccess
	}
	if limit == 0 {
		limit = defaultTransactionPageLimit
	}
	if limit < 1 || limit > maximumTransactionPageLimit {
		return ListResponse{}, domain.NewValidationError(
			"limit",
			"range",
			"must be between 1 and 100",
		)
	}
	transactions, nextCursor, err := service.repository.ListBySeller(
		ctx,
		sellerID,
		limit,
		cursor,
	)
	if err != nil {
		return ListResponse{}, err
	}
	items := make([]Response, len(transactions))
	for index, transaction := range transactions {
		items[index] = transactionResponse(transaction)
	}
	return ListResponse{Items: items, NextCursor: nextCursor}, nil
}

// transactionResponse removes payment and internal persistence fields.
func transactionResponse(transaction Transaction) Response {
	return Response{
		TransactionID:  transaction.TransactionID(),
		IntentID:       transaction.IntentID(),
		SellerID:       transaction.SellerID(),
		RouteID:        transaction.RouteID(),
		BuyerID:        transaction.BuyerID(),
		Status:         transaction.Status(),
		Amount:         transaction.Amount(),
		Asset:          transaction.Asset(),
		Network:        transaction.Network(),
		UpstreamStatus: transaction.UpstreamStatus(),
		ResponseHash:   transaction.ResponseHash(),
		FailureCode:    transaction.FailureCode(),
		CreatedAt:      transaction.CreatedAt(),
		UpdatedAt:      transaction.UpdatedAt(),
	}
}

// NewTransaction validates and creates a proposed transaction.
func NewTransaction(params TransactionParams) (Transaction, error) {
	validationErrors := validateTransactionParams(params)
	if len(validationErrors) > 0 {
		return Transaction{}, validationErrors
	}

	return Transaction{
		transactionID: params.TransactionID,
		intentID:      params.IntentID,
		sellerID:      params.SellerID,
		routeID:       params.RouteID,
		buyerID:       strings.TrimSpace(params.BuyerID),
		amount:        params.Amount,
		asset:         strings.TrimSpace(params.Asset),
		network:       strings.TrimSpace(params.Network),
		status:        StatusProposed,
		createdAt:     params.CreatedAt,
		updatedAt:     params.CreatedAt,
		version:       1,
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
		return next == StatusForwarded
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
	return nil
}

// MarkForwarded claims the transaction for one seller invocation.
func (transaction *Transaction) MarkForwarded(at domain.Timestamp) error {
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
	if at.Before(transaction.updatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	if !allowedTransition(transaction.status, next) {
		return InvalidTransitionError{From: transaction.status, To: next}
	}
	transaction.status = next
	transaction.updatedAt = at
	transaction.version++
	return nil
}
