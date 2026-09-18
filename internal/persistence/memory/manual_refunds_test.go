package memory

import (
	"errors"
	"testing"

	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
)

func TestManualRefundRepositoryIsIdempotentAndRejectsChangedReplay(t *testing.T) {
	t.Parallel()
	repository := NewManualRefundRecordRepository()
	record := testManualRefundRecord(t)
	stored, created, err := repository.SaveIfAbsent(t.Context(), record)
	if err != nil || !created || stored != record {
		t.Fatalf("first save = (%#v, %v, %v)", stored, created, err)
	}
	stored, created, err = repository.SaveIfAbsent(t.Context(), record)
	if err != nil || created || stored != record {
		t.Fatalf("replay = (%#v, %v, %v)", stored, created, err)
	}
	changed := record
	changed.Reference = "different"
	if _, _, err := repository.SaveIfAbsent(t.Context(), changed); !errors.Is(err, disputes.ErrRemediationConflict) {
		t.Fatalf("changed replay error = %v", err)
	}
}

func testManualRefundRecord(t *testing.T) disputes.ManualRefundRecord {
	t.Helper()
	return disputes.ManualRefundRecord{
		DisputeID:     mustID(t, "dsp_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.DisputeIDPrefix),
		TransactionID: mustID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix),
		SellerID:      mustID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		Amount:        domain.MustParseAmount("100"), Asset: "USDC", Network: "eip155:84532",
		Reference: "0xrefund-reference", RecordedBy: "seller-owner", RecordedAt: testTime(),
	}
}
