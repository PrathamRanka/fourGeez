package api

import (
	"context"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	// RequestIDHeader carries the request correlation identifier.
	RequestIDHeader = "X-AgentPay-Request-Id"

	// AgentKeyHeader carries the agent API credential.
	AgentKeyHeader = "X-AgentPay-Agent-Key"

	// MaximumJSONBodyBytes limits control-plane JSON payloads to one MiB.
	MaximumJSONBodyBytes int64 = 1 << 20

	ErrorCodeBadRequest                   = "bad_request"
	ErrorCodeUnauthorized                 = "unauthorized"
	ErrorCodeConflict                     = "conflict"
	ErrorCodeGone                         = "gone"
	ErrorCodeNotFound                     = "not_found"
	ErrorCodePermissionDenied             = "permission_denied"
	ErrorCodeRateLimited                  = "rate_limited"
	ErrorCodeUnprocessable                = "unprocessable_entity"
	ErrorCodeInternal                     = "internal_error"
	ErrorCodeDependencyUnavailable        = "dependency_unavailable"
	ErrorCodeInvalidCredential            = "invalid_credential"
	ErrorCodeTokenExpired                 = "token_expired"
	ErrorCodeTokenRevoked                 = "token_revoked"
	ErrorCodeSubscriptionInactive         = "subscription_inactive"
	ErrorCodeInsufficientScope            = "insufficient_scope"
	ErrorCodeIdempotencyConflict          = "idempotency_conflict"
	ErrorCodeTokenReplayed                = "token_replayed"
	ErrorCodePaymentRejected              = "payment_rejected"
	ErrorCodePaymentReplayed              = "payment_replayed"
	ErrorCodePaymentCapabilityUnsupported = "payment_capability_unsupported"
	ErrorCodePaymentExpired               = "payment_expired"
	ErrorCodePaymentUnavailable           = "payment_unavailable"
	ErrorCodePaymentOutcomeUnknown        = "payment_outcome_unknown"
	ErrorCodeSellerInactive               = "seller_inactive"
	ErrorCodeRouteContractStale           = "route_contract_stale"
	ErrorCodeValidationFailed             = "validation_failed"
)

// IdempotencyDecision contains either a replay or permission to execute.
type IdempotencyDecision struct {
	Key         domain.IdempotencyKey
	RequestHash string
	Replay      bool
	Status      int
	Body        []byte
}

// ErrorBody is the stable machine-readable API error.
type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details,omitempty"`
}

// ErrorResponse wraps an API error according to the OpenAPI contract.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// PrincipalKind identifies an authenticated caller category.
type PrincipalKind string

const (
	PrincipalSeller  PrincipalKind = "seller"
	PrincipalAgent   PrincipalKind = "agent"
	PrincipalBrowser PrincipalKind = "browser"
)

// Principal is the authenticated identity available to controllers.
type Principal struct {
	Kind      PrincipalKind
	Subject   string
	TokenID   string
	SessionID string
	ExpiresAt time.Time
}

// Authenticator validates seller and agent credentials.
type Authenticator interface {
	AuthenticateSeller(ctx context.Context, token string) (Principal, bool)
	AuthenticateAgent(ctx context.Context, key string) (Principal, bool)
}

// SellerRequestLimiter consumes one authenticated seller API request.
type SellerRequestLimiter interface {
	ConsumeAPIRequest(context.Context, domain.ID) error
}

// SellerRequestAuthorizer verifies the authenticated subject owns a seller path.
type SellerRequestAuthorizer interface {
	AuthorizeSeller(context.Context, string, domain.ID) error
}

type requestIDContextKey struct{}
type principalContextKey struct{}
type authenticatorContextKey struct{}
