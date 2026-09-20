package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/fourgeez/agentpay/internal/publicationops"
)

type publicationEventProcessor interface {
	Process(context.Context, publicationops.Event) error
}

func startLambdaRuntime(handler http.Handler, processor publicationEventProcessor) {
	adapter := httpadapter.NewV2(handler)
	lambda.Start(func(ctx context.Context, raw json.RawMessage) (any, error) {
		var source struct {
			Records []struct {
				EventSource string `json:"eventSource"`
			} `json:"Records"`
		}
		if err := json.Unmarshal(raw, &source); err != nil {
			return nil, err
		}
		if len(source.Records) > 0 && source.Records[0].EventSource == "aws:dynamodb" {
			var stream events.DynamoDBEvent
			if err := json.Unmarshal(raw, &stream); err != nil {
				return nil, err
			}
			return handlePublicationStream(ctx, stream, processor), nil
		}

		var request events.APIGatewayV2HTTPRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		return adapter.ProxyWithContext(ctx, request)
	})
}

func handlePublicationStream(
	ctx context.Context,
	stream events.DynamoDBEvent,
	processor publicationEventProcessor,
) events.DynamoDBEventResponse {
	response := events.DynamoDBEventResponse{BatchItemFailures: []events.DynamoDBBatchItemFailure{}}
	for _, record := range stream.Records {
		if !isPublicationOutboxRecord(record) {
			continue
		}
		event, ok := decodePublicationEvent(record)
		if !ok || processor == nil || processor.Process(ctx, event) != nil {
			identifier := record.Change.SequenceNumber
			if identifier == "" {
				identifier = record.EventID
			}
			response.BatchItemFailures = append(response.BatchItemFailures, events.DynamoDBBatchItemFailure{ItemIdentifier: identifier})
			slog.Error("publication outbox processing failed", "sequenceNumber", identifier)
		}
	}
	return response
}

func isPublicationOutboxRecord(record events.DynamoDBEventRecord) bool {
	if record.EventName != string(events.DynamoDBOperationTypeInsert) {
		return false
	}
	entity, ok := record.Change.NewImage["entity"]
	return ok && entity.DataType() == events.DataTypeString && entity.String() == "publicationOutbox"
}

func decodePublicationEvent(record events.DynamoDBEventRecord) (publicationops.Event, bool) {
	if !isPublicationOutboxRecord(record) {
		return publicationops.Event{}, false
	}
	payload, ok := record.Change.NewImage["payload"]
	if !ok || payload.DataType() != events.DataTypeBinary {
		return publicationops.Event{}, false
	}
	var event publicationops.Event
	if err := json.Unmarshal(payload.Binary(), &event); err != nil {
		return publicationops.Event{}, false
	}
	return event, true
}
