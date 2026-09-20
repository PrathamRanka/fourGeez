package notifications

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"io"
	"net"
	"net/url"
	"sort"
	"strings"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
)

const minimumWebhookSecretLength = 32

// WebhookSubscriptionQuota checks seller capacity before subscription creation.
type WebhookSubscriptionQuota interface {
	AllowWebhookSubscription(context.Context, domain.ID, uint64) error
}

// SecureSecretGenerator creates URL-safe webhook signing secrets.
type SecureSecretGenerator struct{ reader io.Reader }

// NewSecureSecretGenerator uses crypto/rand when no reader is supplied.
func NewSecureSecretGenerator(reader io.Reader) *SecureSecretGenerator {
	if reader == nil {
		reader = cryptorand.Reader
	}
	return &SecureSecretGenerator{reader: reader}
}

// NewSecret returns 256 bits of URL-safe secret material.
func (generator *SecureSecretGenerator) NewSecret() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := io.ReadFull(generator.reader, randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

// Service owns seller webhook subscription configuration.
type Service struct {
	repository      Repository
	authorizer      SellerAuthorizer
	idGenerator     domain.IDGenerator
	secretGenerator SecretGenerator
	secretStore     SecretStore
	clock           domain.Clock
	auditRecorder   audit.Recorder
	quotaEnforcer   WebhookSubscriptionQuota
}

// NewService creates the webhook subscription service.
func NewService(
	repository Repository,
	authorizer SellerAuthorizer,
	idGenerator domain.IDGenerator,
	secretGenerator SecretGenerator,
	secretStore SecretStore,
	clock domain.Clock,
	auditRecorder audit.Recorder,
) *Service {
	return &Service{
		repository:      repository,
		authorizer:      authorizer,
		idGenerator:     idGenerator,
		secretGenerator: secretGenerator,
		secretStore:     secretStore,
		clock:           clock,
		auditRecorder:   auditRecorder,
	}
}

// SetQuotaEnforcer configures plan checks for webhook subscription creation.
func (service *Service) SetQuotaEnforcer(quotaEnforcer WebhookSubscriptionQuota) {
	service.quotaEnforcer = quotaEnforcer
}

// Create stores a subscription and returns its signing secret once.
func (service *Service) Create(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	request CreateSubscriptionRequest,
) (SubscriptionCreated, error) {
	if err := service.authorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return SubscriptionCreated{}, err
	}
	subscriptions, err := service.repository.ListBySeller(ctx, sellerID)
	if err != nil {
		return SubscriptionCreated{}, err
	}
	var activeSubscriptionCount uint64
	for _, subscription := range subscriptions {
		if subscription.Status() == SubscriptionStatusActive {
			activeSubscriptionCount++
		}
	}
	if service.quotaEnforcer != nil {
		if err := service.quotaEnforcer.AllowWebhookSubscription(
			ctx,
			sellerID,
			activeSubscriptionCount,
		); err != nil {
			return SubscriptionCreated{}, err
		}
	}
	subscriptionID, err := service.idGenerator.New(domain.WebhookSubscriptionIDPrefix)
	if err != nil {
		return SubscriptionCreated{}, err
	}
	secret, err := service.secretGenerator.NewSecret()
	if err != nil {
		return SubscriptionCreated{}, err
	}
	if len(secret) < minimumWebhookSecretLength {
		return SubscriptionCreated{}, ErrSigningUnavailable
	}
	secretRef := "agentpay/webhooks/" + sellerID.String() + "/" + subscriptionID.String()
	createdAt := domain.NewTimestamp(service.clock.Now())
	subscription, err := NewSubscription(SubscriptionParams{
		SubscriptionID: subscriptionID,
		SellerID:       sellerID,
		EndpointURL:    request.EndpointURL,
		EventTypes:     request.EventTypes,
		SecretRef:      secretRef,
		CreatedAt:      createdAt,
	})
	if err != nil {
		return SubscriptionCreated{}, err
	}
	if err := service.secretStore.PutSecret(ctx, secretRef, []byte(secret)); err != nil {
		return SubscriptionCreated{}, err
	}
	if err := service.repository.Create(ctx, subscription); err != nil {
		return SubscriptionCreated{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID:   sellerID,
		ActorType:  audit.ActorTypeSellerUser,
		ActorID:    ownerSubject,
		Action:     audit.ActionWebhookSubscriptionCreated,
		TargetType: audit.TargetTypeWebhookSubscription,
		TargetID:   subscription.SubscriptionID().String(),
		Outcome:    audit.OutcomeSucceeded,
		ChangedFields: []string{
			"endpointUrl",
			"eventTypes",
			"status",
		},
	}); err != nil {
		return SubscriptionCreated{}, err
	}
	return SubscriptionCreated{
		SubscriptionView: subscriptionView(subscription),
		SigningSecret:    secret,
	}, nil
}

