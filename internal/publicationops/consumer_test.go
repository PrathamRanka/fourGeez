package publicationops

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestConsumerInvalidatesThenRefreshesAndRecordsCompletion(t *testing.T) {
	t.Parallel()
	order := make([]string, 0, 3)
	consumer := NewConsumer(
		&recordingInvalidator{order: &order},
		&recordingRefresher{order: &order},
		&recordingCompletionStore{order: &order},
	)
	event := validEvent()

	if err := consumer.Process(t.Context(), event); err != nil {
		t.Fatal(err)
	}
	if got := join(order); got != "invalidate,refresh,complete" {
		t.Fatalf("processing order = %q", got)
	}
}

func TestConsumerRetriesWithoutCompletingAfterRefreshFailure(t *testing.T) {
	t.Parallel()
	order := make([]string, 0, 2)
	wantErr := errors.New("signing unavailable")
	consumer := NewConsumer(
		&recordingInvalidator{order: &order},
		&recordingRefresher{order: &order, err: wantErr},
		&recordingCompletionStore{order: &order},
	)

	if err := consumer.Process(t.Context(), validEvent()); !errors.Is(err, wantErr) {
		t.Fatalf("Process() error = %v", err)
	}
	if got := join(order); got != "invalidate,refresh" {
		t.Fatalf("processing order = %q", got)
	}
}

func TestConsumerTreatsCompletedReplayAsIdempotent(t *testing.T) {
	t.Parallel()
	order := make([]string, 0, 1)
	completion := &recordingCompletionStore{order: &order, completed: true}
	consumer := NewConsumer(&recordingInvalidator{order: &order}, &recordingRefresher{order: &order}, completion)

	if err := consumer.Process(t.Context(), validEvent()); err != nil {
		t.Fatal(err)
	}
	if len(order) != 0 {
		t.Fatalf("completed replay performed side effects: %#v", order)
	}
}

func TestConsumerRejectsMalformedOutboxEvent(t *testing.T) {
	t.Parallel()
	consumer := NewConsumer(&recordingInvalidator{}, &recordingRefresher{}, &recordingCompletionStore{})
	event := validEvent()
	event.SchemaVersion = "unknown"
	if err := consumer.Process(t.Context(), event); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Process() error = %v", err)
	}
}

func validEvent() Event {
	return Event{
		SchemaVersion:    SchemaVersionV1,
		EventID:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		EventType:        EventRouteChanged,
		SellerID:         "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		AggregateID:      "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		AggregateVersion: 2,
		OccurredAt:       domain.NewTimestamp(time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)),
	}
}

type recordingInvalidator struct {
	order *[]string
	err   error
}

func (recorder *recordingInvalidator) InvalidateSellerPublication(context.Context, string, uint64) error {
	if recorder.order != nil {
		*recorder.order = append(*recorder.order, "invalidate")
	}
	return recorder.err
}

type recordingRefresher struct {
	order *[]string
	err   error
}

func (recorder *recordingRefresher) RefreshSellerPublication(context.Context, string) error {
	if recorder.order != nil {
		*recorder.order = append(*recorder.order, "refresh")
	}
	return recorder.err
}

type recordingCompletionStore struct {
	order     *[]string
	completed bool
}

func (store *recordingCompletionStore) Completed(context.Context, string) (bool, error) {
	return store.completed, nil
}

func (store *recordingCompletionStore) Complete(context.Context, Event) error {
	if store.order != nil {
		*store.order = append(*store.order, "complete")
	}
	store.completed = true
	return nil
}

func join(values []string) string {
	result := ""
	for index, value := range values {
		if index > 0 {
			result += ","
		}
		result += value
	}
	return result
}
