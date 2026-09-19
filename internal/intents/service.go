package intents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"mime"
	"regexp"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/gowebpki/jcs"
)

const (
	intentHashDomain     = "agentpay.intent.v1"
	intentSchemaVersion  = "1"
	maximumBuyerIDLength = 160
	maximumAssetLength   = 160
	maximumNetworkLength = 80
)

const defaultIntentLifetime = 10 * time.Minute

var (
	requestPathPattern     = regexp.MustCompile(`^/[A-Za-z0-9/_-]+$`)
	sha256Pattern          = regexp.MustCompile(`^[a-f0-9]{64}$`)
	purchaseSessionPattern = regexp.MustCompile(`^bps_[A-Za-z0-9]+$`)
)

// Service coordinates intent rules with catalog and persistence boundaries.
type Service struct {
	repository         Repository
	routeRepository    RouteRepository
	idGenerator        domain.IDGenerator
	clock              domain.Clock
	commerceAuthorizer CommerceAuthorizer
}

// NewServiceWithCommerceAuthorizer requires fresh publication and destination checks.
func NewServiceWithCommerceAuthorizer(
	repository Repository,
	routeRepository RouteRepository,
	commerceAuthorizer CommerceAuthorizer,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
) *Service {
	service := NewService(repository, routeRepository, idGenerator, clock)
	service.commerceAuthorizer = commerceAuthorizer
	return service
}

// NewService creates the purchase-intent application service.
func NewService(
	repository Repository,
	routeRepository RouteRepository,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
) *Service {
	return &Service{
		repository:      repository,
		routeRepository: routeRepository,
		idGenerator:     idGenerator,
		clock:           clock,
	}
}

// Create resolves the seller quote and persists an immutable purchase intent.
func (service *Service) Create(
	ctx context.Context,
	buyerID string,
	request CreateIntentRequest,
) (PurchaseIntent, error) {
	var route catalog.PaidRoute
	var destination settlement.PaymentDestination
	var err error
	if service.commerceAuthorizer != nil {
		route, destination, err = service.commerceAuthorizer.AuthorizeIntent(ctx, request.RouteID)
	} else {
		route, err = service.routeRepository.GetRoute(ctx, request.RouteID)
	}
	if err != nil {
		return PurchaseIntent{}, err
	}
	intentID, err := service.idGenerator.New(domain.IntentIDPrefix)
	if err != nil {
		return PurchaseIntent{}, err
	}
	createdAt := domain.NewTimestamp(service.clock.Now())
	purchaseIntent, err := NewPurchaseIntent(PurchaseIntentParams{
		IntentID:             intentID,
		SellerID:             route.SellerID,
		RouteID:              route.RouteID,
		BuyerID:              buyerID,
		ProductDisplayName:   route.DisplayName,
		ProductSlug:          route.ProductSlug,
		PaymentDestinationID: destination.DestinationID,
		PayTo:                choosePayTo(destination.Address, route.PayTo),
		RequestMethod:        RequestMethod(route.Method),
		RequestPath:          route.PathPattern,
		RequestBodyHash:      request.RequestBodyHash,
		Amount:               route.Amount,
		Asset:                route.Asset,
		Network:              route.Network,
		MaximumAmount:        request.MaximumAmount,
		RequiresApproval:     false,
		CreatedAt:            createdAt,
		ExpiresAt:            createdAt.Add(defaultIntentLifetime),
	})
	if err != nil {
		return PurchaseIntent{}, err
	}
	if err := service.repository.Create(ctx, purchaseIntent); err != nil {
		return PurchaseIntent{}, err
	}
	return purchaseIntent, nil
}

func choosePayTo(verifiedAddress, legacyAddress string) string {
	if strings.TrimSpace(verifiedAddress) != "" {
		return strings.TrimSpace(verifiedAddress)
	}
	return strings.TrimSpace(legacyAddress)
}

// Get returns a persisted immutable purchase intent.
func (service *Service) Get(
	ctx context.Context,
	intentID domain.ID,
) (PurchaseIntent, error) {
	return service.repository.Get(ctx, intentID)
}

