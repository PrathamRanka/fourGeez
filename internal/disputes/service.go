package disputes

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// ErrDisputeAccess prevents cross-seller dispute disclosure.
var ErrDisputeAccess = errors.New("dispute was not found")

// Service coordinates dispute classification with transaction state.
type Service struct {
	repository            Repository
	transactionRepository TransactionRepository
	sellerRepository      SellerRepository
	idGenerator           domain.IDGenerator
	clock                 domain.Clock
}

// NewService creates the dispute application service.
func NewService(
	repository Repository,
	transactionRepository TransactionRepository,
	sellerRepository SellerRepository,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
) *Service {
	return &Service{
		repository:            repository,
		transactionRepository: transactionRepository,
		sellerRepository:      sellerRepository,
		idGenerator:           idGenerator,
		clock:                 clock,
	}
}

// Create opens, classifies, and persists a dispute from recorded facts.
func (service *Service) Create(
	ctx context.Context,
	request CreateRequest,
	sellerSubject string,
) (Dispute, error) {
	transaction, err := service.transactionRepository.Get(
		ctx,
		request.TransactionID,
	)
	if err != nil {
		return Dispute{}, err
	}
	if err := service.authorizeSeller(
		ctx,
		transaction,
		sellerSubject,
	); err != nil {
		return Dispute{}, err
	}
	now := domain.NewTimestamp(service.clock.Now())
	disputeID, err := service.idGenerator.New(domain.DisputeIDPrefix)
	if err != nil {
		return Dispute{}, err
	}
	dispute, err := Classify(
		Params{
			DisputeID:     disputeID,
			TransactionID: transaction.TransactionID(),
			Reason:        request.Reason,
			Statement:     request.Statement,
			CreatedAt:     now,
		},
		factsFromTransaction(transaction),
	)
	if err != nil {
		return Dispute{}, err
	}
	expectedVersion := transaction.Version()
	if err := transaction.OpenDispute(now); err != nil {
		return Dispute{}, err
	}
	if err := service.transactionRepository.Update(
		ctx,
		transaction,
		expectedVersion,
	); err != nil {
		return Dispute{}, err
	}

	if err := service.repository.Create(ctx, dispute); err != nil {
		return Dispute{}, err
	}
	if dispute.Status == StatusRefundRecommended {
		refundedVersion := transaction.Version()
		if err := transaction.RecommendRefund(now); err != nil {
			return Dispute{}, err
		}
		if err := service.transactionRepository.Update(
			ctx,
			transaction,
			refundedVersion,
		); err != nil {
			return Dispute{}, err
		}
	}
	return dispute, nil
}

// Get returns a persisted dispute to an agent caller.
func (service *Service) Get(
	ctx context.Context,
	disputeID domain.ID,
) (Dispute, error) {
	return service.repository.Get(ctx, disputeID)
}

// GetForSeller returns a dispute only to its transaction's owning seller.
func (service *Service) GetForSeller(
	ctx context.Context,
	disputeID domain.ID,
	sellerSubject string,
) (Dispute, error) {
	dispute, err := service.repository.Get(ctx, disputeID)
	if err != nil {
		return Dispute{}, err
	}
	transaction, err := service.transactionRepository.Get(
		ctx,
		dispute.TransactionID,
	)
	if err != nil {
		return Dispute{}, err
	}
	if err := service.authorizeSeller(
		ctx,
		transaction,
		sellerSubject,
	); err != nil {
		return Dispute{}, err
	}
	return dispute, nil
}

// authorizeSeller enforces seller ownership when a seller subject is present.
func (service *Service) authorizeSeller(
	ctx context.Context,
	transaction transactions.Transaction,
	sellerSubject string,
) error {
	if sellerSubject == "" {
		return nil
	}
	seller, err := service.sellerRepository.GetSeller(
		ctx,
		transaction.SellerID(),
	)
	if err != nil || seller.OwnerSubject != sellerSubject {
		return ErrDisputeAccess
	}
	return nil
}

// factsFromTransaction derives only facts already recorded on the transaction.
func factsFromTransaction(transaction transactions.Transaction) Facts {
	var facts Facts
	if transaction.PaymentIdentifier() != "" &&
		transaction.PaymentProofHash().String() != "" {
		authorizationValid := true
		duplicatePayment := false
		expectedAmount := transaction.Amount()
		paidAmount := transaction.Amount()
		facts.AuthorizationValid = &authorizationValid
		facts.DuplicatePayment = &duplicatePayment
		facts.ExpectedAmount = &expectedAmount
		facts.PaidAmount = &paidAmount
	}
	switch transaction.Status() {
	case transactions.StatusFulfilled:
		deliverySucceeded := true
		facts.DeliverySucceeded = &deliverySucceeded
	case transactions.StatusFailed:
		deliverySucceeded := false
		facts.DeliverySucceeded = &deliverySucceeded
	}
	return facts
}

