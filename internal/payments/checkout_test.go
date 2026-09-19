package payments

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/transactions"
)

var errTestSubscriptionInactive = errors.New("subscription inactive")

// TestCheckoutServiceRecordsChallengeAndVerification verifies safe payment evidence.
func TestCheckoutServiceRecordsChallengeAndVerification(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	transactionRepository := memory.NewTransactionRepository()
	evidenceRepository := memory.NewEvidenceRepository()
	evidenceSigner, err := evidence.NewLocalHMACSigner(
		"local-evidence-key",
		[]byte(strings.Repeat("e", 32)),
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder := evidence.NewRecorder(
		evidenceRepository,
		domain.NewULIDGenerator(
			fixture.clock,
			strings.NewReader(strings.Repeat("r", 256)),
		),
		evidenceSigner,
		fixture.clock,
	)
	executor := &checkoutExecutor{}
	service := NewCheckoutService(
		fixture.service,
		NewMockAdapter(),
		transactionRepository,
		recorder,
		executor,
		fixture.clock,
	)
	request := CheckoutRequest{
		PaidRouteRequest: PaidRouteRequest{
			Slug:      fixture.seller.Slug,
			Method:    fixture.route.Method,
			ProxyPath: fixture.route.PathPattern,
			IntentID:  fixture.purchaseIntent.IntentID(),
			BuyerID:   fixture.purchaseIntent.BuyerID(),
		},
	}

	challenge, err := service.Execute(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Challenge == nil || challenge.Response != nil {
		t.Fatalf("challenge result = %#v", challenge)
	}

	request.PaymentProof = MockApprovedProof
	delivery, err := service.Execute(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if delivery.Response == nil || delivery.SettlementHeader == "" {
		t.Fatalf("delivery result = %#v", delivery)
	}
	transaction, err := transactionRepository.Get(t.Context(), delivery.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if transaction.PaymentFinality() != transactions.PaymentFinalityFinalized ||
		transaction.PaymentReference() == "" ||
		transaction.Reconciliation().Stage != transactions.ReconciliationStageFinalized {
		t.Fatalf("payment reconciliation = %#v", transaction.Reconciliation())
	}
	if transaction.ProductDisplayName() != fixture.purchaseIntent.ProductDisplayName() ||
		transaction.ProductSlug() != fixture.purchaseIntent.ProductSlug() ||
		transaction.PaymentDestinationID() != fixture.purchaseIntent.PaymentDestinationID() ||
		transaction.PaymentRail() != transactions.PaymentRailX402 {
		t.Fatalf("transaction commerce snapshot = %#v", transaction.Snapshot())
	}
	events, err := evidenceRepository.ListByTransaction(
		t.Context(),
		delivery.TransactionID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 ||
		events[0].EventType != evidence.EventPaymentChallenged ||
		events[1].EventType != evidence.EventPaymentVerified {
		t.Fatalf("events = %#v", events)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), MockApprovedProof) {
		t.Fatal("evidence contains the raw payment proof")
	}
}

func TestCheckoutServiceCopiesBrowserPurchaseAuthorityIntoTransaction(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	snapshot := fixture.purchaseIntent.Snapshot()
	snapshot.BuyerID = "browser:bps_01K5D09YJ0C0M7RJM4FWQ0K9H9"
	snapshot.PurchaseSessionID = "bps_01K5D09YJ0C0M7RJM4FWQ0K9H9"
	snapshot.PurchaseChannel = intents.PurchaseChannelBrowser
	snapshot.IntentHash = ""
	var err error
	fixture.purchaseIntent, err = intents.NewPurchaseIntent(intents.PurchaseIntentParams{
		IntentID: snapshot.IntentID, SellerID: snapshot.SellerID, RouteID: snapshot.RouteID,
		BuyerID: snapshot.BuyerID, PurchaseSessionID: snapshot.PurchaseSessionID, PurchaseChannel: snapshot.PurchaseChannel,
		ProductDisplayName: snapshot.ProductDisplayName, ProductSlug: snapshot.ProductSlug,
		PaymentDestinationID: snapshot.PaymentDestinationID, PayTo: snapshot.PayTo,
		RequestMethod: snapshot.RequestMethod, RequestPath: snapshot.RequestPath, RequestBodyHash: snapshot.RequestBodyHash,
		Amount: snapshot.Amount, Asset: snapshot.Asset, Network: snapshot.Network, MaximumAmount: snapshot.MaximumAmount,
		RequiresApproval: false, CreatedAt: snapshot.CreatedAt, ExpiresAt: snapshot.ExpiresAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.intentRepository = &paidRouteIntentRepository{purchaseIntent: fixture.purchaseIntent}
	transactionRepository := memory.NewTransactionRepository()
	service := NewCheckoutService(
		fixture.service,
		NewMockAdapter(),
		transactionRepository,
		newCheckoutEvidenceRecorder(t, fixture),
		&checkoutExecutor{},
		fixture.clock,
	)
	lifecycle := &recordingBrowserPurchaseLifecycle{}
	service.SetBrowserPurchaseLifecycle(lifecycle)
	request := CheckoutRequest{PaidRouteRequest: PaidRouteRequest{
		Slug: fixture.seller.Slug, Method: fixture.route.Method, ProxyPath: fixture.route.PathPattern,
		IntentID: fixture.purchaseIntent.IntentID(), BuyerID: fixture.purchaseIntent.BuyerID(),
	}}

	challenge, err := service.Execute(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := transactionRepository.Get(t.Context(), challenge.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if transaction.PurchaseChannel() != transactions.PurchaseChannelBrowser || transaction.PurchaseSessionID() != fixture.purchaseIntent.PurchaseSessionID() {
		t.Fatalf("transaction browser binding = (%s, %q)", transaction.PurchaseChannel(), transaction.PurchaseSessionID())
	}
	if lifecycle.claimedSession != browserpurchase.PurchaseSessionID(fixture.purchaseIntent.PurchaseSessionID()) ||
		lifecycle.claimedTransaction != transaction.TransactionID() {
		t.Fatalf("browser claim = (%q, %q)", lifecycle.claimedSession, lifecycle.claimedTransaction)
	}

	request.PaymentProof = MockApprovedProof
	if _, err := service.Execute(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if lifecycle.completedSession != lifecycle.claimedSession ||
		lifecycle.completedTransaction != transaction.TransactionID() ||
		lifecycle.completedNetwork != BaseSepoliaNetwork ||
		lifecycle.completedAddress == "" {
		t.Fatalf("browser completion = %#v", lifecycle)
	}
}

func TestCheckoutServiceRechecksEntitlementBeforeEveryPaymentBoundary(t *testing.T) {
	fixture := newPaidRouteFixture(t, false)
	authorizer := &recordingCommerceAuthorizer{denyAt: CommerceOperationSettlement}
	adapter := &recordingCheckoutAdapter{}
	executor := &checkoutExecutor{}
	service := NewCheckoutServiceWithCommerceAuthorizer(
		fixture.service,
		adapter,
		memory.NewTransactionRepository(),
		newCheckoutEvidenceRecorder(t, fixture),
		executor,
		fixture.clock,
		authorizer,
	)

	request := checkoutRequest(fixture)
	request.PaymentProof = MockApprovedProof
	_, err := service.Execute(t.Context(), request)
	if !errors.Is(err, errTestSubscriptionInactive) {
		t.Fatalf("Execute() error = %v, want subscription inactive", err)
	}
	if got := authorizer.operations; len(got) != 2 || got[0] != CommerceOperationVerification || got[1] != CommerceOperationSettlement {
		t.Fatalf("authorization operations = %#v", got)
	}
	if adapter.verifyCalls != 1 || adapter.settleCalls != 0 || executor.calls != 0 {
		t.Fatalf("calls = verify %d settle %d execute %d", adapter.verifyCalls, adapter.settleCalls, executor.calls)
	}
}

func TestCheckoutServiceFulfillsPaymentFinalizedBeforeCancellation(t *testing.T) {
	fixture := newPaidRouteFixture(t, false)
	authorizer := &recordingCommerceAuthorizer{}
	adapter := &recordingCheckoutAdapter{afterSettlement: func() {
		authorizer.denyAt = CommerceOperationExecution
	}}
	executor := &checkoutExecutor{}
	service := NewCheckoutServiceWithCommerceAuthorizer(
		fixture.service,
		adapter,
		memory.NewTransactionRepository(),
		newCheckoutEvidenceRecorder(t, fixture),
		executor,
		fixture.clock,
		authorizer,
	)

	request := checkoutRequest(fixture)
	request.PaymentProof = MockApprovedProof
	result, err := service.Execute(t.Context(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Response == nil || executor.calls != 1 {
		t.Fatalf("result = %#v, executor calls = %d", result, executor.calls)
	}
	for _, operation := range authorizer.operations {
		if operation == CommerceOperationExecution {
			t.Fatal("finalized payment was re-authorized after the cancellation boundary")
		}
	}
}

func TestCheckoutServiceAuthorizesChallengeBeforeIssuingIt(t *testing.T) {
	fixture := newPaidRouteFixture(t, false)
	authorizer := &recordingCommerceAuthorizer{denyAt: CommerceOperationChallenge}
	adapter := &recordingCheckoutAdapter{}
	service := NewCheckoutServiceWithCommerceAuthorizer(
		fixture.service,
		adapter,
		memory.NewTransactionRepository(),
		newCheckoutEvidenceRecorder(t, fixture),
		&checkoutExecutor{},
		fixture.clock,
		authorizer,
	)

	_, err := service.Execute(t.Context(), checkoutRequest(fixture))
	if !errors.Is(err, errTestSubscriptionInactive) {
		t.Fatalf("Execute() error = %v, want subscription inactive", err)
	}
	if adapter.challengeCalls != 0 {
		t.Fatalf("challenge calls = %d, want 0", adapter.challengeCalls)
	}
}

func TestCheckoutServiceRejectsBodyThatDoesNotMatchImmutableIntent(t *testing.T) {
	fixture := newPaidRouteFixture(t, false)
	adapter := &recordingCheckoutAdapter{}
	service := NewCheckoutService(
		fixture.service,
		adapter,
		memory.NewTransactionRepository(),
		newCheckoutEvidenceRecorder(t, fixture),
		&checkoutExecutor{},
		fixture.clock,
	)
	request := checkoutRequest(fixture)
	request.Body = []byte(`{"unexpected":true}`)
	request.ContentType = "application/json"

	_, err := service.Execute(t.Context(), request)
	if !errors.Is(err, ErrPaidRouteMismatch) {
		t.Fatalf("Execute() error = %v, want paid route mismatch", err)
	}
	if adapter.challengeCalls != 0 || adapter.verifyCalls != 0 || adapter.settleCalls != 0 {
		t.Fatalf("adapter calls = (%d, %d, %d)", adapter.challengeCalls, adapter.verifyCalls, adapter.settleCalls)
	}
}

func TestCheckoutServiceUsesAuthoritativeTransactionStateForFinalizedRetry(t *testing.T) {
	fixture := newPaidRouteFixture(t, false)
	adapter := &recordingCheckoutAdapter{}
	executor := &checkoutExecutor{}
	service := NewCheckoutService(
		fixture.service,
		adapter,
		memory.NewTransactionRepository(),
		newCheckoutEvidenceRecorder(t, fixture),
		executor,
		fixture.clock,
	)
	request := checkoutRequest(fixture)
	request.PaymentProof = MockApprovedProof
	if _, err := service.Execute(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(t.Context(), request); err != nil {
		t.Fatalf("finalized retry error = %v", err)
	}
	if adapter.verifyCalls != 1 || adapter.settleCalls != 1 || executor.calls != 2 {
		t.Fatalf("calls after replay = verify %d settle %d execute %d", adapter.verifyCalls, adapter.settleCalls, executor.calls)
	}
}

func TestCheckoutServiceRejectsSettlementForDifferentPaymentIdentifier(t *testing.T) {
	fixture := newPaidRouteFixture(t, false)
	adapter := &recordingCheckoutAdapter{settlementIdentifier: "payment-other"}
	executor := &checkoutExecutor{}
	service := NewCheckoutService(
		fixture.service,
		adapter,
		memory.NewTransactionRepository(),
		newCheckoutEvidenceRecorder(t, fixture),
		executor,
		fixture.clock,
	)
	request := checkoutRequest(fixture)
	request.PaymentProof = MockApprovedProof
	result, err := service.Execute(t.Context(), request)
	if !errors.Is(err, ErrPaymentRejected) {
		t.Fatalf("Execute() error = %v, want payment rejected", err)
	}
	if result.RecoveryAction != RecoveryActionStartNewCheckout {
		t.Fatalf("recovery action = %q", result.RecoveryAction)
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls = %d, want 0", executor.calls)
	}
}

func TestCheckoutServiceRejectedProofNeverSettlesOrFulfills(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	transactionRepository := memory.NewTransactionRepository()
	adapter := &recordingCheckoutAdapter{verifyError: ErrPaymentRejected}
	executor := &checkoutExecutor{}
	service := NewCheckoutService(
		fixture.service,
		adapter,
		transactionRepository,
		newCheckoutEvidenceRecorder(t, fixture),
		executor,
		fixture.clock,
	)
	request := checkoutRequest(fixture)
	request.PaymentProof = "rejected-proof"

	result, err := service.Execute(t.Context(), request)
	if !errors.Is(err, ErrPaymentRejected) {
		t.Fatalf("Execute() error = %v, want payment rejected", err)
	}
	if result.RecoveryAction != RecoveryActionSignFreshAuthorization {
		t.Fatalf("recovery action = %q", result.RecoveryAction)
	}
	if adapter.verifyCalls != 1 || adapter.settleCalls != 0 || executor.calls != 0 {
		t.Fatalf("calls = verify %d settle %d execute %d", adapter.verifyCalls, adapter.settleCalls, executor.calls)
	}

	transactionID, err := transactionIDForIntent(fixture.purchaseIntent.IntentID())
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := transactionRepository.Get(t.Context(), transactionID)
	if err != nil {
		t.Fatal(err)
	}
	if transaction.Status() != transactions.StatusPaymentRequired || transaction.PaymentIdentifier() != "" {
		t.Fatalf("transaction after rejection = %#v", transaction.Snapshot())
	}
}

func TestCheckoutServiceClassifiesVerificationUnavailableForSafeRetry(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	adapter := &recordingCheckoutAdapter{verifyError: ErrPaymentUnavailable}
	service := NewCheckoutService(
		fixture.service,
		adapter,
		memory.NewTransactionRepository(),
		newCheckoutEvidenceRecorder(t, fixture),
		&checkoutExecutor{},
		fixture.clock,
	)
	request := checkoutRequest(fixture)
	request.PaymentProof = MockApprovedProof

	result, err := service.Execute(t.Context(), request)
	if !errors.Is(err, ErrPaymentUnavailable) || result.RecoveryAction != RecoveryActionRetrySameRequest {
		t.Fatalf("Execute() = (%#v, %v)", result, err)
	}
	if adapter.verifyCalls != 1 || adapter.settleCalls != 0 {
		t.Fatalf("calls = verify %d settle %d", adapter.verifyCalls, adapter.settleCalls)
	}
}

func TestCheckoutServiceRejectsChangedProofAfterVerificationFailure(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	adapter := &recordingCheckoutAdapter{settleError: ErrPaymentUnavailable}
	executor := &checkoutExecutor{}
	service := NewCheckoutService(
		fixture.service,
		adapter,
		memory.NewTransactionRepository(),
		newCheckoutEvidenceRecorder(t, fixture),
		executor,
		fixture.clock,
	)
	request := checkoutRequest(fixture)
	request.PaymentProof = MockApprovedProof
	if _, err := service.Execute(t.Context(), request); !errors.Is(err, ErrPaymentUnavailable) {
		t.Fatalf("first Execute() error = %v, want payment unavailable", err)
	}

	request.PaymentProof = "changed-proof"
	if _, err := service.Execute(t.Context(), request); !errors.Is(err, ErrPaymentReplay) {
		t.Fatalf("replay Execute() error = %v, want payment replay", err)
	}
	if adapter.verifyCalls != 1 || adapter.settleCalls != 1 || executor.calls != 0 {
		t.Fatalf("calls after replay = verify %d settle %d execute %d", adapter.verifyCalls, adapter.settleCalls, executor.calls)
	}
}

func TestCheckoutServiceRetriesSameProofAfterUnknownSettlement(t *testing.T) {
	t.Parallel()

	fixture := newPaidRouteFixture(t, false)
	adapter := &recordingCheckoutAdapter{settleError: ErrPaymentUnavailable}
	executor := &checkoutExecutor{}
	service := NewCheckoutService(
		fixture.service,
		adapter,
		memory.NewTransactionRepository(),
		newCheckoutEvidenceRecorder(t, fixture),
		executor,
		fixture.clock,
	)
	request := checkoutRequest(fixture)
	request.PaymentProof = MockApprovedProof

	result, err := service.Execute(t.Context(), request)
	if !errors.Is(err, ErrPaymentUnavailable) || result.RecoveryAction != RecoveryActionRetrySamePayment {
		t.Fatalf("first Execute() = (%#v, %v)", result, err)
	}
	adapter.settleError = nil
	result, err = service.Execute(t.Context(), request)
	if err != nil {
		t.Fatalf("retry Execute() error = %v", err)
	}
	if result.Response == nil || adapter.verifyCalls != 1 || adapter.settleCalls != 2 || executor.calls != 1 {
		t.Fatalf("retry result/calls = %#v verify=%d settle=%d execute=%d", result, adapter.verifyCalls, adapter.settleCalls, executor.calls)
	}
}

func checkoutRequest(fixture paidRouteFixture) CheckoutRequest {
	return CheckoutRequest{PaidRouteRequest: PaidRouteRequest{
		Slug: fixture.seller.Slug, Method: fixture.route.Method,
		ProxyPath: fixture.route.PathPattern, IntentID: fixture.purchaseIntent.IntentID(),
		BuyerID: fixture.purchaseIntent.BuyerID(),
	}}
}

func newCheckoutEvidenceRecorder(t *testing.T, fixture paidRouteFixture) *evidence.Recorder {
	t.Helper()
	signer, err := evidence.NewLocalHMACSigner("local-evidence-key", []byte(strings.Repeat("e", 32)))
	if err != nil {
		t.Fatal(err)
	}
	return evidence.NewRecorder(
		memory.NewEvidenceRepository(),
		domain.NewULIDGenerator(fixture.clock, strings.NewReader(strings.Repeat("r", 256))),
		signer,
		fixture.clock,
	)
}

type recordingCommerceAuthorizer struct {
	operations []CommerceOperation
	denyAt     CommerceOperation
}

func (authorizer *recordingCommerceAuthorizer) AuthorizeCommerce(
	_ context.Context,
	_ domain.ID,
	operation CommerceOperation,
) error {
	authorizer.operations = append(authorizer.operations, operation)
	if operation == authorizer.denyAt {
		return errTestSubscriptionInactive
	}
	return nil
}

type recordingCheckoutAdapter struct {
	challengeCalls       int
	verifyCalls          int
	settleCalls          int
	afterSettlement      func()
	settlementIdentifier string
	verifyError          error
	settleError          error
}

func (adapter *recordingCheckoutAdapter) PaymentCapabilities() PaymentCapabilityCatalog {
	return NewMockAdapter().PaymentCapabilities()
}

func (adapter *recordingCheckoutAdapter) CreateChallenge(_ context.Context, requirements Requirements) (Challenge, error) {
	adapter.challengeCalls++
	return Challenge{Requirements: requirements, Header: "challenge"}, nil
}

func (adapter *recordingCheckoutAdapter) Verify(_ context.Context, _ string, _ Requirements) (VerificationResult, error) {
	adapter.verifyCalls++
	if adapter.verifyError != nil {
		return VerificationResult{}, adapter.verifyError
	}
	return VerificationResult{Valid: true, PaymentIdentifier: "payment-test"}, nil
}

func (adapter *recordingCheckoutAdapter) Settle(_ context.Context, _ string, _ Requirements) (SettlementResult, error) {
	adapter.settleCalls++
	if adapter.settleError != nil {
		return SettlementResult{}, adapter.settleError
	}
	if adapter.afterSettlement != nil {
		adapter.afterSettlement()
	}
	identifier := adapter.settlementIdentifier
	if identifier == "" {
		identifier = "payment-test"
	}
	return SettlementResult{Settled: true, PaymentIdentifier: identifier, PaymentReference: "0xsettled", ResponseHeader: "settled"}, nil
}

type checkoutExecutor struct{ calls int }

type recordingBrowserPurchaseLifecycle struct {
	claimedSession       browserpurchase.PurchaseSessionID
	claimedTransaction   domain.ID
	completedSession     browserpurchase.PurchaseSessionID
	completedTransaction domain.ID
	completedNetwork     string
	completedAddress     string
	completedAt          time.Time
}

func (lifecycle *recordingBrowserPurchaseLifecycle) ClaimTransaction(
	_ context.Context,
	purchaseSessionID browserpurchase.PurchaseSessionID,
	transactionID domain.ID,
) (browserpurchase.BrowserPurchaseSession, error) {
	lifecycle.claimedSession = purchaseSessionID
	lifecycle.claimedTransaction = transactionID
	return browserpurchase.BrowserPurchaseSession{PurchaseSessionID: purchaseSessionID}, nil
}

func (lifecycle *recordingBrowserPurchaseLifecycle) Complete(
	_ context.Context,
	purchaseSessionID browserpurchase.PurchaseSessionID,
	network string,
	address string,
	transactionID domain.ID,
	terminalAt time.Time,
) error {
	lifecycle.completedSession = purchaseSessionID
	lifecycle.completedTransaction = transactionID
	lifecycle.completedNetwork = network
	lifecycle.completedAddress = address
	lifecycle.completedAt = terminalAt
	return nil
}

// Execute returns a deterministic seller response for checkout tests.
func (executor *checkoutExecutor) Execute(
	context.Context,
	proxy.ExecutionRequest,
) (proxy.ForwardResponse, error) {
	executor.calls++
	return proxy.ForwardResponse{
		StatusCode:  200,
		Body:        []byte(`{"ok":true}`),
		ContentType: "application/json",
	}, nil
}