// createPurchaseIntent constructs an immutable validated purchase proposal.
func createPurchaseIntent(params PurchaseIntentParams) (PurchaseIntent, error) {
	params = normalizePurchaseIdentity(params)
	validationErrors := validatePurchaseIntentParams(params)
	if len(validationErrors) > 0 {
		return PurchaseIntent{}, validationErrors
	}

	status := PurchaseIntentStatusReady
	if params.RequiresApproval {
		status = PurchaseIntentStatusApprovalPending
	}

	intentHash, err := hashCanonicalValue(intentHashDomain, intentHashPayload{
		SchemaVersion:        intentSchemaVersion,
		IntentID:             params.IntentID.String(),
		SellerID:             params.SellerID.String(),
		RouteID:              params.RouteID.String(),
		BuyerID:              strings.TrimSpace(params.BuyerID),
		PurchaseSessionID:    strings.TrimSpace(params.PurchaseSessionID),
		PurchaseChannel:      params.PurchaseChannel,
		ProductDisplayName:   strings.TrimSpace(params.ProductDisplayName),
		ProductSlug:          strings.TrimSpace(params.ProductSlug),
		PaymentDestinationID: params.PaymentDestinationID.String(),
		PayTo:                strings.TrimSpace(params.PayTo),
		RequestMethod:        params.RequestMethod,
		RequestPath:          params.RequestPath,
		RequestBodyHash:      params.RequestBodyHash.String(),
		Amount:               params.Amount.String(),
		Asset:                strings.TrimSpace(params.Asset),
		Network:              strings.TrimSpace(params.Network),
		MaximumAmount:        params.MaximumAmount.String(),
		RequiresApproval:     params.RequiresApproval,
		CreatedAt:            params.CreatedAt.String(),
		ExpiresAt:            params.ExpiresAt.String(),
	})
	if err != nil {
		return PurchaseIntent{}, err
	}

	return PurchaseIntent{
		intentID:             params.IntentID,
		sellerID:             params.SellerID,
		routeID:              params.RouteID,
		buyerID:              strings.TrimSpace(params.BuyerID),
		purchaseSessionID:    strings.TrimSpace(params.PurchaseSessionID),
		purchaseChannel:      params.PurchaseChannel,
		productDisplayName:   strings.TrimSpace(params.ProductDisplayName),
		productSlug:          strings.TrimSpace(params.ProductSlug),
		paymentDestinationID: params.PaymentDestinationID,
		payTo:                strings.TrimSpace(params.PayTo),
		requestMethod:        params.RequestMethod,
		requestPath:          params.RequestPath,
		requestBodyHash:      params.RequestBodyHash,
		amount:               params.Amount,
		asset:                strings.TrimSpace(params.Asset),
		network:              strings.TrimSpace(params.Network),
		maximumAmount:        params.MaximumAmount,
		requiresApproval:     params.RequiresApproval,
		intentHash:           intentHash,
		status:               status,
		createdAt:            params.CreatedAt,
		expiresAt:            params.ExpiresAt,
		version:              1,
	}, nil
}

func (purchaseIntent *PurchaseIntent) Cancel(at domain.Timestamp) error {
	if purchaseIntent.status == PurchaseIntentStatusCancelled {
		return nil
	}
	if purchaseIntent.status != PurchaseIntentStatusReady {
		return InvalidLifecycleTransitionError{From: purchaseIntent.status, To: PurchaseIntentStatusCancelled}
	}
	if !at.Time().Before(purchaseIntent.expiresAt.Time()) {
		return ErrIntentExpired
	}
	if at.Time().Before(purchaseIntent.createdAt.Time()) {
		return domain.NewValidationError("cancelledAt", "chronology", "must not precede intent creation")
	}
	purchaseIntent.status = PurchaseIntentStatusCancelled
	cancelledAt := at
	purchaseIntent.cancelledAt = &cancelledAt
	purchaseIntent.cancellationReason = CancellationReasonBuyerRequested
	purchaseIntent.version++
	return nil
}

func (purchaseIntent *PurchaseIntent) Expire(at domain.Timestamp) error {
	if purchaseIntent.status == PurchaseIntentStatusExpired {
		return nil
	}
	if purchaseIntent.status != PurchaseIntentStatusReady || at.Time().Before(purchaseIntent.expiresAt.Time()) {
		return InvalidLifecycleTransitionError{From: purchaseIntent.status, To: PurchaseIntentStatusExpired}
	}
	purchaseIntent.status = PurchaseIntentStatusExpired
	purchaseIntent.version++
	return nil
}

func (purchaseIntent *PurchaseIntent) Claim(at domain.Timestamp) error {
	if purchaseIntent.status == PurchaseIntentStatusExecuted {
		return nil
	}
	if purchaseIntent.status != PurchaseIntentStatusReady {
		return InvalidLifecycleTransitionError{From: purchaseIntent.status, To: PurchaseIntentStatusExecuted}
	}
	if !at.Time().Before(purchaseIntent.expiresAt.Time()) {
		return ErrIntentExpired
	}
	if at.Time().Before(purchaseIntent.createdAt.Time()) {
		return domain.NewValidationError("executedAt", "chronology", "must not precede intent creation")
	}
	purchaseIntent.status = PurchaseIntentStatusExecuted
	purchaseIntent.version++
	return nil
}

