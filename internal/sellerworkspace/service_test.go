package sellerworkspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/transactions"
)

func TestOnboardingDerivesAuthoritativeStepsAndPersistsProgress(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	fixture.routes = []catalog.PaidRoute{fixture.route}
	fixture.destinations = []settlement.PaymentDestination{fixture.destination}
	fixture.credentials = []integrations.CredentialView{fixture.credential}
	fixture.entitlement = fixture.activeEntitlement
	fixture.transactions = []transactions.Transaction{fixture.transaction}

	first, err := fixture.service.Onboarding(context.Background(), fixture.principal)
	if err != nil {
		t.Fatal(err)
	}
	if first.Publication.Allowed {
		t.Fatal("publication must remain blocked before connector, sandbox, and preview checks")
	}
	assertStep(t, first, StepConnectorVerified, StepIncomplete)
	assertStep(t, first, StepSandboxPurchase, StepIncomplete)

	if err := fixture.service.RecordConnectorVerification(context.Background(), fixture.principal); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RecordSandboxPurchase(context.Background(), fixture.principal, fixture.transaction.TransactionID()); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RecordStorefrontPreview(context.Background(), fixture.principal); err != nil {
		t.Fatal(err)
	}

	resumed, err := fixture.service.Onboarding(context.Background(), fixture.principal)
	if err != nil {
		t.Fatal(err)
	}
	if !resumed.Complete || !resumed.Publication.Allowed {
		t.Fatalf("resumed onboarding = %#v, want complete and publishable", resumed)
	}
	if fixture.repository.putCalls != 4 {
		t.Fatalf("workspace writes = %d, want initial state plus three progress updates", fixture.repository.putCalls)
	}
}

func TestSandboxPurchaseMustBeAnAuthoritativeFulfilledSellerTransaction(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	fixture.transactions = nil
	if err := fixture.service.RecordSandboxPurchase(context.Background(), fixture.principal, fixture.transaction.TransactionID()); !errors.Is(err, ErrSandboxPurchaseInvalid) {
		t.Fatalf("RecordSandboxPurchase() error = %v", err)
	}
}

func TestPublicationGateFailsClosedUntilOnboardingIsComplete(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	if err := fixture.service.AuthorizePublication(context.Background(), fixture.seller.SellerID); !errors.Is(err, ErrPublicationBlocked) {
		t.Fatalf("AuthorizePublication() error = %v, want blocked", err)
	}
	fixture.routes = []catalog.PaidRoute{fixture.route}
	fixture.destinations = []settlement.PaymentDestination{fixture.destination}
	fixture.credentials = []integrations.CredentialView{fixture.credential}
	fixture.transactions = []transactions.Transaction{fixture.transaction}
	fixture.entitlement = fixture.activeEntitlement
	if err := fixture.service.RecordConnectorVerification(context.Background(), fixture.principal); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RecordSandboxPurchase(context.Background(), fixture.principal, fixture.transaction.TransactionID()); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RecordStorefrontPreview(context.Background(), fixture.principal); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.AuthorizePublication(context.Background(), fixture.seller.SellerID); err != nil {
		t.Fatalf("AuthorizePublication() error = %v", err)
	}
}

func TestOnboardingFailsClosedWhenEntitlementIsMissing(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	fixture.billingErr = billing.ErrSellerEntitlementNotFound

	view, err := fixture.service.Onboarding(context.Background(), fixture.principal)
	if err != nil {
		t.Fatal(err)
	}
	assertStep(t, view, StepSubscriptionActive, StepBlocked)
	if view.Publication.Allowed {
		t.Fatal("publication must fail closed when entitlement is unavailable")
	}
}

func TestCredentialIssuanceRequiresAuthoritativeMCPPrerequisites(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	fixture.destinations = []settlement.PaymentDestination{fixture.destination}
	fixture.entitlement = fixture.activeEntitlement

	if err := fixture.service.AuthorizeCredentialIssuance(context.Background(), fixture.principal.Subject, fixture.seller.SellerID); err != nil {
		t.Fatalf("AuthorizeCredentialIssuance() error = %v", err)
	}

	fixture.destination.Network = "unsupported:testnet"
	fixture.destinations = []settlement.PaymentDestination{fixture.destination}
	if err := fixture.service.AuthorizeCredentialIssuance(context.Background(), fixture.principal.Subject, fixture.seller.SellerID); !errors.Is(err, ErrCredentialIssuanceBlocked) {
		t.Fatalf("AuthorizeCredentialIssuance() error = %v, want blocked", err)
	}
}

