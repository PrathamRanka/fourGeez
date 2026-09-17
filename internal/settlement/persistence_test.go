package settlement

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

// TestPaymentDestinationSnapshotPersistsPrivateChallengeHash verifies API hiding and storage retention.
func TestPaymentDestinationSnapshotPersistsPrivateChallengeHash(t *testing.T) {
	t.Parallel()

	destination, err := NewPaymentDestination(PaymentDestinationParams{
		DestinationID: mustSettlementID(
			t,
			"dst_01K5D09YJ0C0M7RJM4FWQ0K9H8",
			domain.PaymentDestinationIDPrefix,
		),
		SellerID: mustSettlementID(
			t,
			"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
			domain.SellerIDPrefix,
		),
		Asset:     "USDC",
		Network:   "eip155:84532",
		Address:   "0x1111111111111111111111111111111111111111",
		CreatedAt: domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatal(err)
	}
	destination.ChallengeHash = strings.Repeat("a", 64)
	expiresAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 10, 0, 0, time.UTC))
	destination.ChallengeExpiresAt = &expiresAt

	apiPayload, err := json.Marshal(destination)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(apiPayload), "challengeHash") {
		t.Fatalf("API payload exposed challenge hash: %s", apiPayload)
	}
	persistencePayload, err := json.Marshal(destination.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(persistencePayload), destination.ChallengeHash) {
		t.Fatalf("persistence payload omitted challenge hash: %s", persistencePayload)
	}
	var snapshot PaymentDestinationSnapshot
	if err := json.Unmarshal(persistencePayload, &snapshot); err != nil {
		t.Fatal(err)
	}
	restored, err := RestorePaymentDestination(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, destination) {
		t.Fatalf("restored destination = %#v, want %#v", restored, destination)
	}
}
