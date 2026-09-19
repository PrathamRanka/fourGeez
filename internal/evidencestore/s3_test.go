package evidencestore

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
)

func TestS3RepositoryAppendsImmutableEvidenceObject(t *testing.T) {
	t.Parallel()
	client := &fakeS3Client{}
	repository, err := NewS3Repository(client, "agentpay-evidence")
	if err != nil {
		t.Fatal(err)
	}
	event := testEvidenceEvent()
	if err := repository.Append(t.Context(), event); err != nil {
		t.Fatal(err)
	}
	if client.putInput == nil || aws.ToString(client.putInput.Bucket) != "agentpay-evidence" || aws.ToString(client.putInput.Key) != "transactions/txn_01K5D09YJ0C0M7RJM4FWQ0K9H7/events/000001.json" || aws.ToString(client.putInput.IfNoneMatch) != "*" {
		t.Fatalf("PutObject input = %#v", client.putInput)
	}
}

func TestS3RepositoryListsEvidenceInSequenceOrder(t *testing.T) {
	t.Parallel()
	event := testEvidenceEvent()
	encoded, err := encodeEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeS3Client{
		listOutput: &awss3.ListObjectsV2Output{Contents: []types.Object{{Key: aws.String("transactions/txn_01K5D09YJ0C0M7RJM4FWQ0K9H7/events/000001.json")}}},
		objects:    map[string][]byte{"transactions/txn_01K5D09YJ0C0M7RJM4FWQ0K9H7/events/000001.json": encoded},
	}
	repository, err := NewS3Repository(client, "agentpay-evidence")
	if err != nil {
		t.Fatal(err)
	}
	events, err := repository.ListByTransaction(t.Context(), event.TransactionID)
	if err != nil || len(events) != 1 || events[0].EventID != event.EventID {
		t.Fatalf("ListByTransaction() = %#v, %v", events, err)
	}
}

type fakeS3Client struct {
	putInput   *awss3.PutObjectInput
	listOutput *awss3.ListObjectsV2Output
	objects    map[string][]byte
}

func (client *fakeS3Client) PutObject(_ context.Context, input *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	client.putInput = input
	_, _ = io.ReadAll(input.Body)
	return &awss3.PutObjectOutput{}, nil
}

func (client *fakeS3Client) ListObjectsV2(_ context.Context, _ *awss3.ListObjectsV2Input, _ ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error) {
	return client.listOutput, nil
}

func (client *fakeS3Client) GetObject(_ context.Context, input *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	return &awss3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(client.objects[aws.ToString(input.Key)]))}, nil
}

func testEvidenceEvent() evidence.Event {
	now := domain.NewTimestamp(time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC))
	digest, _ := intents.ParseSHA256Digest(strings.Repeat("a", 64))
	return evidence.Event{
		EventID: domain.ID("evt_01K5D09YJ0C0M7RJM4FWQ0K9H8"), TransactionID: domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		Sequence: 1, EventType: evidence.EventPaymentChallenged, ActorType: evidence.ActorSystem,
		Payload: map[string]any{"amount": "100"}, EventHash: digest, KMSKeyID: "kms-key", KMSSignature: "signature", CreatedAt: now,
	}
}
