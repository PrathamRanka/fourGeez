package settlement

import (
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

// TestNewPaymentDestinationValidatesPendingConfiguration verifies WAL-001 domain rules.
func TestNewPaymentDestinationValidatesPendingConfiguration(t *testing.T) {
	t.Parallel()

	validParameters := PaymentDestinationParams{
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
		CreatedAt: domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)),
	}

	destination, err := NewPaymentDestination(validParameters)
	if err != nil {
		t.Fatal(err)
	}
	if destination.Status != PaymentDestinationStatusPendingVerification {
		t.Fatalf("status = %q", destination.Status)
	}
	if destination.Version != 1 {
		t.Fatalf("version = %d, want 1", destination.Version)
	}

	testCases := []struct {
		name   string
		mutate func(*PaymentDestinationParams)
	}{
		{
			name: "missing asset",
			mutate: func(parameters *PaymentDestinationParams) {
				parameters.Asset = ""
			},
		},
		{
			name: "missing network",
			mutate: func(parameters *PaymentDestinationParams) {
				parameters.Network = ""
			},
		},
		{
			name: "missing address",
			mutate: func(parameters *PaymentDestinationParams) {
				parameters.Address = ""
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			parameters := validParameters
			testCase.mutate(&parameters)
			if _, err := NewPaymentDestination(parameters); err == nil {
				t.Fatal("NewPaymentDestination() error = nil")
			}
		})
	}
}

// mustSettlementID parses a stable identifier fixture.
func mustSettlementID(t *testing.T, value string, prefix domain.IDPrefix) domain.ID {
	t.Helper()

	identifier, err := domain.ParseID(value, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