// Classify validates a dispute request and applies the deterministic rules.
func Classify(params Params, facts Facts) (Dispute, error) {
	if err := validateParams(params); err != nil {
		return Dispute{}, err
	}
	status, code, explanation := classify(params.Reason, facts)
	return newDispute(params, status, code, explanation), nil
}

// newDispute constructs the classified dispute model.
func newDispute(params Params, status Status, code, explanation string) Dispute {
	return Dispute{
		DisputeID:          params.DisputeID,
		TransactionID:      params.TransactionID,
		Reason:             params.Reason,
		Statement:          strings.TrimSpace(params.Statement),
		Status:             status,
		RuleVersion:        RuleVersion,
		ClassificationCode: code,
		Explanation:        explanation,
		CreatedAt:          params.CreatedAt,
	}
}

// classify applies the rule associated with a dispute reason.
func classify(reason Reason, facts Facts) (Status, string, string) {
	switch reason {
	case ReasonUnauthorized:
		if facts.AuthorizationValid == nil {
			return insufficientEvidence()
		}
		if !*facts.AuthorizationValid {
			return StatusRefundRecommended, CodeAuthorizationNotValid, "Recorded authorization evidence is not valid; a refund is recommended."
		}
		return StatusDenied, CodeAuthorizationValid, "Recorded authorization evidence is valid, so the unauthorized claim is denied."
	case ReasonDuplicate:
		if facts.DuplicatePayment == nil {
			return insufficientEvidence()
		}
		if *facts.DuplicatePayment {
			return StatusRefundRecommended, CodeDuplicatePaymentConfirmed, "A duplicate payment was confirmed; a refund is recommended."
		}
		return StatusDenied, CodeDuplicatePaymentNotFound, "No duplicate payment was found, so the duplicate claim is denied."
	case ReasonWrongAmount:
		if facts.ExpectedAmount == nil || facts.PaidAmount == nil {
			return insufficientEvidence()
		}
		if facts.ExpectedAmount.Compare(*facts.PaidAmount) != 0 {
			return StatusRefundRecommended, CodeWrongAmountConfirmed, "The paid amount differs from the frozen intent amount; a refund is recommended."
		}
		return StatusDenied, CodeAmountMatchesIntent, "The paid amount matches the frozen intent amount, so the wrong-amount claim is denied."
	case ReasonNotDelivered:
		if facts.DeliverySucceeded == nil {
			return insufficientEvidence()
		}
		if !*facts.DeliverySucceeded {
			return StatusRefundRecommended, CodeDeliveryNotConfirmed, "Successful seller delivery was not recorded; a refund is recommended."
		}
		return StatusDenied, CodeDeliveryConfirmed, "Successful seller delivery was recorded, so the non-delivery claim is denied."
	case ReasonQualityOrOutput:
		return StatusSellerReview, CodeQualityReviewRequired, "Output quality requires seller review and is not judged automatically."
	default:
		panic("validated dispute reason reached classifier")
	}
}

// insufficientEvidence returns the manual-review fallback classification.
func insufficientEvidence() (Status, string, string) {
	return StatusSellerReview, CodeInsufficientEvidence, "Recorded evidence is insufficient for an automatic recommendation; seller review is required."
}

// validateParams validates a dispute creation request.
func validateParams(params Params) error {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.DisputeID.String(), domain.DisputeIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("disputeId", "format", "must be a dispute identifier"))
	}
	if _, err := domain.ParseID(params.TransactionID.String(), domain.TransactionIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("transactionId", "format", "must be a transaction identifier"))
	}
	if !validReason(params.Reason) {
		validationErrors = append(validationErrors, domain.NewValidationError("reason", "supported", "must use a documented dispute reason"))
	}
	if utf8.RuneCountInString(params.Statement) > maximumStatementLength {
		validationErrors = append(validationErrors, domain.NewValidationError("statement", "length", "must not exceed 2000 characters"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	if len(validationErrors) > 0 {
		return validationErrors
	}
	return nil
}

// validReason reports whether a reason belongs to the public dispute vocabulary.
func validReason(reason Reason) bool {
	switch reason {
	case ReasonUnauthorized, ReasonDuplicate, ReasonWrongAmount, ReasonNotDelivered, ReasonQualityOrOutput:
		return true
	default:
		return false
	}
}