func (service *Service) Cancel(ctx context.Context, intentID domain.ID, buyerID string) (PurchaseIntent, error) {
	repository, ok := service.repository.(LifecycleRepository)
	if !ok {
		return PurchaseIntent{}, errors.New("purchase intent lifecycle persistence is unavailable")
	}
	purchaseIntent, err := repository.Get(ctx, intentID)
	if err != nil {
		return PurchaseIntent{}, err
	}
	if strings.TrimSpace(buyerID) == "" || purchaseIntent.BuyerID() != strings.TrimSpace(buyerID) {
		return PurchaseIntent{}, ErrIntentAccess
	}
	now := domain.NewTimestamp(service.clock.Now())
	switch purchaseIntent.Status() {
	case PurchaseIntentStatusCancelled:
		return purchaseIntent, nil
	case PurchaseIntentStatusExpired:
		return PurchaseIntent{}, ErrIntentExpired
	case PurchaseIntentStatusExecuted, PurchaseIntentStatusApprovalPending:
		return PurchaseIntent{}, ErrIntentStateConflict
	}
	if !now.Time().Before(purchaseIntent.ExpiresAt().Time()) {
		if purchaseIntent.Status() == PurchaseIntentStatusReady {
			expectedVersion := purchaseIntent.Version()
			if expireErr := purchaseIntent.Expire(now); expireErr == nil {
				if updateErr := repository.Update(ctx, purchaseIntent, expectedVersion); updateErr != nil && !errors.Is(updateErr, persistence.ErrConditionFailed) {
					return PurchaseIntent{}, updateErr
				}
			}
		}
		return PurchaseIntent{}, ErrIntentExpired
	}
	expectedVersion := purchaseIntent.Version()
	if err := purchaseIntent.Cancel(now); err != nil {
		return PurchaseIntent{}, ErrIntentStateConflict
	}
	if err := repository.Update(ctx, purchaseIntent, expectedVersion); err != nil {
		if !errors.Is(err, persistence.ErrConditionFailed) {
			return PurchaseIntent{}, err
		}
		current, loadErr := repository.Get(ctx, intentID)
		if loadErr != nil {
			return PurchaseIntent{}, loadErr
		}
		if current.Status() == PurchaseIntentStatusCancelled {
			return current, nil
		}
		return PurchaseIntent{}, ErrIntentStateConflict
	}
	return purchaseIntent, nil
}

// parseSHA256Digest validates the canonical lowercase digest representation.
func parseSHA256Digest(raw string) (SHA256Digest, error) {
	if !sha256Pattern.MatchString(raw) {
		return "", domain.NewValidationError(
			"sha256",
			"format",
			"must contain 64 lowercase hexadecimal characters",
		)
	}
	return SHA256Digest(raw), nil
}

// hashRequestBody canonicalizes JSON before hashing request bytes.
func hashRequestBody(requestBody []byte, mediaType string) (SHA256Digest, error) {
	canonicalBody := requestBody
	if mediaType != "" {
		parsedMediaType, _, err := mime.ParseMediaType(mediaType)
		if err != nil {
			return "", domain.NewValidationError(
				"requestContentType",
				"format",
				"must be a valid media type",
			)
		}
		if parsedMediaType == "application/json" || strings.HasSuffix(parsedMediaType, "+json") {
			canonicalBody, err = jcs.Transform(requestBody)
			if err != nil {
				return "", domain.NewValidationError(
					"requestBody",
					"json",
					"must contain valid canonicalizable JSON",
				)
			}
		}
	}
	return hashBytes(canonicalBody), nil
}

// hashCanonicalValue hashes a JCS value with an unambiguous domain separator.
func hashCanonicalValue(domainSeparator string, value any) (SHA256Digest, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(encoded)
	if err != nil {
		return "", err
	}

	payload := make([]byte, 0, len(domainSeparator)+1+len(canonical))
	payload = append(payload, domainSeparator...)
	payload = append(payload, 0)
	payload = append(payload, canonical...)
	return hashBytes(payload), nil
}

// hashBytes returns the lowercase SHA-256 digest of value.
func hashBytes(value []byte) SHA256Digest {
	digest := sha256.Sum256(value)
	return SHA256Digest(hex.EncodeToString(digest[:]))
}

