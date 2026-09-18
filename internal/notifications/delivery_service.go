package notifications

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	defaultDeliveryPageSize = 50
	maximumDeliveryPageSize = 100
)

// WebhookDeliveryQuota consumes one retry-safe logical delivery unit.
type WebhookDeliveryQuota interface {
	ConsumeWebhookDelivery(context.Context, domain.ID, string) error
}

// DeliveryRepository persists webhook deliveries and their unique event claims.
type DeliveryRepository interface {
	CreateIfAbsent(context.Context, Delivery) (Delivery, bool, error)
	Get(context.Context, domain.ID, domain.ID) (Delivery, error)
	Update(context.Context, Delivery, uint64) error
	ListBySeller(context.Context, domain.ID, int, string) ([]Delivery, *string, error)
}

// EventSigner signs one canonical event with a subscription secret.
type EventSigner interface {
	Sign(context.Context, string, WebhookEvent) (SignedEvent, error)
}

// DeliverySender sends one signed webhook through the protected HTTP boundary.
type DeliverySender interface {
	Send(context.Context, string, SignedEvent) (WebhookSendResult, error)
}

// DeliveryPage contains one bounded seller-visible delivery history page.
type DeliveryPage struct {
	Items      []DeliveryView `json:"items"`
	NextCursor *string        `json:"nextCursor"`
}

// DeliveryService owns webhook fan-out, attempts, history, and redelivery.
type DeliveryService struct {
	deliveryRepository     DeliveryRepository
	subscriptionRepository Repository
	authorizer             SellerAuthorizer
	idGenerator            domain.IDGenerator
	signer                 EventSigner
	sender                 DeliverySender
	clock                  domain.Clock
	quotaEnforcer          WebhookDeliveryQuota
}

// NewDeliveryService creates the webhook delivery use case.
func NewDeliveryService(
	deliveryRepository DeliveryRepository,
	subscriptionRepository Repository,
	authorizer SellerAuthorizer,
	idGenerator domain.IDGenerator,
	signer EventSigner,
	sender DeliverySender,
	clock domain.Clock,
) *DeliveryService {
	return &DeliveryService{
		deliveryRepository:     deliveryRepository,
		subscriptionRepository: subscriptionRepository,
		authorizer:             authorizer,
		idGenerator:            idGenerator,
		signer:                 signer,
		sender:                 sender,
		clock:                  clock,
	}
}

// SetQuotaEnforcer configures retry-safe webhook delivery metering.
func (service *DeliveryService) SetQuotaEnforcer(quotaEnforcer WebhookDeliveryQuota) {
	service.quotaEnforcer = quotaEnforcer
}

// Enqueue creates one idempotent delivery for each matching active subscription.
func (service *DeliveryService) Enqueue(
	ctx context.Context,
	event WebhookEvent,
) ([]DeliveryView, error) {
	canonicalBody, err := CanonicalEventBody(event)
	if err != nil {
		return nil, err
	}
	subscriptions, err := service.subscriptionRepository.ListBySeller(ctx, event.SellerID)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(canonicalBody)
	payloadHash := hex.EncodeToString(digest[:])
	createdAt := domain.NewTimestamp(service.clock.Now())
	views := make([]DeliveryView, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		if subscription.Status() != SubscriptionStatusActive ||
			!subscriptionIncludesEvent(subscription, event.EventType) {
			continue
		}
		sourceID := subscription.SubscriptionID().String() + ":" + event.EventID.String()
		if service.quotaEnforcer != nil {
			if err := service.quotaEnforcer.ConsumeWebhookDelivery(
				ctx,
				event.SellerID,
				sourceID,
			); err != nil {
				return nil, err
			}
		}
		deliveryID, err := service.idGenerator.New(domain.WebhookDeliveryIDPrefix)
		if err != nil {
			return nil, err
		}
		delivery, err := NewDelivery(DeliveryParams{
			DeliveryID:     deliveryID,
			SellerID:       event.SellerID,
			SubscriptionID: subscription.SubscriptionID(),
			Event:          event,
			PayloadHash:    payloadHash,
			CreatedAt:      createdAt,
		})
		if err != nil {
			return nil, err
		}
		stored, _, err := service.deliveryRepository.CreateIfAbsent(ctx, delivery)
		if err != nil {
			return nil, err
		}
		views = append(views, deliveryView(stored))
	}
	return views, nil
}