// NewSubscription validates and creates active webhook configuration.
func NewSubscription(params SubscriptionParams) (Subscription, error) {
	var validationErrors domain.ValidationErrors
	if params.SubscriptionID.Prefix() != domain.WebhookSubscriptionIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("subscriptionId", "prefix", "must use the webhook subscription prefix"))
	}
	if params.SellerID.Prefix() != domain.SellerIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "prefix", "must use the seller prefix"))
	}
	endpointURL := strings.TrimSpace(params.EndpointURL)
	if validationError := validateWebhookEndpoint(endpointURL); validationError != nil {
		validationErrors = append(validationErrors, *validationError)
	}
	eventTypes, validationError := normalizeEventTypes(params.EventTypes)
	if validationError != nil {
		validationErrors = append(validationErrors, *validationError)
	}
	if strings.TrimSpace(params.SecretRef) == "" {
		validationErrors = append(validationErrors, domain.NewValidationError("secretRef", "required", "is required"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	if len(validationErrors) > 0 {
		return Subscription{}, validationErrors
	}
	return Subscription{subscriptionID: params.SubscriptionID, sellerID: params.SellerID, endpointURL: endpointURL, eventTypes: eventTypes, secretRef: strings.TrimSpace(params.SecretRef), status: SubscriptionStatusActive, createdAt: params.CreatedAt, updatedAt: params.CreatedAt, version: 1}, nil
}

// List returns redacted subscriptions after seller authorization.
func (service *Service) List(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
) ([]SubscriptionView, error) {
	if err := service.authorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return nil, err
	}
	subscriptions, err := service.repository.ListBySeller(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	views := make([]SubscriptionView, len(subscriptions))
	for index, subscription := range subscriptions {
		views[index] = subscriptionView(subscription)
	}
	return views, nil
}

// Disable stops new deliveries to one seller-owned subscription.
func (service *Service) Disable(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	subscriptionID domain.ID,
	request DisableSubscriptionRequest,
) (SubscriptionView, error) {
	if err := service.authorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return SubscriptionView{}, err
	}
	if request.ExpectedVersion == 0 {
		return SubscriptionView{}, domain.NewValidationError("expectedVersion", "required", "must be greater than zero")
	}
	subscription, err := service.repository.Get(ctx, sellerID, subscriptionID)
	if err != nil {
		return SubscriptionView{}, err
	}
	if subscription.Version() != request.ExpectedVersion {
		return SubscriptionView{}, domain.NewValidationError("expectedVersion", "stale", "does not match the current subscription version")
	}
	expectedVersion := subscription.Version()
	if err := subscription.Disable(domain.NewTimestamp(service.clock.Now())); err != nil {
		return SubscriptionView{}, err
	}
	if err := service.repository.Update(ctx, subscription, expectedVersion); err != nil {
		return SubscriptionView{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID:      sellerID,
		ActorType:     audit.ActorTypeSellerUser,
		ActorID:       ownerSubject,
		Action:        audit.ActionWebhookSubscriptionDisabled,
		TargetType:    audit.TargetTypeWebhookSubscription,
		TargetID:      subscription.SubscriptionID().String(),
		Outcome:       audit.OutcomeSucceeded,
		ChangedFields: []string{"status"},
	}); err != nil {
		return SubscriptionView{}, err
	}
	return subscriptionView(subscription), nil
}

// validateWebhookEndpoint rejects unsafe static endpoint forms before delivery-time DNS checks.
func validateWebhookEndpoint(raw string) *domain.ValidationError {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		validationError := domain.NewValidationError(
			"endpointUrl",
			"https",
			"must be a public HTTPS URL",
		)
		return &validationError
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		validationError := domain.NewValidationError(
			"endpointUrl",
			"public",
			"must not target localhost",
		)
		return &validationError
	}
	if address := net.ParseIP(host); address != nil && (address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast()) {
		validationError := domain.NewValidationError(
			"endpointUrl",
			"public",
			"must target a public address",
		)
		return &validationError
	}
	return nil
}

// normalizeEventTypes validates, deduplicates, and sorts the event allowlist.
func normalizeEventTypes(eventTypes []EventType) ([]EventType, *domain.ValidationError) {
	if len(eventTypes) == 0 {
		validationError := domain.NewValidationError(
			"eventTypes",
			"required",
			"must contain at least one event",
		)
		return nil, &validationError
	}
	unique := make(map[EventType]struct{})
	for _, eventType := range eventTypes {
		if !validEventType(eventType) {
			validationError := domain.NewValidationError(
				"eventTypes",
				"supported",
				"contains an unsupported event",
			)
			return nil, &validationError
		}
		unique[eventType] = struct{}{}
	}
	result := make([]EventType, 0, len(unique))
	for eventType := range unique {
		result = append(result, eventType)
	}
	sort.Slice(result, func(leftIndex int, rightIndex int) bool { return result[leftIndex] < result[rightIndex] })
	return result, nil
}

// validEventType reports whether an event belongs to the public webhook contract.
func validEventType(eventType EventType) bool {
	switch eventType {
	case EventPaymentVerified, EventFulfillmentSucceeded, EventFulfillmentFailed, EventDisputeChanged:
		return true
	default:
		return false
	}
}

// subscriptionView removes the secret reference from public responses.
func subscriptionView(subscription Subscription) SubscriptionView {
	return SubscriptionView{
		SubscriptionID: subscription.SubscriptionID(),
		SellerID:       subscription.SellerID(),
		EndpointURL:    subscription.EndpointURL(),
		EventTypes:     subscription.EventTypes(),
		Status:         subscription.Status(),
		CreatedAt:      subscription.CreatedAt(),
		UpdatedAt:      subscription.UpdatedAt(),
		Version:        subscription.Version(),
	}
}
