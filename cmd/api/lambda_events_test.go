package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/publicationops"
)

func TestHandlePublicationStreamProcessesOnlyOutboxRecordsAndReportsRetryableFailures(t *testing.T) {
	t.Parallel()

	first := publicationStreamRecord(t, "100", validPublicationEvent("a"))
	second := publicationStreamRecord(t, "200", validPublicationEvent("b"))
	nonOutbox := publicationStreamRecord(t, "300", validPublicationEvent("c"))
	nonOutbox.Change.NewImage["entity"] = events.NewStringAttribute("transaction")
	processor := &recordingPublicationProcessor{failEventID: validPublicationEvent("b").EventID}

	response := handlePublicationStream(t.Context(), events.DynamoDBEvent{
		Records: []events.DynamoDBEventRecord{first, second, nonOutbox},
	}, processor)

	if len(processor.events) != 2 {
		t.Fatalf("processed events = %d, want 2", len(processor.events))
	}
	if len(response.BatchItemFailures) != 1 || response.BatchItemFailures[0].ItemIdentifier != "200" {
		t.Fatalf("batch failures = %#v", response.BatchItemFailures)
	}
}

func TestDecodePublicationEventRejectsMalformedPayload(t *testing.T) {
	t.Parallel()

	record := publicationStreamRecord(t, "100", validPublicationEvent("a"))
	record.Change.NewImage["payload"] = events.NewBinaryAttribute([]byte("{"))
	if _, ok := decodePublicationEvent(record); ok {
		t.Fatal("malformed publication payload accepted")
	}
}

func publicationStreamRecord(t *testing.T, sequence string, event publicationops.Event) events.DynamoDBEventRecord {
	t.Helper()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return events.DynamoDBEventRecord{
		EventName: "INSERT",
		Change: events.DynamoDBStreamRecord{
			SequenceNumber: sequence,
			NewImage: map[string]events.DynamoDBAttributeValue{
				"entity":  events.NewStringAttribute("publicationOutbox"),
				"payload": events.NewBinaryAttribute(payload),
			},
		},
	}
}

func validPublicationEvent(suffix string) publicationops.Event {
	return publicationops.Event{
		SchemaVersion:    publicationops.SchemaVersionV1,
		EventID:          suffix + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		EventType:        publicationops.EventRouteChanged,
		SellerID:         "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		AggregateID:      "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		AggregateVersion: 2,
		OccurredAt:       domain.NewTimestamp(time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)),
	}
}

type recordingPublicationProcessor struct {
	events      []publicationops.Event
	failEventID string
}

func (processor *recordingPublicationProcessor) Process(_ context.Context, event publicationops.Event) error {
	processor.events = append(processor.events, event)
	if event.EventID == processor.failEventID {
		return errors.New("retry")
	}
	return nil
}
