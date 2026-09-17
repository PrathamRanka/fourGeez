package settlement

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	maximumAssetLength   = 160
	maximumNetworkLength = 80
	maximumAddressLength = 256
)

// Service owns seller payment-destination creation and reads.
type Service struct {
	repository        Repository
	sellerAuthorizer  SellerAuthorizer
	idGenerator       domain.IDGenerator
	nonceGenerator    OwnershipNonceGenerator
	ownershipVerifier OwnershipVerifier
	clock             domain.Clock
	auditRecorder     audit.Recorder
}

// NewService creates the payment-destination application service.
func NewService(
	repository Repository,
	sellerAuthorizer SellerAuthorizer,
	idGenerator domain.IDGenerator,
	nonceGenerator OwnershipNonceGenerator,
	ownershipVerifier OwnershipVerifier,
	clock domain.Clock,
	auditRecorder audit.Recorder,
) *Service {
	return &Service{
		repository:        repository,
		sellerAuthorizer:  sellerAuthorizer,
		idGenerator:       idGenerator,
		nonceGenerator:    nonceGenerator,
		ownershipVerifier: ownershipVerifier,
		clock:             clock,
		auditRecorder:     auditRecorder,
	}
}

// NewPaymentDestination validates and creates pending public payout configuration.
func NewPaymentDestination(
	parameters PaymentDestinationParams,
) (PaymentDestination, error) {
	validationErrors := make(domain.ValidationErrors, 0)
	if parameters.DestinationID.Prefix() != domain.PaymentDestinationIDPrefix {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"destinationId",
				"prefix",
				"must use the payment-destination prefix",
			),
		)
	}
	if parameters.SellerID.Prefix() != domain.SellerIDPrefix {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("sellerId", "prefix", "must use the seller prefix"),
		)
	}
	asset := strings.TrimSpace(parameters.Asset)
	network := strings.TrimSpace(parameters.Network)
	address := strings.TrimSpace(parameters.Address)
	validationErrors = append(
		validationErrors,
		validatePublicValue("asset", asset, maximumAssetLength)...,
	)
	validationErrors = append(
		validationErrors,
		validatePublicValue("network", network, maximumNetworkLength)...,
	)
	validationErrors = append(
		validationErrors,
		validatePublicValue("address", address, maximumAddressLength)...,
	)
	if parameters.CreatedAt.Time().IsZero() {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("createdAt", "required", "is required"),
		)
	}
	if len(validationErrors) > 0 {
		return PaymentDestination{}, validationErrors
	}

	return PaymentDestination{
		DestinationID: parameters.DestinationID,
		SellerID:      parameters.SellerID,
		Asset:         asset,
		Network:       network,
		Address:       address,
		Status:        PaymentDestinationStatusPendingVerification,
		CreatedAt:     parameters.CreatedAt,
		UpdatedAt:     parameters.CreatedAt,
		Version:       1,
	}, nil
}

// Create adds a pending destination for an authenticated seller owner.
func (service *Service) Create(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	request CreatePaymentDestinationRequest,
) (PaymentDestination, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return PaymentDestination{}, err
	}
	destinationID, err := service.idGenerator.New(domain.PaymentDestinationIDPrefix)
	if err != nil {
		return PaymentDestination{}, err
	}
	destination, err := NewPaymentDestination(PaymentDestinationParams{
		DestinationID: destinationID,
		SellerID:      sellerID,
		Asset:         request.Asset,
		Network:       request.Network,
		Address:       request.Address,
		CreatedAt:     domain.NewTimestamp(service.clock.Now()),
	})
	if err != nil {
		return PaymentDestination{}, err
	}
	if err := service.repository.Create(ctx, destination); err != nil {
		return PaymentDestination{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID:   sellerID,
		ActorType:  audit.ActorTypeSellerUser,
		ActorID:    ownerSubject,
		Action:     audit.ActionPaymentDestinationCreated,
		TargetType: audit.TargetTypePaymentDestination,
		TargetID:   destination.DestinationID.String(),
		Outcome:    audit.OutcomeSucceeded,
		ChangedFields: []string{
			"asset",
			"network",
			"address",
			"status",
		},
	}); err != nil {
		return PaymentDestination{}, err
	}
	return destination, nil
}

// Get returns one destination only after seller ownership is verified.
func (service *Service) Get(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	destinationID domain.ID,
) (PaymentDestination, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return PaymentDestination{}, err
	}
	return service.repository.Get(ctx, sellerID, destinationID)
}

// List returns seller destinations in deterministic newest-first order.
func (service *Service) List(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
) ([]PaymentDestination, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return nil, err
	}
	destinations, err := service.repository.ListBySeller(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	sort.Slice(destinations, func(leftIndex int, rightIndex int) bool {
		left := destinations[leftIndex]
		right := destinations[rightIndex]
		if left.CreatedAt.String() == right.CreatedAt.String() {
			return left.DestinationID.String() > right.DestinationID.String()
		}
		return left.CreatedAt.Time().After(right.CreatedAt.Time())
	})
	return destinations, nil
}

