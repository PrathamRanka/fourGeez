package audit

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	maximumActorIDLength     = 256
	maximumRequestIDLength   = 128
	maximumChangedFieldCount = 16
)

type actionDefinition struct {
	targetType    TargetType
	changedFields map[string]struct{}
}

var actionDefinitions = map[Action]actionDefinition{
	ActionCredentialCreated:           newActionDefinition(TargetTypeIntegrationCredential, "label", "scopes", "expiresAt"),
	ActionCredentialRevoked:           newActionDefinition(TargetTypeIntegrationCredential, "revokedAt"),
	ActionCredentialRotated:           newActionDefinition(TargetTypeIntegrationCredential, "revokedAt", "replacedByCredentialId"),
	ActionCredentialExchangeSucceeded: newActionDefinition(TargetTypeIntegrationCredential, "lastUsedAt"),
	ActionCredentialExchangeDenied:    newActionDefinition(TargetTypeIntegrationCredential, "authorization"),
	ActionEntitlementChanged:          newActionDefinition(TargetTypeSeller, "status", "accessEndsAt", "entitlementEpoch", "sourceRevision", "credentialRotationRequired"),
	ActionMCPConfirmationIssued: newActionDefinition(
		TargetTypeMCPConfirmationGrant,
		"credentialId", "tool", "targetType", "targetId", "argumentsSha256", "expectedResourceVersion", "expiresAt",
	),
	ActionMCPConfirmationConsumed:    newActionDefinition(TargetTypeMCPConfirmationGrant, "consumedAt", "consumedByIdempotencyKeyHash"),
	ActionMCPConfirmationDenied:      newActionDefinition(TargetTypeMCPConfirmationGrant, "authorization"),
	ActionPaymentDestinationCreated:  newActionDefinition(TargetTypePaymentDestination, "asset", "network", "address", "status"),
	ActionPaymentDestinationVerified: newActionDefinition(TargetTypePaymentDestination, "status", "verifiedAt"),
	ActionPaymentDestinationDisabled: newActionDefinition(TargetTypePaymentDestination, "status"),
	ActionPaymentDestinationRotated:  newActionDefinition(TargetTypePaymentDestination, "status"),
	ActionRouteDraftCreated: newActionDefinition(
		TargetTypePaidRoute,
		"displayName",
		"productSlug",
		"method",
		"pathPattern",
		"description",
		"mimeType",
		"amount",
		"asset",
		"network",
		"payTo",
		"approvalThresholdAmount",
		"upstreamTimeoutSeconds",
		"lifecycleStatus",
		"enabled",
	),
	ActionRoutePriceChanged: newActionDefinition(TargetTypePaidRoute, "amount"),
	ActionRoutePublished: newActionDefinition(
		TargetTypePaidRoute,
		"displayName",
		"productSlug",
		"method",
		"pathPattern",
		"description",
		"mimeType",
		"amount",
		"asset",
		"network",
		"payTo",
		"approvalThresholdAmount",
		"upstreamTimeoutSeconds",
		"lifecycleStatus",
		"enabled",
	),
	ActionRoutePaused:                 newActionDefinition(TargetTypePaidRoute, "lifecycleStatus", "enabled"),
	ActionRouteArchived:               newActionDefinition(TargetTypePaidRoute, "lifecycleStatus", "enabled"),
	ActionRouteEmergencyDisabled:      newActionDefinition(TargetTypePaidRoute, "lifecycleStatus", "enabled"),
	ActionWebhookSubscriptionCreated:  newActionDefinition(TargetTypeWebhookSubscription, "endpointUrl", "eventTypes", "status"),
	ActionWebhookSubscriptionUpdated:  newActionDefinition(TargetTypeWebhookSubscription, "endpointUrl", "eventTypes", "status"),
	ActionWebhookSubscriptionDisabled: newActionDefinition(TargetTypeWebhookSubscription, "status"),
	ActionSellerSuspended:             newActionDefinition(TargetTypeSeller, "status"),
}

// Service owns audit validation, append operations, and seller history reads.
type Service struct {
	repository Repository
	authorizer SellerAuthorizer
	appender   *Appender
}

// Appender validates and persists audit events for mutation packages.
type Appender struct {
	repository  Repository
	idGenerator domain.IDGenerator
	clock       domain.Clock
}