// Attempt sends one due delivery and persists its classified outcome.
func (service *DeliveryService) Attempt(
	ctx context.Context,
	sellerID domain.ID,
	deliveryID domain.ID,
) (DeliveryView, error) {
	delivery, err := service.deliveryRepository.Get(ctx, sellerID, deliveryID)
	if err != nil {
		return DeliveryView{}, err
	}
	attemptedAt := domain.NewTimestamp(service.clock.Now())
	if !delivery.canAttempt(attemptedAt) {
		return DeliveryView{}, ErrDeliveryConflict
	}
	subscription, err := service.subscriptionRepository.Get(
		ctx,
		sellerID,
		delivery.SubscriptionID(),
	)
	if err != nil {
		return DeliveryView{}, err
	}
	expectedVersion := delivery.Version()
	signedEvent, err := service.signer.Sign(ctx, subscription.SecretRef(), delivery.Event())
	if err != nil {
		return DeliveryView{}, err
	}
	result, sendErr := service.sender.Send(ctx, subscription.EndpointURL(), signedEvent)
	if transitionErr := recordDeliveryAttempt(&delivery, attemptedAt, result, sendErr); transitionErr != nil {
		return DeliveryView{}, transitionErr
	}
	if err := service.deliveryRepository.Update(ctx, delivery, expectedVersion); err != nil {
		return DeliveryView{}, err
	}
	return deliveryView(delivery), nil
}

// List returns one authorized bounded seller delivery page.
func (service *DeliveryService) List(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	limit int,
	cursor string,
) (DeliveryPage, error) {
	if err := service.authorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return DeliveryPage{}, err
	}
	if limit == 0 {
		limit = defaultDeliveryPageSize
	}
	if limit < 1 || limit > maximumDeliveryPageSize {
		return DeliveryPage{}, domain.NewValidationError(
			"limit",
			"range",
			"must be between 1 and 100",
		)
	}
	deliveries, nextCursor, err := service.deliveryRepository.ListBySeller(
		ctx,
		sellerID,
		limit,
		strings.TrimSpace(cursor),
	)
	if err != nil {
		return DeliveryPage{}, err
	}
	views := make([]DeliveryView, len(deliveries))
	for index, delivery := range deliveries {
		views[index] = deliveryView(delivery)
	}
	return DeliveryPage{Items: views, NextCursor: nextCursor}, nil
}

// Redeliver authorizes and immediately reschedules one dead-letter delivery.
func (service *DeliveryService) Redeliver(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	deliveryID domain.ID,
) (DeliveryView, error) {
	if err := service.authorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return DeliveryView{}, err
	}
	delivery, err := service.deliveryRepository.Get(ctx, sellerID, deliveryID)
	if err != nil {
		return DeliveryView{}, err
	}
	expectedVersion := delivery.Version()
	if err := delivery.ScheduleRedelivery(domain.NewTimestamp(service.clock.Now())); err != nil {
		return DeliveryView{}, err
	}
	if err := service.deliveryRepository.Update(ctx, delivery, expectedVersion); err != nil {
		return DeliveryView{}, err
	}
	return deliveryView(delivery), nil
}

// recordDeliveryAttempt maps transport and HTTP outcomes into domain transitions.
func recordDeliveryAttempt(
	delivery *Delivery,
	attemptedAt domain.Timestamp,
	result WebhookSendResult,
	sendErr error,
) error {
	if sendErr != nil {
		return delivery.RecordFailure(
			attemptedAt,
			deliveryFailureForError(sendErr),
			0,
			"",
		)
	}
	if result.StatusCode >= http.StatusOK && result.StatusCode < http.StatusMultipleChoices {
		return delivery.RecordSuccess(
			attemptedAt,
			result.StatusCode,
			result.ResponseBodyHash,
		)
	}
	failureCode := DeliveryFailurePermanentResponse
	if retryableWebhookStatus(result.StatusCode) {
		failureCode = DeliveryFailureRetryableResponse
	}
	return delivery.RecordFailure(
		attemptedAt,
		failureCode,
		result.StatusCode,
		result.ResponseBodyHash,
	)
}

// deliveryFailureForError maps internal transport errors to safe public codes.
func deliveryFailureForError(err error) DeliveryFailureCode {
	switch {
	case errors.Is(err, ErrWebhookForbiddenTarget),
		errors.Is(err, ErrWebhookRedirectForbidden):
		return DeliveryFailureForbiddenTarget
	case errors.Is(err, ErrWebhookTimeout):
		return DeliveryFailureTimeout
	case errors.Is(err, ErrWebhookResponseTooLarge):
		return DeliveryFailureResponseTooLarge
	default:
		return DeliveryFailureUnavailable
	}
}

// retryableWebhookStatus reports whether an HTTP response permits automatic retry.
func retryableWebhookStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooEarly ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}

// subscriptionIncludesEvent reports whether a subscription selected an event type.
func subscriptionIncludesEvent(subscription Subscription, eventType EventType) bool {
	for _, subscribedType := range subscription.EventTypes() {
		if subscribedType == eventType {
			return true
		}
	}
	return false
}

// deliveryIdentity returns the unique subscription-event delivery claim.
func deliveryIdentity(subscriptionID domain.ID, eventID domain.ID) string {
	return subscriptionID.String() + "\x00" + eventID.String()
}
