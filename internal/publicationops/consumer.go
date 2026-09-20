package publicationops

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
)

const SchemaVersionV1 = "agentpay.publication-outbox.v1"

type EventType string

const (
	EventRouteChanged       EventType = "route.changed"
	EventEntitlementChanged EventType = "entitlement.changed"
)

var (
	ErrInvalidEvent = errors.New("invalid publication outbox event")
	hexDigest       = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type Event struct {
	SchemaVersion    string           `json:"schemaVersion"`
	EventID          string           `json:"eventId"`
	EventType        EventType        `json:"eventType"`
	SellerID         string           `json:"sellerId"`
	AggregateID      string           `json:"aggregateId"`
	AggregateVersion uint64           `json:"aggregateVersion"`
	OccurredAt       domain.Timestamp `json:"occurredAt"`
}

type CacheInvalidator interface {
	InvalidateSellerPublication(context.Context, string, uint64) error
}

type Refresher interface {
	RefreshSellerPublication(context.Context, string) error
}

type CompletionStore interface {
	Completed(context.Context, string) (bool, error)
	Complete(context.Context, Event) error
}

type Consumer struct {
	invalidator CacheInvalidator
	refresher   Refresher
	completions CompletionStore
}

func NewConsumer(invalidator CacheInvalidator, refresher Refresher, completions CompletionStore) *Consumer {
	return &Consumer{invalidator: invalidator, refresher: refresher, completions: completions}
}

func (consumer *Consumer) Process(ctx context.Context, event Event) error {
	if err := validateEvent(event); err != nil {
		return err
	}
	if consumer.invalidator == nil || consumer.refresher == nil || consumer.completions == nil {
		return errors.New("publication consumer dependencies are unavailable")
	}
	completed, err := consumer.completions.Completed(ctx, event.EventID)
	if err != nil {
		return err
	}
	if completed {
		return nil
	}
	if err := consumer.invalidator.InvalidateSellerPublication(ctx, event.SellerID, event.AggregateVersion); err != nil {
		return err
	}
	if err := consumer.refresher.RefreshSellerPublication(ctx, event.SellerID); err != nil {
		return err
	}
	return consumer.completions.Complete(ctx, event)
}

func validateEvent(event Event) error {
	if event.SchemaVersion != SchemaVersionV1 || !hexDigest.MatchString(event.EventID) ||
		(event.EventType != EventRouteChanged && event.EventType != EventEntitlementChanged) ||
		!strings.HasPrefix(event.SellerID, "sel_") || strings.TrimSpace(event.AggregateID) == "" ||
		event.AggregateVersion == 0 || event.OccurredAt.Time().IsZero() {
		return ErrInvalidEvent
	}
	return nil
}