// NewAppender creates the mutation-facing audit recorder.
func NewAppender(
	repository Repository,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
) *Appender {
	return &Appender{
		repository:  repository,
		idGenerator: idGenerator,
		clock:       clock,
	}
}

// NewService creates the append-only audit service.
func NewService(
	repository Repository,
	authorizer SellerAuthorizer,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
) *Service {
	return &Service{
		repository: repository,
		authorizer: authorizer,
		appender:   NewAppender(repository, idGenerator, clock),
	}
}

// Record validates and appends one immutable audit event.
func (service *Service) Record(
	ctx context.Context,
	request RecordRequest,
) error {
	return service.appender.Record(ctx, request)
}

// Record validates and appends one immutable audit event.
func (appender *Appender) Record(
	ctx context.Context,
	request RecordRequest,
) error {
	requestID := request.RequestID
	if requestID == "" {
		requestID, _ = api.RequestIDFromContext(ctx)
	}
	auditEventID, err := appender.idGenerator.New(domain.AuditEventIDPrefix)
	if err != nil {
		return err
	}
	event, err := NewEvent(EventParams{
		AuditEventID:  auditEventID,
		SellerID:      request.SellerID,
		ActorType:     request.ActorType,
		ActorID:       request.ActorID,
		Action:        request.Action,
		TargetType:    request.TargetType,
		TargetID:      request.TargetID,
		Outcome:       request.Outcome,
		RequestID:     requestID,
		ChangedFields: request.ChangedFields,
		OccurredAt:    domain.NewTimestamp(appender.clock.Now()),
	})
	if err != nil {
		return err
	}
	return appender.repository.Create(ctx, event)
}

// List authorizes and returns one bounded newest-first seller history page.
func (service *Service) List(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	limit int,
	cursor string,
) (EventPage, error) {
	if err := service.authorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return EventPage{}, err
	}
	if limit == 0 {
		limit = DefaultPageLimit
	}
	if limit < 1 || limit > MaximumPageLimit {
		return EventPage{}, domain.NewValidationError(
			"limit",
			"range",
			"must be between 1 and 100",
		)
	}
	events, nextCursor, err := service.repository.ListBySeller(
		ctx,
		sellerID,
		limit,
		cursor,
	)
	if err != nil {
		return EventPage{}, err
	}
	views := make([]EventView, len(events))
	for index, event := range events {
		views[index] = eventView(event)
	}
	return EventPage{Items: views, NextCursor: nextCursor}, nil
}

// NewEvent validates and creates one immutable audit event.
func NewEvent(parameters EventParams) (Event, error) {
	validationErrors := make(domain.ValidationErrors, 0)
	if parameters.AuditEventID.Prefix() != domain.AuditEventIDPrefix {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("auditEventId", "prefix", "must use the audit event prefix"),
		)
	}
	if parameters.SellerID.Prefix() != domain.SellerIDPrefix {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("sellerId", "prefix", "must use the seller prefix"),
		)
	}
	validationErrors = append(validationErrors, validateActor(parameters.ActorType, parameters.ActorID)...)
	validationErrors = append(
		validationErrors,
		validateAction(parameters.Action, parameters.TargetType, parameters.ChangedFields)...,
	)
	if err := validateTargetID(parameters.TargetType, parameters.TargetID); err != nil {
		validationErrors = append(validationErrors, *err)
	}
	if !validOutcome(parameters.Outcome) {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("outcome", "supported", "must use a supported audit outcome"),
		)
	}
	if !validBoundedText(parameters.RequestID, maximumRequestIDLength) {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("requestId", "format", "must contain 1-128 printable characters"),
		)
	}
	if parameters.OccurredAt.Time().IsZero() {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("occurredAt", "required", "is required"),
		)
	}
	if len(validationErrors) > 0 {
		return Event{}, validationErrors
	}
	changedFields := append([]string(nil), parameters.ChangedFields...)
	sort.Strings(changedFields)
	return Event{
		auditEventID:  parameters.AuditEventID,
		sellerID:      parameters.SellerID,
		actorType:     parameters.ActorType,
		actorID:       strings.TrimSpace(parameters.ActorID),
		action:        parameters.Action,
		targetType:    parameters.TargetType,
		targetID:      parameters.TargetID,
		outcome:       parameters.Outcome,
		requestID:     parameters.RequestID,
		changedFields: changedFields,
		occurredAt:    parameters.OccurredAt,
	}, nil
}