// IssueOwnershipChallenge replaces any prior challenge with one short-lived proof request.
func (service *Service) IssueOwnershipChallenge(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	destinationID domain.ID,
) (OwnershipChallengeResponse, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return OwnershipChallengeResponse{}, err
	}
	destination, err := service.repository.Get(ctx, sellerID, destinationID)
	if err != nil {
		return OwnershipChallengeResponse{}, err
	}
	if destination.Status != PaymentDestinationStatusPendingVerification {
		return OwnershipChallengeResponse{}, ErrOwnershipStateInvalid
	}
	nonce, err := service.nonceGenerator.NewNonce()
	if err != nil {
		return OwnershipChallengeResponse{}, err
	}
	now := domain.NewTimestamp(service.clock.Now())
	expiresAt := domain.NewTimestamp(now.Time().Add(OwnershipChallengeLifetime))
	challenge := buildOwnershipChallenge(destination, nonce, expiresAt)
	expectedVersion := destination.Version
	destination.ChallengeHash = ownershipChallengeHash(challenge)
	destination.ChallengeExpiresAt = &expiresAt
	destination.UpdatedAt = now
	destination.Version++
	if err := service.repository.SaveChallenge(ctx, destination, expectedVersion); err != nil {
		return OwnershipChallengeResponse{}, err
	}
	return OwnershipChallengeResponse{
		Challenge: challenge,
		ExpiresAt: expiresAt,
	}, nil
}

// VerifyOwnership validates one signed challenge and atomically activates its destination.
func (service *Service) VerifyOwnership(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	destinationID domain.ID,
	request VerifyOwnershipRequest,
) (PaymentDestination, error) {
	if err := validateOwnershipProofInput(request); err != nil {
		return PaymentDestination{}, err
	}
	if err := service.sellerAuthorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return PaymentDestination{}, err
	}
	destination, err := service.repository.Get(ctx, sellerID, destinationID)
	if err != nil {
		return PaymentDestination{}, err
	}
	if destination.Status != PaymentDestinationStatusPendingVerification ||
		destination.ChallengeHash == "" ||
		destination.ChallengeExpiresAt == nil {
		return PaymentDestination{}, ErrOwnershipStateInvalid
	}
	now := domain.NewTimestamp(service.clock.Now())
	if !now.Time().Before(destination.ChallengeExpiresAt.Time()) {
		return PaymentDestination{}, ErrOwnershipChallengeExpired
	}
	if !ownershipChallengeMatches(request.Challenge, destination.ChallengeHash) {
		return PaymentDestination{}, ErrOwnershipChallengeMismatch
	}
	valid, err := service.ownershipVerifier.VerifyOwnership(
		ctx,
		destination,
		request.Challenge,
		request.Signature,
	)
	if err != nil {
		return PaymentDestination{}, err
	}
	if !valid {
		return PaymentDestination{}, ErrOwnershipProofInvalid
	}

	activeDestination, err := service.findActiveDestination(ctx, destination)
	if err != nil {
		return PaymentDestination{}, err
	}
	if activeDestination != nil && !request.ConfirmRotation {
		return PaymentDestination{}, ErrRotationConfirmationRequired
	}

	expectedVersion := destination.Version
	destination.Status = PaymentDestinationStatusActive
	destination.ChallengeHash = ""
	destination.ChallengeExpiresAt = nil
	destination.VerifiedAt = &now
	destination.UpdatedAt = now
	destination.Version++
	activation := PaymentDestinationActivation{
		Destination:     destination,
		ExpectedVersion: expectedVersion,
	}
	if activeDestination != nil {
		activation.RotatedExpectedVersion = activeDestination.Version
		activeDestination.Status = PaymentDestinationStatusRotated
		activeDestination.UpdatedAt = now
		activeDestination.Version++
		activation.RotatedDestination = activeDestination
	}
	if err := service.repository.Activate(ctx, activation); err != nil {
		return PaymentDestination{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID:   sellerID,
		ActorType:  audit.ActorTypeSellerUser,
		ActorID:    ownerSubject,
		Action:     audit.ActionPaymentDestinationVerified,
		TargetType: audit.TargetTypePaymentDestination,
		TargetID:   destination.DestinationID.String(),
		Outcome:    audit.OutcomeSucceeded,
		ChangedFields: []string{
			"status",
			"verifiedAt",
		},
	}); err != nil {
		return PaymentDestination{}, err
	}
	if activeDestination != nil {
		if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
			SellerID:   sellerID,
			ActorType:  audit.ActorTypeSellerUser,
			ActorID:    ownerSubject,
			Action:     audit.ActionPaymentDestinationRotated,
			TargetType: audit.TargetTypePaymentDestination,
			TargetID:   activeDestination.DestinationID.String(),
			Outcome:    audit.OutcomeSucceeded,
			ChangedFields: []string{
				"status",
			},
		}); err != nil {
			return PaymentDestination{}, err
		}
	}
	return destination, nil
}

// findActiveDestination locates the current active destination for an exact payment pair.
func (service *Service) findActiveDestination(
	ctx context.Context,
	destination PaymentDestination,
) (*PaymentDestination, error) {
	destinations, err := service.repository.ListBySeller(ctx, destination.SellerID)
	if err != nil {
		return nil, err
	}
	for index := range destinations {
		candidate := destinations[index]
		if candidate.DestinationID != destination.DestinationID &&
			candidate.Asset == destination.Asset &&
			candidate.Network == destination.Network &&
			candidate.Status == PaymentDestinationStatusActive {
			return &candidate, nil
		}
	}
	return nil, nil
}

// validatePublicValue rejects empty, overlong, and control-character values.
func validatePublicValue(
	fieldName string,
	value string,
	maximumLength int,
) domain.ValidationErrors {
	validationErrors := make(domain.ValidationErrors, 0)
	if value == "" {
		return append(
			validationErrors,
			domain.NewValidationError(fieldName, "required", "is required"),
		)
	}
	if len(value) > maximumLength {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(fieldName, "length", "exceeds the maximum length"),
		)
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(fieldName, "format", "must not contain control characters"),
		)
	}
	return validationErrors
}
