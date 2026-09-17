package notifications

import (
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

const maximumAutomaticDeliveryAttempts uint32 = 5

var (
	sha256HexPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

	// ErrDeliveryNotFound reports a missing seller-owned delivery.
	ErrDeliveryNotFound = errors.New("webhook delivery was not found")
	// ErrDeliveryConflict reports an invalid concurrent delivery transition.
	ErrDeliveryConflict = errors.New("webhook delivery state conflict")
	// ErrWebhookForbiddenTarget reports a destination that could enable SSRF.
	ErrWebhookForbiddenTarget = errors.New("webhook target is forbidden")
	// ErrWebhookRedirectForbidden reports a redirect-based destination change.
	ErrWebhookRedirectForbidden = errors.New("webhook redirects are forbidden")
	// ErrWebhookTimeout reports a delivery attempt that exceeded its limit.
	ErrWebhookTimeout = errors.New("webhook request timed out")
	// ErrWebhookUnavailable reports a failed webhook transport.
	ErrWebhookUnavailable = errors.New("webhook endpoint is unavailable")
	// ErrWebhookResponseTooLarge reports response bytes above the hash limit.
	ErrWebhookResponseTooLarge = errors.New("webhook response is too large")
)

// DeliveryStatus identifies the current webhook delivery lifecycle state.
type DeliveryStatus string

const (
	DeliveryStatusPending        DeliveryStatus = "pending"
	DeliveryStatusRetryScheduled DeliveryStatus = "retry_scheduled"
	DeliveryStatusDelivered      DeliveryStatus = "delivered"
	DeliveryStatusDeadLetter     DeliveryStatus = "dead_letter"
)

// DeliveryFailureCode is a safe classification for the latest failed attempt.
type DeliveryFailureCode string

const (
	DeliveryFailureForbiddenTarget   DeliveryFailureCode = "forbidden_target"
	DeliveryFailureTimeout           DeliveryFailureCode = "timeout"
	DeliveryFailureUnavailable       DeliveryFailureCode = "unavailable"
	DeliveryFailureResponseTooLarge  DeliveryFailureCode = "response_too_large"
	DeliveryFailureRetryableResponse DeliveryFailureCode = "retryable_response"
	DeliveryFailurePermanentResponse DeliveryFailureCode = "permanent_response"
)

// DeliveryParams contains the values required to create a webhook delivery.
type DeliveryParams struct {
	DeliveryID     domain.ID
	SellerID       domain.ID
	SubscriptionID domain.ID
	Event          WebhookEvent
	PayloadHash    string
	CreatedAt      domain.Timestamp
}

// Delivery stores one stable event delivery and its bounded attempt history.
type Delivery struct {
	deliveryID         domain.ID
	sellerID           domain.ID
	subscriptionID     domain.ID
	event              WebhookEvent
	payloadHash        string
	status             DeliveryStatus
	attemptCount       uint32
	nextAttemptAt      *domain.Timestamp
	lastAttemptAt      *domain.Timestamp
	deliveredAt        *domain.Timestamp
	responseStatusCode *int
	responseBodyHash   *string
	errorCode          *DeliveryFailureCode
	createdAt          domain.Timestamp
	updatedAt          domain.Timestamp
	version            uint64
}

// DeliveryView is the seller-visible delivery history representation.
type DeliveryView struct {
	DeliveryID         domain.ID            `json:"deliveryId"`
	SellerID           domain.ID            `json:"sellerId"`
	SubscriptionID     domain.ID            `json:"subscriptionId"`
	EventID            domain.ID            `json:"eventId"`
	EventType          EventType            `json:"eventType"`
	PayloadHash        string               `json:"payloadHash"`
	Status             DeliveryStatus       `json:"status"`
	AttemptCount       uint32               `json:"attemptCount"`
	NextAttemptAt      *domain.Timestamp    `json:"nextAttemptAt"`
	LastAttemptAt      *domain.Timestamp    `json:"lastAttemptAt"`
	DeliveredAt        *domain.Timestamp    `json:"deliveredAt"`
	ResponseStatusCode *int                 `json:"responseStatusCode"`
	ResponseBodyHash   *string              `json:"responseBodyHash"`
	ErrorCode          *DeliveryFailureCode `json:"errorCode"`
	CreatedAt          domain.Timestamp     `json:"createdAt"`
	UpdatedAt          domain.Timestamp     `json:"updatedAt"`
	Version            uint64               `json:"version"`
}

var automaticRetryDelays = []time.Duration{
	time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
}

// NewDelivery validates and creates one immediately due webhook delivery.
func NewDelivery(params DeliveryParams) (Delivery, error) {
	var validationErrors domain.ValidationErrors
	if params.DeliveryID.Prefix() != domain.WebhookDeliveryIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("deliveryId", "prefix", "must use the webhook delivery prefix"))
	}
	if params.SellerID.Prefix() != domain.SellerIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "prefix", "must use the seller prefix"))
	}
	if params.SubscriptionID.Prefix() != domain.WebhookSubscriptionIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("subscriptionId", "prefix", "must use the webhook subscription prefix"))
	}
	if params.Event.SchemaVersion != "1" ||
		params.Event.EventID.Prefix() != domain.EvidenceIDPrefix ||
		params.Event.SellerID != params.SellerID ||
		!validEventType(params.Event.EventType) ||
		params.Event.OccurredAt.Time().IsZero() ||
		params.Event.Payload == nil {
		validationErrors = append(validationErrors, domain.NewValidationError("event", "contract", "must satisfy the webhook event contract"))
	}
	if !sha256HexPattern.MatchString(params.PayloadHash) {
		validationErrors = append(validationErrors, domain.NewValidationError("payloadHash", "sha256", "must be lowercase SHA-256 hex"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	if len(validationErrors) > 0 {
		return Delivery{}, validationErrors
	}
	nextAttemptAt := params.CreatedAt
	return Delivery{
		deliveryID:     params.DeliveryID,
		sellerID:       params.SellerID,
		subscriptionID: params.SubscriptionID,
		event:          params.Event,
		payloadHash:    params.PayloadHash,
		status:         DeliveryStatusPending,
		nextAttemptAt:  &nextAttemptAt,
		createdAt:      params.CreatedAt,
		updatedAt:      params.CreatedAt,
		version:        1,
	}, nil
}

// RecordSuccess marks the first successful two-hundred-range response.
func (delivery *Delivery) RecordSuccess(
	attemptedAt domain.Timestamp,
	statusCode int,
	responseBodyHash string,
) error {
	if !delivery.canAttempt(attemptedAt) || statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return ErrDeliveryConflict
	}
	if responseBodyHash != "" && !sha256HexPattern.MatchString(responseBodyHash) {
		return domain.NewValidationError("responseBodyHash", "sha256", "must be lowercase SHA-256 hex")
	}
	delivery.attemptCount++
	delivery.status = DeliveryStatusDelivered
	delivery.nextAttemptAt = nil
	delivery.lastAttemptAt = timestampPointer(attemptedAt)
	delivery.deliveredAt = timestampPointer(attemptedAt)
	delivery.responseStatusCode = intPointer(statusCode)
	delivery.responseBodyHash = optionalStringPointer(responseBodyHash)
	delivery.errorCode = nil
	delivery.updatedAt = attemptedAt
	delivery.version++
	return nil
}

// RecordFailure records safe failure metadata and applies bounded retry policy.
func (delivery *Delivery) RecordFailure(
	attemptedAt domain.Timestamp,
	errorCode DeliveryFailureCode,
	statusCode int,
	responseBodyHash string,
) error {
	if !delivery.canAttempt(attemptedAt) || !validDeliveryFailureCode(errorCode) {
		return ErrDeliveryConflict
	}
	if responseBodyHash != "" && !sha256HexPattern.MatchString(responseBodyHash) {
		return domain.NewValidationError("responseBodyHash", "sha256", "must be lowercase SHA-256 hex")
	}
	delivery.attemptCount++
	delivery.lastAttemptAt = timestampPointer(attemptedAt)
	delivery.responseStatusCode = optionalIntPointer(statusCode)
	delivery.responseBodyHash = optionalStringPointer(responseBodyHash)
	delivery.errorCode = failureCodePointer(errorCode)
	delivery.updatedAt = attemptedAt
	delivery.version++

	if errorCode == DeliveryFailurePermanentResponse ||
		errorCode == DeliveryFailureForbiddenTarget ||
		errorCode == DeliveryFailureResponseTooLarge ||
		delivery.attemptCount >= maximumAutomaticDeliveryAttempts {
		delivery.status = DeliveryStatusDeadLetter
		delivery.nextAttemptAt = nil
		return nil
	}
	retryIndex := delivery.attemptCount - 1
	delivery.status = DeliveryStatusRetryScheduled
	nextAttemptAt := attemptedAt.Add(automaticRetryDelays[retryIndex])
	delivery.nextAttemptAt = &nextAttemptAt
	return nil
}

// ScheduleRedelivery makes one dead-letter delivery immediately due again.
func (delivery *Delivery) ScheduleRedelivery(scheduledAt domain.Timestamp) error {
	if delivery.status != DeliveryStatusDeadLetter || scheduledAt.Time().Before(delivery.updatedAt.Time()) {
		return ErrDeliveryConflict
	}
	delivery.status = DeliveryStatusPending
	delivery.nextAttemptAt = timestampPointer(scheduledAt)
	delivery.responseStatusCode = nil
	delivery.responseBodyHash = nil
	delivery.errorCode = nil
	delivery.updatedAt = scheduledAt
	delivery.version++
	return nil
}

// canAttempt reports whether the delivery is due and mutable.
func (delivery Delivery) canAttempt(attemptedAt domain.Timestamp) bool {
	return (delivery.status == DeliveryStatusPending ||
		delivery.status == DeliveryStatusRetryScheduled) &&
		delivery.nextAttemptAt != nil &&
		!attemptedAt.Time().Before(delivery.nextAttemptAt.Time())
}

// DeliveryID returns the stable delivery identifier.
func (delivery Delivery) DeliveryID() domain.ID {
	return delivery.deliveryID
}

// SellerID returns the owning seller identifier.
func (delivery Delivery) SellerID() domain.ID {
	return delivery.sellerID
}

// SubscriptionID returns the target subscription identifier.
func (delivery Delivery) SubscriptionID() domain.ID {
	return delivery.subscriptionID
}

// Event returns the stable webhook event.
func (delivery Delivery) Event() WebhookEvent {
	return delivery.event
}

// PayloadHash returns the canonical webhook body digest.
func (delivery Delivery) PayloadHash() string {
	return delivery.payloadHash
}

// Status returns the current delivery lifecycle state.
func (delivery Delivery) Status() DeliveryStatus {
	return delivery.status
}

// AttemptCount returns the completed HTTP attempt count.
func (delivery Delivery) AttemptCount() uint32 {
	return delivery.attemptCount
}

// NextAttemptAt returns an isolated due timestamp when one exists.
func (delivery Delivery) NextAttemptAt() *domain.Timestamp {
	return copyTimestampPointer(delivery.nextAttemptAt)
}

// Version returns the optimistic concurrency version.
func (delivery Delivery) Version() uint64 {
	return delivery.version
}

// validDeliveryFailureCode reports whether a failure is safe and documented.
func validDeliveryFailureCode(errorCode DeliveryFailureCode) bool {
	switch errorCode {
	case DeliveryFailureForbiddenTarget,
		DeliveryFailureTimeout,
		DeliveryFailureUnavailable,
		DeliveryFailureResponseTooLarge,
		DeliveryFailureRetryableResponse,
		DeliveryFailurePermanentResponse:
		return true
	default:
		return false
	}
}

// timestampPointer returns a pointer to an isolated timestamp value.
func timestampPointer(value domain.Timestamp) *domain.Timestamp {
	copy := value
	return &copy
}

// copyTimestampPointer copies an optional timestamp.
func copyTimestampPointer(value *domain.Timestamp) *domain.Timestamp {
	if value == nil {
		return nil
	}
	return timestampPointer(*value)
}

// intPointer returns a pointer to an integer.
func intPointer(value int) *int {
	copy := value
	return &copy
}

// optionalIntPointer returns nil for missing HTTP status metadata.
func optionalIntPointer(value int) *int {
	if value == 0 {
		return nil
	}
	return intPointer(value)
}

// optionalStringPointer returns nil for missing hash metadata.
func optionalStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

// failureCodePointer returns a pointer to a safe delivery failure code.
func failureCodePointer(value DeliveryFailureCode) *DeliveryFailureCode {
	copy := value
	return &copy
}