// validatePurchaseIntentParams validates every execution-relevant field.
func validatePurchaseIntentParams(params PurchaseIntentParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.IntentID.String(), domain.IntentIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("intentId", "format", "must be a purchase-intent identifier"))
	}
	if _, err := domain.ParseID(params.SellerID.String(), domain.SellerIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "format", "must be a seller identifier"))
	}
	if _, err := domain.ParseID(params.RouteID.String(), domain.RouteIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("routeId", "format", "must be a route identifier"))
	}
	if strings.TrimSpace(params.ProductDisplayName) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("productDisplayName", "required", "is required"))
	}
	if strings.TrimSpace(params.ProductSlug) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("productSlug", "required", "is required"))
	}
	if params.PaymentDestinationID != "" {
		if _, err := domain.ParseID(params.PaymentDestinationID.String(), domain.PaymentDestinationIDPrefix); err != nil {
			validationErrors = append(validationErrors, domain.NewValidationError("paymentDestinationId", "format", "must be a payment-destination identifier"))
		}
	}
	if strings.TrimSpace(params.PayTo) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("payTo", "required", "is required"))
	}

	buyerID := strings.TrimSpace(params.BuyerID)
	if buyerID == "" || len(buyerID) > maximumBuyerIDLength {
		validationErrors = append(validationErrors, domain.NewValidationError("buyerId", "length", "must contain 1-160 characters"))
	}
	switch params.PurchaseChannel {
	case PurchaseChannelAgent:
		if strings.TrimSpace(params.PurchaseSessionID) != "" {
			validationErrors = append(validationErrors, domain.NewValidationError("purchaseSessionId", "forbidden", "must be empty for agent purchases"))
		}
	case PurchaseChannelBrowser:
		if !purchaseSessionPattern.MatchString(strings.TrimSpace(params.PurchaseSessionID)) || buyerID != "browser:"+strings.TrimSpace(params.PurchaseSessionID) {
			validationErrors = append(validationErrors, domain.NewValidationError("purchaseSessionId", "binding", "must identify the owning browser purchase session"))
		}
	default:
		validationErrors = append(validationErrors, domain.NewValidationError("purchaseChannel", "supported", "must be agent or browser"))
	}
	if params.RequestMethod != RequestMethodGet && params.RequestMethod != RequestMethodPost {
		validationErrors = append(validationErrors, domain.NewValidationError("requestMethod", "supported", "must be GET or POST"))
	}
	if !requestPathPattern.MatchString(params.RequestPath) {
		validationErrors = append(validationErrors, domain.NewValidationError("requestPath", "format", "must be a literal absolute route path"))
	}
	if !sha256Pattern.MatchString(params.RequestBodyHash.String()) {
		validationErrors = append(validationErrors, domain.NewValidationError("requestBodyHash", "format", "must be a SHA-256 digest"))
	}
	if params.Amount.IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("amount", "positive", "must be greater than zero"))
	}
	if params.Amount.Compare(params.MaximumAmount) > 0 {
		validationErrors = append(validationErrors, domain.NewValidationError("maximumAmount", "limit", "must be greater than or equal to the seller's exact price"))
	}
	if value := strings.TrimSpace(params.Asset); value == "" || len(value) > maximumAssetLength {
		validationErrors = append(validationErrors, domain.NewValidationError("asset", "length", "must contain 1-160 characters"))
	}
	if value := strings.TrimSpace(params.Network); value == "" || len(value) > maximumNetworkLength {
		validationErrors = append(validationErrors, domain.NewValidationError("network", "length", "must contain 1-80 characters"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	if params.ExpiresAt.Time().IsZero() || !params.ExpiresAt.Time().After(params.CreatedAt.Time()) {
		validationErrors = append(validationErrors, domain.NewValidationError("expiresAt", "chronology", "must occur after creation"))
	}
	return validationErrors
}

func normalizePurchaseIdentity(params PurchaseIntentParams) PurchaseIntentParams {
	params.BuyerID = strings.TrimSpace(params.BuyerID)
	params.PurchaseSessionID = strings.TrimSpace(params.PurchaseSessionID)
	if params.PurchaseChannel != "" {
		return params
	}
	const browserBuyerPrefix = "browser:"
	if strings.HasPrefix(params.BuyerID, browserBuyerPrefix) {
		params.PurchaseChannel = PurchaseChannelBrowser
		if params.PurchaseSessionID == "" {
			params.PurchaseSessionID = strings.TrimPrefix(params.BuyerID, browserBuyerPrefix)
		}
		return params
	}
	params.PurchaseChannel = PurchaseChannelAgent
	return params
}
