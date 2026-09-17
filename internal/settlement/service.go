package settlement

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	maximumAssetLength   = 160
	maximumNetworkLength = 80
	maximumAddressLength = 256
)

// Service owns seller payment-destination creation and reads.
type Service struct {
	repository       Repository
	sellerAuthorizer SellerAuthorizer
	idGenerator      domain.IDGenerator
	clock            domain.Clock
}

// NewService creates the payment-destination application service.
func NewService(
	repository Repository,
	sellerAuthorizer SellerAuthorizer,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
) *Service {
	return &Service{
		repository:       repository,
		sellerAuthorizer: sellerAuthorizer,
		idGenerator:      idGenerator,
		clock:            clock,
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
