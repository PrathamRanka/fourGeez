package dynamodb

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestManualRefundRepositoryUsesCreateOnlyDisputeRecord(t *testing.T) {
	t.Parallel()
	record := disputes.ManualRefundRecord{
		DisputeID:     mustDynamoID(t, "dsp_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.DisputeIDPrefix),
		TransactionID: mustDynamoID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		SellerID:      mustDynamoID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		Amount:        domain.MustParseAmount("100"), Asset: "USDC", Network: "eip155:84532",
		Reference: "0xrefund-reference", RecordedBy: "seller-owner", RecordedAt: testDynamoTime(),
	}
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
