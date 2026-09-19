package audit

import (
	"context"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	DefaultPageLimit = 50
	MaximumPageLimit = 100
)

// ActorType identifies the authenticated or trusted source of a change.
type ActorType string

const (
	ActorTypeSellerUser            ActorType = "seller_user"
	ActorTypeIntegrationCredential ActorType = "integration_credential"
	ActorTypeAdministrator         ActorType = "administrator"
	ActorTypeSystem                ActorType = "system"
)

// Action identifies one fixed control-plane mutation.
type Action string

const (
	ActionCredentialCreated           Action = "credential.created"
	ActionCredentialRevoked           Action = "credential.revoked"
	ActionCredentialRotated           Action = "credential.rotated"
	ActionCredentialExchangeSucceeded Action = "credential.exchange_succeeded"
	ActionCredentialExchangeDenied    Action = "credential.exchange_denied"
	ActionEntitlementChanged          Action = "entitlement.changed"
	ActionMCPConfirmationIssued       Action = "mcp_confirmation.issued"
	ActionMCPConfirmationConsumed     Action = "mcp_confirmation.consumed"
	ActionMCPConfirmationDenied       Action = "mcp_confirmation.denied"
	ActionPaymentDestinationCreated   Action = "payment_destination.created"
	ActionPaymentDestinationVerified  Action = "payment_destination.verified"
	ActionPaymentDestinationDisabled  Action = "payment_destination.disabled"
	ActionPaymentDestinationRotated   Action = "payment_destination.rotated"
	ActionRouteDraftCreated           Action = "route.draft_created"
	ActionRoutePriceChanged           Action = "route.price_changed"
	ActionRoutePublished              Action = "route.published"
	ActionRoutePaused                 Action = "route.paused"
	ActionRouteArchived               Action = "route.archived"
	ActionRouteEmergencyDisabled      Action = "route.emergency_disabled"
	ActionWebhookSubscriptionCreated  Action = "webhook_subscription.created"
	ActionWebhookSubscriptionUpdated  Action = "webhook_subscription.updated"
	ActionWebhookSubscriptionDisabled Action = "webhook_subscription.disabled"
	ActionSellerSuspended             Action = "seller.suspended"
	ActionServiceIntegrationActivated Action = "service_integration.activated"
	ActionServiceEndpointVerified     Action = "service_endpoint.verified"
	ActionIntegrationVerificationDone Action = "integration_verification.completed"
	ActionManualRefundRecorded        Action = "manual_refund.recorded"
)

// TargetType identifies the domain record changed by an action.
type TargetType string

const (
	TargetTypeIntegrationCredential TargetType = "integration_credential"
	TargetTypeMCPConfirmationGrant  TargetType = "mcp_confirmation_grant"
	TargetTypePaymentDestination    TargetType = "payment_destination"
	TargetTypePaidRoute             TargetType = "paid_route"
	TargetTypeWebhookSubscription   TargetType = "webhook_subscription"
	TargetTypeSeller                TargetType = "seller"
	TargetTypeDispute               TargetType = "dispute"
)

// Outcome identifies the result represented by an audit event.
type Outcome string

const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
	OutcomeDenied    Outcome = "denied"
)

// EventParams contains the complete immutable audit event input.
type EventParams struct {
	AuditEventID  domain.ID
	SellerID      domain.ID
	ActorType     ActorType
	ActorID       string
	Action        Action
	TargetType    TargetType
	TargetID      string
	Outcome       Outcome
	RequestID     string
	ChangedFields []string
	OccurredAt    domain.Timestamp
}

// Event is one immutable seller-scoped control-plane change.
type Event struct {
	auditEventID  domain.ID
	sellerID      domain.ID
	actorType     ActorType
	actorID       string
	action        Action
	targetType    TargetType
	targetID      string
	outcome       Outcome
	requestID     string
	changedFields []string
	occurredAt    domain.Timestamp
}

// EventView is the public seller-visible audit representation.
type EventView struct {
	AuditEventID  domain.ID        `json:"auditEventId"`
	SellerID      domain.ID        `json:"sellerId"`
	ActorType     ActorType        `json:"actorType"`
	ActorID       string           `json:"actorId"`
	Action        Action           `json:"action"`
	TargetType    TargetType       `json:"targetType"`
	TargetID      string           `json:"targetId"`
	Outcome       Outcome          `json:"outcome"`
	RequestID     string           `json:"requestId"`
	ChangedFields []string         `json:"changedFields"`
	OccurredAt    domain.Timestamp `json:"occurredAt"`
}

// EventPage contains one bounded newest-first audit page.
type EventPage struct {
	Items      []EventView `json:"items"`
	NextCursor *string     `json:"nextCursor,omitempty"`
}

// RecordRequest contains safe metadata supplied by a mutation boundary.
type RecordRequest struct {
	SellerID      domain.ID
	ActorType     ActorType
	ActorID       string
	Action        Action
	TargetType    TargetType
	TargetID      string
	Outcome       Outcome
	RequestID     string
	ChangedFields []string
}

// Repository persists and queries append-only seller audit history.
type Repository interface {
	Create(context.Context, Event) error
	ListBySeller(context.Context, domain.ID, int, string) ([]Event, *string, error)
}

// SellerAuthorizer verifies that a subject owns the requested seller.
type SellerAuthorizer interface {
	AuthorizeSeller(context.Context, string, domain.ID) error
}

// Recorder is the narrow mutation-package boundary for audit appends.
type Recorder interface {
	Record(context.Context, RecordRequest) error
}

// NoopRecorder explicitly disables audit recording in isolated tests.
type NoopRecorder struct{}

// Record intentionally discards an event for tests outside AUD-001 coverage.
func (NoopRecorder) Record(context.Context, RecordRequest) error {
	return nil
}

// AuditEventID returns the immutable event identifier.
func (event Event) AuditEventID() domain.ID {
	return event.auditEventID
}

// SellerID returns the owning seller identifier.
func (event Event) SellerID() domain.ID {
	return event.sellerID
}

// ActorType returns the actor category.
func (event Event) ActorType() ActorType {
	return event.actorType
}

// ActorID returns the actor identifier.
func (event Event) ActorID() string {
	return event.actorID
}

// Action returns the fixed audit action.
func (event Event) Action() Action {
	return event.action
}

// TargetType returns the changed record category.
func (event Event) TargetType() TargetType {
	return event.targetType
}

// TargetID returns the changed record identifier.
func (event Event) TargetID() string {
	return event.targetID
}

// Outcome returns the recorded mutation result.
func (event Event) Outcome() Outcome {
	return event.outcome
}

// RequestID returns the originating request correlation identifier.
func (event Event) RequestID() string {
	return event.requestID
}

// ChangedFields returns an isolated list of changed field names.
func (event Event) ChangedFields() []string {
	return append([]string(nil), event.changedFields...)
}

// OccurredAt returns the immutable event timestamp.
func (event Event) OccurredAt() domain.Timestamp {
	return event.occurredAt
}