func TestAuthenticatedConnectorVerificationIsIdempotent(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	if _, err := fixture.service.Onboarding(context.Background(), fixture.principal); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RecordAuthenticatedConnectorVerification(context.Background(), fixture.seller.SellerID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RecordAuthenticatedConnectorVerification(context.Background(), fixture.seller.SellerID); err != nil {
		t.Fatal(err)
	}
	if fixture.repository.putCalls != 2 {
		t.Fatalf("workspace writes = %d, want initial state plus one connector verification", fixture.repository.putCalls)
	}
}

func TestPublicationReadinessRequiresVerifiedServiceConnection(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	fixture.seller.Status = catalog.SellerStatusDraft
	fixture.seller.SigningSecretRef = ""
	fixture.routes = []catalog.PaidRoute{fixture.route}
	fixture.destinations = []settlement.PaymentDestination{fixture.destination}
	fixture.credentials = []integrations.CredentialView{fixture.credential}
	fixture.transactions = []transactions.Transaction{fixture.transaction}
	fixture.entitlement = fixture.activeEntitlement
	if err := fixture.service.RecordConnectorVerification(context.Background(), fixture.principal); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RecordSandboxPurchase(context.Background(), fixture.principal, fixture.transaction.TransactionID()); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RecordStorefrontPreview(context.Background(), fixture.principal); err != nil {
		t.Fatal(err)
	}
	view, err := fixture.service.Onboarding(context.Background(), fixture.principal)
	if err != nil {
		t.Fatal(err)
	}
	assertStep(t, view, StepServiceConnectionVerified, StepBlocked)
	if view.Publication.Allowed {
		t.Fatal("draft seller service must not be publishable")
	}
}

func TestDashboardBuildsBoundedSellerSummaries(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	fixture.routes = []catalog.PaidRoute{fixture.route}
	fixture.destinations = []settlement.PaymentDestination{fixture.destination}
	fixture.credentials = []integrations.CredentialView{fixture.credential}
	fixture.subscriptions = []notifications.SubscriptionView{{Status: notifications.SubscriptionStatusActive}}
	fixture.deliveries = []notifications.DeliveryView{{Status: notifications.DeliveryStatusDeadLetter}}
	fixture.transactions = []transactions.Transaction{fixture.transaction}
	fixture.events[fixture.transaction.TransactionID()] = []evidence.Event{{EventType: evidence.EventPaymentVerified}}
	fixture.entitlement = fixture.activeEntitlement

	overview, err := fixture.service.Dashboard(context.Background(), fixture.principal)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Products.Total != 1 || overview.Products.Published != 1 {
		t.Fatalf("product summary = %#v", overview.Products)
	}
	if overview.Transactions.Total != 1 || overview.Transactions.Fulfilled != 1 {
		t.Fatalf("transaction summary = %#v", overview.Transactions)
	}
	if overview.Evidence.EventCount != 1 || overview.Evidence.TransactionChains != 1 {
		t.Fatalf("evidence summary = %#v", overview.Evidence)
	}
	if overview.Webhooks.ActiveSubscriptions != 1 || overview.Webhooks.DeadLetterDeliveries != 1 {
		t.Fatalf("webhook summary = %#v", overview.Webhooks)
	}
	if overview.Credentials.Active != 1 || !overview.Billing.PortalAvailable {
		t.Fatalf("credential/billing summaries = %#v / %#v", overview.Credentials, overview.Billing)
	}
}

func TestCurrentSellerIsTakenFromPrincipalAndOwnershipIsRechecked(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	fixture.seller.OwnerSubject = "another-owner"

	_, err := fixture.service.Dashboard(context.Background(), fixture.principal)
	if !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("Dashboard() error = %v, want concealed ownership failure", err)
	}
}

