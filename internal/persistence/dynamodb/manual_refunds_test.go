package dynamodb

import (
	"errors"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestManualRefundRepositoryUsesCreateOnlyDisputeRecord(t *testing.T) {
	t.Parallel()
	record := testDynamoManualRefundRecord(t)
	client := &fakeClient{}
	repository := NewManualRefundRecordRepository(client, "agentpay-dev")
	stored, created, err := repository.SaveIfAbsent(t.Context(), record)
	if err != nil || !created || stored != record {
		t.Fatalf("save = (%#v, %v, %v)", stored, created, err)
	}
	if client.putInput == nil || client.putInput.ConditionExpression == nil || *client.putInput.ConditionExpression != createItemCondition {
		t.Fatalf("put = %#v", client.putInput)
	}
	client.putErr = &types.ConditionalCheckFailedException{Message: stringPointer("exists")}
	client.getOutput = nil
	if _, _, err := repository.SaveIfAbsent(t.Context(), record); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("missing replay error = %v", err)
	}
}

func TestManualRefundRepositoryReturnsExistingRecordAfterConcurrentCreate(t *testing.T) {
	t.Parallel()

	existing := testDynamoManualRefundRecord(t)
	storedRecord, err := newStoredRecord(
		disputePartitionKey(existing.DisputeID.String()),
		manualRefundRecordSortKey,
		"manualRefundRecord",
		existing,
	)
	if err != nil {
		t.Fatal(err)
	}
	item, err := marshalStoredRecord(storedRecord)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{
		putErr:    &types.ConditionalCheckFailedException{Message: stringPointer("lost")},
		getOutput: &awssdk.GetItemOutput{Item: item},
	}
	repository := NewManualRefundRecordRepository(client, "agentpay-dev")
	retry := existing
	retry.RecordedAt = existing.RecordedAt.Add(time.Second)

	stored, created, err := repository.SaveIfAbsent(t.Context(), retry)
	if err != nil || created || stored != existing {
		t.Fatalf("SaveIfAbsent() = (%#v, %v, %v)", stored, created, err)
	}
}

func testDynamoManualRefundRecord(t *testing.T) disputes.ManualRefundRecord {
	t.Helper()
	return disputes.ManualRefundRecord{
		DisputeID:         mustDynamoID(t, "dsp_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.DisputeIDPrefix),
		TransactionID:     mustDynamoID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		SellerID:          mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		Amount:            domain.MustParseAmount("100"),
		Asset:             "USDC",
		Network:           "eip155:84532",
		Reference:         "0xrefund-reference",
		RecordedBy:        "seller-owner",
		RecordedAt:        testDynamoTime(),
		VerificationState: disputes.RefundVerificationSellerReported,
	}
}