// newActionDefinition creates one immutable action validation definition.
func newActionDefinition(
	targetType TargetType,
	changedFields ...string,
) actionDefinition {
	allowlist := make(map[string]struct{}, len(changedFields))
	for _, changedField := range changedFields {
		allowlist[changedField] = struct{}{}
	}
	return actionDefinition{targetType: targetType, changedFields: allowlist}
}

// validateActor validates actor vocabulary and identifier shape.
func validateActor(actorType ActorType, actorID string) domain.ValidationErrors {
	validationErrors := make(domain.ValidationErrors, 0)
	switch actorType {
	case ActorTypeSellerUser, ActorTypeAdministrator, ActorTypeSystem:
		if !validBoundedText(actorID, maximumActorIDLength) {
			validationErrors = append(
				validationErrors,
				domain.NewValidationError("actorId", "format", "must contain 1-256 printable characters"),
			)
		}
	case ActorTypeIntegrationCredential:
		if _, err := domain.ParseID(actorID, domain.CredentialIDPrefix); err != nil {
			validationErrors = append(
				validationErrors,
				domain.NewValidationError("actorId", "format", "must identify an integration credential"),
			)
		}
	default:
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("actorType", "supported", "must use a supported audit actor"),
		)
	}
	return validationErrors
}

// validateAction enforces action, target, and changed-field vocabulary.
func validateAction(
	action Action,
	targetType TargetType,
	changedFields []string,
) domain.ValidationErrors {
	definition, exists := actionDefinitions[action]
	if !exists {
		return domain.ValidationErrors{
			domain.NewValidationError("action", "supported", "must use a supported audit action"),
		}
	}
	validationErrors := make(domain.ValidationErrors, 0)
	if targetType != definition.targetType {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("targetType", "action", "does not match the audit action"),
		)
	}
	if len(changedFields) == 0 || len(changedFields) > maximumChangedFieldCount {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("changedFields", "length", "must contain 1-16 field names"),
		)
		return validationErrors
	}
	seen := make(map[string]struct{}, len(changedFields))
	for _, changedField := range changedFields {
		if _, duplicate := seen[changedField]; duplicate {
			validationErrors = append(
				validationErrors,
				domain.NewValidationError("changedFields", "unique", "must not contain duplicate field names"),
			)
			continue
		}
		seen[changedField] = struct{}{}
		if _, allowed := definition.changedFields[changedField]; !allowed {
			validationErrors = append(
				validationErrors,
				domain.NewValidationError("changedFields", "allowlist", "contains a field not allowed for the action"),
			)
		}
	}
	return validationErrors
}

// validateTargetID verifies the target identifier for its declared type.
func validateTargetID(targetType TargetType, targetID string) *domain.ValidationError {
	var prefix domain.IDPrefix
	switch targetType {
	case TargetTypeIntegrationCredential:
		prefix = domain.CredentialIDPrefix
	case TargetTypeMCPConfirmationGrant:
		prefix = domain.MCPConfirmationGrantIDPrefix
	case TargetTypePaymentDestination:
		prefix = domain.PaymentDestinationIDPrefix
	case TargetTypePaidRoute:
		prefix = domain.RouteIDPrefix
	case TargetTypeWebhookSubscription:
		prefix = domain.WebhookSubscriptionIDPrefix
	case TargetTypeSeller:
		prefix = domain.SellerIDPrefix
	default:
		validationError := domain.NewValidationError(
			"targetType",
			"supported",
			"must use a supported audit target",
		)
		return &validationError
	}
	if _, err := domain.ParseID(targetID, prefix); err != nil {
		validationError := domain.NewValidationError(
			"targetId",
			"format",
			"must match the declared audit target type",
		)
		return &validationError
	}
	return nil
}

// validOutcome reports whether an outcome belongs to the fixed vocabulary.
func validOutcome(outcome Outcome) bool {
	switch outcome {
	case OutcomeSucceeded, OutcomeFailed, OutcomeDenied:
		return true
	default:
		return false
	}
}

// validBoundedText rejects empty, overlong, and control-character metadata.
func validBoundedText(value string, maximumLength int) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" &&
		len(trimmed) <= maximumLength &&
		strings.IndexFunc(trimmed, unicode.IsControl) < 0
}
