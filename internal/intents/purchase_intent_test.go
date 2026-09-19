package intents

import (
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestNewPurchaseIntentFreezesExecutionFields(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	requestBodyHash, err := HashRequestBody([]byte(`{"topic":"agent commerce"}`), "application/json")
	if err != nil {
		t.Fatalf("HashRequestBody() error = %v", err)
	}
	params := PurchaseIntentParams{
		IntentID:             mustIntentID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:             mustIntentID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		RouteID:              mustIntentID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:              "agent_demo_123",
		ProductDisplayName:   "Research Report",
		ProductSlug:          "research-report",
		PaymentDestinationID: mustIntentID(t, "dst_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.PaymentDestinationIDPrefix),
		PayTo:                "0x1111111111111111111111111111111111111111",
		RequestMethod:        RequestMethodPost,
		RequestPath:          "/research/board",
		RequestBodyHash:      requestBodyHash,
		Amount:               domain.MustParseAmount("35000000"),
		Asset:                "test-usdc",
		Network:              "test-network",
		MaximumAmount:        domain.MustParseAmount("50000000"),
		RequiresApproval:     true,
		CreatedAt:            createdAt,
		ExpiresAt:            createdAt.Add(10 * time.Minute),
	}

	purchaseIntent, err := NewPurchaseIntent(params)
	if err != nil {
		t.Fatalf("NewPurchaseIntent() error = %v", err)
	}

	if purchaseIntent.Status() != PurchaseIntentStatusApprovalPending {
		t.Fatalf("Status() = %q, want %q", purchaseIntent.Status(), PurchaseIntentStatusApprovalPending)
	}
	if purchaseIntent.Amount() != params.Amount || purchaseIntent.MaximumAmount() != params.MaximumAmount {
		t.Fatal("purchase intent did not freeze the quoted and maximum amounts")
	}
	if purchaseIntent.IntentHash().String() == "" {
		t.Fatal("purchase intent hash is empty")
	}

	second, err := NewPurchaseIntent(params)
	if err != nil {
		t.Fatalf("second NewPurchaseIntent() error = %v", err)
	}
	if second.IntentHash() != purchaseIntent.IntentHash() {
		t.Fatalf("identical intents produced different hashes: first=%s second=%s", purchaseIntent.IntentHash(), second.IntentHash())
	}
}

func TestNewPurchaseIntentWithoutApprovalIsReady(t *testing.T) {
	t.Parallel()

	params := validPurchaseIntentParams(t)
	params.RequiresApproval = false

	purchaseIntent, err := NewPurchaseIntent(params)
	if err != nil {
		t.Fatalf("NewPurchaseIntent() error = %v", err)
	}
	if purchaseIntent.Status() != PurchaseIntentStatusReady {
		t.Fatalf("Status() = %q, want %q", purchaseIntent.Status(), PurchaseIntentStatusReady)
	}
}

func TestPurchaseIntentLifecycleIsExclusiveAndExpiryWins(t *testing.T) {
	t.Parallel()

	params := validPurchaseIntentParams(t)
	params.RequiresApproval = false

	cancelled, err := NewPurchaseIntent(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := cancelled.Cancel(params.CreatedAt.Add(time.Minute)); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if cancelled.Status() != PurchaseIntentStatusCancelled ||
		cancelled.CancellationReason() != CancellationReasonBuyerRequested ||
		cancelled.CancelledAt() == nil || cancelled.Version() != 2 {
		t.Fatalf("cancelled snapshot = %#v", cancelled.Snapshot())
	}
	if err := cancelled.Claim(params.CreatedAt.Add(2 * time.Minute)); err == nil {
		t.Fatal("Claim() accepted a cancelled intent")
	}

	expired, err := NewPurchaseIntent(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := expired.Expire(params.ExpiresAt); err != nil {
		t.Fatalf("Expire() error = %v", err)
	}
	if expired.Status() != PurchaseIntentStatusExpired || expired.Version() != 2 {
		t.Fatalf("expired snapshot = %#v", expired.Snapshot())
	}
	if err := expired.Cancel(params.ExpiresAt); err == nil {
		t.Fatal("Cancel() accepted an expired intent")
	}

	executed, err := NewPurchaseIntent(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := executed.Claim(params.CreatedAt.Add(time.Minute)); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if executed.Status() != PurchaseIntentStatusExecuted || executed.Version() != 2 {
		t.Fatalf("executed snapshot = %#v", executed.Snapshot())
	}
	if err := executed.Cancel(params.CreatedAt.Add(2 * time.Minute)); err == nil {
		t.Fatal("Cancel() accepted an executed intent")
	}
}

func TestPurchaseIntentResponseHasExactSingleProductTotal(t *testing.T) {
	t.Parallel()

	params := validPurchaseIntentParams(t)
	params.RequiresApproval = false
	purchaseIntent, err := NewPurchaseIntent(params)
	if err != nil {
		t.Fatal(err)
	}

	breakdown := purchaseIntent.Response().PriceBreakdown
	if breakdown.Calculation != PriceCalculationFixedSingleProduct ||
		breakdown.Quantity != 1 ||
		breakdown.UnitAmount != params.Amount ||
		breakdown.Subtotal != params.Amount ||
		!breakdown.Adjustments.IsZero() ||
		breakdown.Total != params.Amount ||
		breakdown.Asset != params.Asset || breakdown.Network != params.Network {
		t.Fatalf("price breakdown = %#v", breakdown)
	}
}

func TestNewPurchaseIntentValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*PurchaseIntentParams)
		wantField string
	}{
		{name: "intent ID prefix", mutate: func(params *PurchaseIntentParams) {
			params.IntentID = mustIntentID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
		}, wantField: "intentId"},
		{name: "seller ID prefix", mutate: func(params *PurchaseIntentParams) {
			params.SellerID = mustIntentID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
		}, wantField: "sellerId"},
		{name: "route ID prefix", mutate: func(params *PurchaseIntentParams) {
			params.RouteID = mustIntentID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
		}, wantField: "routeId"},
		{name: "buyer required", mutate: func(params *PurchaseIntentParams) { params.BuyerID = "" }, wantField: "buyerId"},
		{name: "method unsupported", mutate: func(params *PurchaseIntentParams) { params.RequestMethod = RequestMethod("DELETE") }, wantField: "requestMethod"},
		{name: "path invalid", mutate: func(params *PurchaseIntentParams) { params.RequestPath = "research" }, wantField: "requestPath"},
		{name: "zero amount", mutate: func(params *PurchaseIntentParams) { params.Amount = domain.MustParseAmount("0") }, wantField: "amount"},
		{name: "over maximum", mutate: func(params *PurchaseIntentParams) { params.MaximumAmount = domain.MustParseAmount("34999999") }, wantField: "maximumAmount"},
		{name: "asset required", mutate: func(params *PurchaseIntentParams) { params.Asset = "" }, wantField: "asset"},
		{name: "network required", mutate: func(params *PurchaseIntentParams) { params.Network = "" }, wantField: "network"},
		{name: "expiration not after creation", mutate: func(params *PurchaseIntentParams) { params.ExpiresAt = params.CreatedAt }, wantField: "expiresAt"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			params := validPurchaseIntentParams(t)
			test.mutate(&params)
			_, err := NewPurchaseIntent(params)
			assertIntentValidationField(t, err, test.wantField)
		})
	}
}

func TestPurchaseIntentHashChangesWithExecutionFields(t *testing.T) {
	t.Parallel()

	base := validPurchaseIntentParams(t)
	first, err := NewPurchaseIntent(base)
	if err != nil {
		t.Fatalf("first NewPurchaseIntent() error = %v", err)
	}

	changed := base
	changed.Amount = domain.MustParseAmount("35000001")
	second, err := NewPurchaseIntent(changed)
	if err != nil {
		t.Fatalf("second NewPurchaseIntent() error = %v", err)
	}
	if first.IntentHash() == second.IntentHash() {
		t.Fatal("changing the frozen amount did not change the intent hash")
	}
}

func validPurchaseIntentParams(t *testing.T) PurchaseIntentParams {
	t.Helper()
	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	requestBodyHash, err := HashRequestBody([]byte(`{"topic":"agent commerce"}`), "application/json")
	if err != nil {
		t.Fatalf("HashRequestBody() error = %v", err)
	}
	return PurchaseIntentParams{
		IntentID:             mustIntentID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:             mustIntentID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		RouteID:              mustIntentID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:              "agent_demo_123",
		ProductDisplayName:   "Research Report",
		ProductSlug:          "research-report",
		PaymentDestinationID: mustIntentID(t, "dst_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.PaymentDestinationIDPrefix),
		PayTo:                "0x1111111111111111111111111111111111111111",
		RequestMethod:        RequestMethodPost,
		RequestPath:          "/research/board",
		RequestBodyHash:      requestBodyHash,
		Amount:               domain.MustParseAmount("35000000"),
		Asset:                "test-usdc",
		Network:              "test-network",
		MaximumAmount:        domain.MustParseAmount("50000000"),
		RequiresApproval:     true,
		CreatedAt:            createdAt,
		ExpiresAt:            createdAt.Add(10 * time.Minute),
	}
}

func mustIntentID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatalf("ParseID(%q) error = %v", raw, err)
	}
	return identifier
}

func assertIntentValidationField(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error for %q", field)
	}

	var validationErrors domain.ValidationErrors
	if errors.As(err, &validationErrors) {
		if !validationErrors.HasField(field) {
			t.Fatalf("validation errors = %#v, want field %q", validationErrors, field)
		}
		return
	}

	var validationError domain.ValidationError
	if errors.As(err, &validationError) && validationError.Field == field {
		return
	}
	t.Fatalf("error = %#v, want validation error for %q", err, field)
}