func TestSettingsUpdateUsesOptimisticVersion(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	initial, err := fixture.service.Settings(context.Background(), fixture.principal)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := fixture.service.UpdateSettings(context.Background(), fixture.principal, UpdateSettingsRequest{
		SupportEmail:                "support@example.com",
		SecurityNotificationEmail:   "security@example.com",
		WebhookFailureNotifications: true,
		ExpectedVersion:             initial.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.SupportEmail != "support@example.com" || updated.Version != initial.Version+1 {
		t.Fatalf("updated settings = %#v", updated)
	}
	_, err = fixture.service.UpdateSettings(context.Background(), fixture.principal, UpdateSettingsRequest{ExpectedVersion: initial.Version})
	if !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("stale UpdateSettings() error = %v, want condition failure", err)
	}
}

func TestBillingPortalUsesCurrentSellerOnly(t *testing.T) {
	t.Parallel()
	fixture := newWorkspaceFixture(t)
	session, err := fixture.service.CreateBillingPortalSession(context.Background(), fixture.principal, "https://app.agentpay.example/dashboard/billing")
	if err != nil {
		t.Fatal(err)
	}
	if session.URL == "" || fixture.portalSellerID != fixture.seller.SellerID {
		t.Fatalf("portal session = %#v, seller = %s", session, fixture.portalSellerID)
	}
}

type workspaceFixture struct {
	service           *Service
	repository        *workspaceRepository
	principal         Principal
	seller            catalog.Seller
	route             catalog.PaidRoute
	destination       settlement.PaymentDestination
	credential        integrations.CredentialView
	transaction       transactions.Transaction
	activeEntitlement billing.SellerPlanResponse
	entitlement       billing.SellerPlanResponse
	billingErr        error
	routes            []catalog.PaidRoute
	destinations      []settlement.PaymentDestination
	credentials       []integrations.CredentialView
	transactions      []transactions.Transaction
	events            map[domain.ID][]evidence.Event
	subscriptions     []notifications.SubscriptionView
	deliveries        []notifications.DeliveryView
	portalSellerID    domain.ID
}

type workspaceRepository struct {
	state    WorkspaceState
	found    bool
	putCalls int
}

func newWorkspaceRepository() *workspaceRepository { return &workspaceRepository{} }

func (repository *workspaceRepository) Get(_ context.Context, sellerID domain.ID) (WorkspaceState, error) {
	if !repository.found || repository.state.SellerID != sellerID {
		return WorkspaceState{}, persistence.ErrNotFound
	}
	return repository.state, nil
}

func (repository *workspaceRepository) Put(_ context.Context, state WorkspaceState, expectedVersion uint64) error {
	if !repository.found {
		if expectedVersion != 0 {
			return persistence.ErrConditionFailed
		}
	} else if repository.state.Version != expectedVersion {
		return persistence.ErrConditionFailed
	}
	repository.state = state
	repository.found = true
	repository.putCalls++
	return nil
}

func newWorkspaceFixture(t *testing.T) *workspaceFixture {
	t.Helper()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	sellerID := mustWorkspaceID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	routeID := mustWorkspaceID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix)
	transactionID := mustWorkspaceID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
	intentID := mustWorkspaceID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix)
	credentialID := mustWorkspaceID(t, "key_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.CredentialIDPrefix)
	destinationID := mustWorkspaceID(t, "dst_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.PaymentDestinationIDPrefix)
	createdAt := domain.NewTimestamp(now.Add(-time.Hour))
	seller, err := catalog.NewSeller(catalog.SellerParams{SellerID: sellerID, OwnerSubject: "owner-123", Slug: "demo-seller", Name: "Demo seller", UpstreamBaseURL: "https://seller.example", CreatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := seller.Activate("secret/seller/demo", createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	route, err := catalog.NewPaidRoute(catalog.PaidRouteParams{RouteID: routeID, SellerID: sellerID, DisplayName: "Research report", ProductSlug: "research-report", Method: catalog.RouteMethodPost, PathPattern: "/reports", Description: "A report", MIMEType: "application/json", Amount: domain.MustParseAmount("100"), Asset: "USDC", Network: "eip155:84532", PayTo: "0x1111111111111111111111111111111111111111", UpstreamTimeoutSeconds: 10, CreatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := transactions.NewTransaction(transactions.TransactionParams{TransactionID: transactionID, IntentID: intentID, SellerID: sellerID, RouteID: routeID, BuyerID: "buyer", Amount: domain.MustParseAmount("100"), Asset: "USDC", Network: "eip155:84532", CreatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.RequirePayment(createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.VerifyPayment("payment-1", mustWorkspaceDigest(t), createdAt.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.FinalizePayment("payment-1", "0xtestnettransaction", createdAt.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkForwarded(createdAt.Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkFulfilled(200, mustWorkspaceDigest(t), transactions.ResponseSummary{ContentType: "application/json", ContentLength: 12}, createdAt.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	statusReason := billing.EntitlementStatusReason("")
	activeEntitlement := billing.SellerPlanResponse{Assignment: billing.SellerEntitlementView{SellerID: sellerID, PlanID: billing.PlanStarter, Status: billing.EntitlementStatusActive, AccessEndsAt: domain.NewTimestamp(now.Add(24 * time.Hour)), NetworkAccess: billing.NetworkAccessEnabled, DashboardAccess: billing.DashboardAccessFull, StatusReason: &statusReason}}
	fixture := &workspaceFixture{
		principal: Principal{Subject: "owner-123", SellerID: &sellerID, EmailVerified: true}, seller: seller, route: route,
		destination: settlement.PaymentDestination{DestinationID: destinationID, SellerID: sellerID, Asset: "USDC", Network: "eip155:84532", Status: settlement.PaymentDestinationStatusActive, VerifiedAt: pointerTimestamp(createdAt)},
		credential:  integrations.CredentialView{CredentialID: credentialID, SellerID: sellerID, CreatedAt: createdAt, UpdatedAt: createdAt, Version: 1},
		transaction: transaction, activeEntitlement: activeEntitlement, events: make(map[domain.ID][]evidence.Event),
	}
	fixture.repository = newWorkspaceRepository()
	fixture.service = NewService(Dependencies{
		Workspaces: fixture.repository, Sellers: fixture, Products: fixture, PaymentDestinations: fixture,
		Credentials: fixture, Transactions: fixture, Evidence: fixture, WebhookSubscriptions: fixture,
		WebhookDeliveries: fixture, Billing: fixture,
		BillingPortal: fixture, AccountVerification: StaticAccountVerification(true), Clock: domain.FixedClock{Value: now},
	})
	return fixture
}

func (fixture *workspaceFixture) GetSeller(context.Context, domain.ID) (catalog.Seller, error) {
	return fixture.seller, nil
}
func (fixture *workspaceFixture) ListRoutesBySeller(context.Context, domain.ID) ([]catalog.PaidRoute, error) {
	return fixture.routes, nil
}
func (fixture *workspaceFixture) ListPaymentDestinations(context.Context, domain.ID) ([]settlement.PaymentDestination, error) {
	return fixture.destinations, nil
}
func (fixture *workspaceFixture) ListCredentials(context.Context, domain.ID) ([]integrations.CredentialView, error) {
	return fixture.credentials, nil
}
func (fixture *workspaceFixture) ListTransactions(context.Context, domain.ID, int, string) ([]transactions.Transaction, *string, error) {
	return fixture.transactions, nil, nil
}
func (fixture *workspaceFixture) ListByTransaction(_ context.Context, transactionID domain.ID) ([]evidence.Event, error) {
	return fixture.events[transactionID], nil
}
func (fixture *workspaceFixture) ListWebhookSubscriptions(context.Context, domain.ID) ([]notifications.SubscriptionView, error) {
	return fixture.subscriptions, nil
}
func (fixture *workspaceFixture) ListWebhookDeliveries(context.Context, domain.ID, int, string) ([]notifications.DeliveryView, *string, error) {
	return fixture.deliveries, nil, nil
}
func (fixture *workspaceFixture) ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error) {
	return fixture.entitlement, fixture.billingErr
}
func (fixture *workspaceFixture) PlanState(context.Context, domain.ID) (ProviderPlanState, error) {
	return ProviderPlanState{Provider: "stripe", PortalAvailable: true}, nil
}
func (fixture *workspaceFixture) CreatePortalSession(_ context.Context, sellerID domain.ID, returnURL string) (BillingPortalSession, error) {
	fixture.portalSellerID = sellerID
	return BillingPortalSession{URL: returnURL + "?portal=1", ExpiresAt: domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 5, 0, 0, time.UTC))}, nil
}

func assertStep(t *testing.T, view OnboardingView, name StepName, status StepStatus) {
	t.Helper()
	for _, step := range view.Steps {
		if step.Name == name {
			if step.Status != status {
				t.Fatalf("step %s status = %s, want %s", name, step.Status, status)
			}
			return
		}
	}
	t.Fatalf("step %s was not returned", name)
}

func mustWorkspaceID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
func mustWorkspaceDigest(t *testing.T) intents.SHA256Digest {
	t.Helper()
	digest, err := intents.ParseSHA256Digest("0101010101010101010101010101010101010101010101010101010101010101")
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
func pointerTimestamp(value domain.Timestamp) *domain.Timestamp { return &value }
