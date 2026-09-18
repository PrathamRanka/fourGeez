//go:build agentpay_dev

package devseed

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/analytics"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/sellerworkspace"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// TestLaunchReadyProfileSeedsEveryRequiredLocalScenario verifies LCH-008 coverage.
func TestLaunchReadyProfileSeedsEveryRequiredLocalScenario(t *testing.T) {
	t.Parallel()

	fixture := newSeedFixture(t)
	metadata, err := fixture.seeder.ResetAndSeed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ProfileName != ProfileLaunchReady ||
		metadata.LaunchReadySellerID.String() == "" ||
		metadata.IncompleteSellerID.String() == "" {
		t.Fatalf("metadata = %#v", metadata)
	}

	launchReadySeller, err := fixture.catalog.GetSeller(
		context.Background(),
		metadata.LaunchReadySellerID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if launchReadySeller.Status != catalog.SellerStatusActive ||
		launchReadySeller.SigningSecretRef == "" ||
		launchReadySeller.OwnerSubject != "local-seller" {
		t.Fatalf("launch-ready seller = %#v", launchReadySeller)
	}
	incompleteSeller, err := fixture.catalog.GetSeller(
		context.Background(),
		metadata.IncompleteSellerID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if incompleteSeller.Status != catalog.SellerStatusDraft {
		t.Fatalf("incomplete seller status = %q", incompleteSeller.Status)
	}

	routes, err := fixture.catalog.ListRoutesBySeller(
		context.Background(),
		metadata.LaunchReadySellerID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatalf("routes = %d, want 2", len(routes))
	}
	assertSeedRoutePolicy(t, routes, metadata)
	destinations, err := fixture.paymentDestinations.ListBySeller(
		context.Background(),
		metadata.LaunchReadySellerID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(destinations) != 2 {
		t.Fatalf("payment destinations = %d, want 2", len(destinations))
	}
	for _, destination := range destinations {
		if destination.Status != "active" || destination.VerifiedAt == nil {
			t.Fatalf("destination = %#v", destination)
		}
	}

	transactionPage, _, err := fixture.transactions.ListBySeller(
		context.Background(),
		metadata.LaunchReadySellerID,
		20,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	assertSeedTransactionStates(t, transactionPage)
	assertSeedEvidence(t, fixture, metadata)
	assertSeedWebhookAttempts(t, fixture, metadata)
	assertSeedAnalyticsPairs(t, metadata.LaunchReadySellerID, transactionPage)
	activeEntitlement, err := fixture.entitlements.Get(context.Background(), metadata.LaunchReadySellerID)
	if err != nil || activeEntitlement.Status() != billing.EntitlementStatusActive {
		t.Fatalf("launch-ready entitlement = (%#v, %v)", activeEntitlement.Snapshot(), err)
	}
	incompleteEntitlement, err := fixture.entitlements.Get(context.Background(), metadata.IncompleteSellerID)
	if err != nil || incompleteEntitlement.Status() != billing.EntitlementStatusSuspended {
		t.Fatalf("incomplete entitlement = (%#v, %v)", incompleteEntitlement.Snapshot(), err)
	}
	workspace, err := fixture.workspaces.Get(context.Background(), metadata.LaunchReadySellerID)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.ConnectorVerifiedAt == nil || workspace.SandboxPurchaseTransactionID == nil || workspace.StorefrontPreviewedAt == nil {
		t.Fatalf("launch-ready workspace = %#v", workspace)
	}
	credentials, err := fixture.credentials.ListBySeller(context.Background(), metadata.LaunchReadySellerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(credentials) != 1 || !credentials[0].HasScope(integrations.ScopePublish) || credentials[0].EntitlementEpoch() == 0 {
		t.Fatalf("launch-ready credentials = %#v", credentials)
	}
}

// TestResetEndpointClearsRuntimeChangesAndReappliesTheNamedProfile verifies disposal.
func TestResetEndpointClearsRuntimeChangesAndReappliesTheNamedProfile(t *testing.T) {
	t.Parallel()

	fixture := newSeedFixture(t)
	if _, err := fixture.seeder.ResetAndSeed(context.Background()); err != nil {
		t.Fatal(err)
	}
	extraSellerID := mustSeedID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9HZ",
		domain.SellerIDPrefix,
	)
	extraSeller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID:        extraSellerID,
		OwnerSubject:    "local-extra-owner",
		Slug:            "local-extra",
		Name:            "Local Extra",
		UpstreamBaseURL: "https://extra.example",
		CreatedAt:       fixedSeedTimestamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.catalog.CreateSeller(context.Background(), extraSeller); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	fixture.seeder.RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, ResetPath, nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("reset status = %d, body = %s", response.Code, response.Body.String())
	}
	if _, err := fixture.catalog.GetSeller(context.Background(), extraSellerID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("extra seller lookup error = %v, want not found", err)
	}
	transactionsPage, _, err := fixture.transactions.ListBySeller(
		context.Background(),
		fixture.seeder.Metadata().LaunchReadySellerID,
		20,
		"",
	)
	if err != nil || len(transactionsPage) != 4 {
		t.Fatalf("transactions after reset = (%d, %v), want (4, nil)", len(transactionsPage), err)
	}
}

// TestNewRejectsAnyProductionShapedSeedConfiguration protects the build-tag boundary.
func TestNewRejectsAnyProductionShapedSeedConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "production environment", mutate: func(config *Config) { config.Environment = "production" }},
		{name: "persistent repositories", mutate: func(config *Config) { config.RepositoryMode = "dynamodb" }},
		{name: "public bind", mutate: func(config *Config) { config.HTTPAddress = "0.0.0.0:8080" }},
		{name: "unknown profile", mutate: func(config *Config) { config.ProfileName = "other" }},
		{name: "weak webhook secret", mutate: func(config *Config) { config.WebhookSigningSecret = "short" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSeedRepositories(t)
			config := validSeedConfig()
			test.mutate(&config)
			if _, err := New(config, fixture.repositories, fixture.signer); err == nil {
				t.Fatal("New() error = nil, want guarded configuration rejection")
			}
		})
	}
}

type seedFixture struct {
	seeder              *Seeder
	catalog             *memory.CatalogRepository
	transactions        *memory.TransactionRepository
	evidence            *memory.EvidenceRepository
	entitlements        *memory.SellerEntitlementRepository
	paymentDestinations *memory.PaymentDestinationRepository
	webhookDeliveries   *memory.WebhookDeliveryRepository
	workspaces          *memory.SellerWorkspaceRepository
	credentials         *memory.IntegrationCredentialRepository
	signer              evidence.Signer
}

type seedRepositoriesFixture struct {
	repositories Repositories
	catalog      *memory.CatalogRepository
	transactions *memory.TransactionRepository
	evidence     *memory.EvidenceRepository
	entitlements *memory.SellerEntitlementRepository
	signer       evidence.Signer
}

func newSeedFixture(t *testing.T) seedFixture {
	t.Helper()
	repositories := newSeedRepositories(t)
	seeder, err := New(validSeedConfig(), repositories.repositories, repositories.signer)
	if err != nil {
		t.Fatal(err)
	}
	return seedFixture{
		seeder:              seeder,
		catalog:             repositories.catalog,
		transactions:        repositories.transactions,
		evidence:            repositories.evidence,
		entitlements:        repositories.entitlements,
		paymentDestinations: repositories.repositories.PaymentDestinations.(*memory.PaymentDestinationRepository),
		webhookDeliveries:   repositories.repositories.WebhookDeliveries.(*memory.WebhookDeliveryRepository),
		workspaces:          repositories.repositories.SellerWorkspaces.(*memory.SellerWorkspaceRepository),
		credentials:         repositories.repositories.IntegrationCredentials.(*memory.IntegrationCredentialRepository),
		signer:              repositories.signer,
	}
}

func newSeedRepositories(t *testing.T) seedRepositoriesFixture {
	t.Helper()
	catalogRepository := memory.NewCatalogRepository()
	intentRepository := memory.NewPurchaseIntentRepository()
	approvalRepository := memory.NewApprovalRepository()
	transactionRepository := memory.NewTransactionRepository()
	evidenceRepository := memory.NewEvidenceRepository()
	entitlementRepository := memory.NewSellerEntitlementRepository()
	disputeRepository := memory.NewDisputeRepository()
	paymentDestinationRepository := memory.NewPaymentDestinationRepository()
	webhookSubscriptionRepository := memory.NewWebhookSubscriptionRepository()
	webhookDeliveryRepository := memory.NewWebhookDeliveryRepository()
	webhookSecretStore := memory.NewWebhookSecretStore()
	sellerWorkspaceRepository := memory.NewSellerWorkspaceRepository()
	integrationCredentialRepository := memory.NewIntegrationCredentialRepository()
	idempotencyStore := memory.NewIdempotencyStore()
	resetter := memory.NewDevelopmentResetter(memory.DevelopmentRepositories{
		Catalog:                catalogRepository,
		PurchaseIntents:        intentRepository,
		Approvals:              approvalRepository,
		Transactions:           transactionRepository,
		Evidence:               evidenceRepository,
		SellerEntitlements:     entitlementRepository,
		Disputes:               disputeRepository,
		PaymentDestinations:    paymentDestinationRepository,
		WebhookSubscriptions:   webhookSubscriptionRepository,
		WebhookDeliveries:      webhookDeliveryRepository,
		WebhookSecrets:         webhookSecretStore,
		SellerWorkspaces:       sellerWorkspaceRepository,
		IntegrationCredentials: integrationCredentialRepository,
		Idempotency:            idempotencyStore,
	})
	signer, err := evidence.NewLocalHMACSigner(
		"local-seed-evidence-v1",
		[]byte(strings.Repeat("e", 32)),
	)
	if err != nil {
		t.Fatal(err)
	}
	return seedRepositoriesFixture{
		repositories: Repositories{
			Catalog:                catalogRepository,
			PurchaseIntents:        intentRepository,
			Transactions:           transactionRepository,
			Evidence:               evidenceRepository,
			Disputes:               disputeRepository,
			PaymentDestinations:    paymentDestinationRepository,
			WebhookSubscriptions:   webhookSubscriptionRepository,
			WebhookDeliveries:      webhookDeliveryRepository,
			WebhookSecrets:         webhookSecretStore,
			SellerWorkspaces:       sellerWorkspaceRepository,
			IntegrationCredentials: integrationCredentialRepository,
			SellerEntitlements:     entitlementRepository,
			Reset:                  resetter.Reset,
		},
		catalog:      catalogRepository,
		transactions: transactionRepository,
		evidence:     evidenceRepository,
		entitlements: entitlementRepository,
		signer:       signer,
	}
}

var _ sellerworkspace.Repository = (*memory.SellerWorkspaceRepository)(nil)

func validSeedConfig() Config {
	return Config{
		Environment:          "local",
		RepositoryMode:       "memory",
		HTTPAddress:          "127.0.0.1:8080",
		ProfileName:          ProfileLaunchReady,
		WebhookSigningSecret: strings.Repeat("w", 32),
	}
}

func assertSeedRoutePolicy(t *testing.T, routes []catalog.PaidRoute, metadata Metadata) {
	t.Helper()
	for _, route := range routes {
		if route.LifecycleStatus != catalog.RouteLifecyclePublished || !route.Enabled {
			t.Fatalf("route = %#v, want published", route)
		}
		if route.ApprovalThresholdAmount == nil {
			t.Fatalf("route %s has no approval threshold", route.RouteID)
		}
		switch route.RouteID {
		case metadata.BelowThresholdRouteID:
			if route.Amount.Compare(*route.ApprovalThresholdAmount) >= 0 {
				t.Fatalf("below-threshold route amount = %s, threshold = %s", route.Amount, *route.ApprovalThresholdAmount)
			}
		case metadata.ApprovalRequiredRouteID:
			if route.Amount.Compare(*route.ApprovalThresholdAmount) < 0 {
				t.Fatalf("approval route amount = %s, threshold = %s", route.Amount, *route.ApprovalThresholdAmount)
			}
		default:
			t.Fatalf("unexpected route %s", route.RouteID)
		}
	}
}

func assertSeedTransactionStates(t *testing.T, transactionPage []transactions.Transaction) {
	t.Helper()
	statuses := make(map[transactions.TransactionStatus]bool)
	for _, transaction := range transactionPage {
		statuses[transaction.Status()] = true
	}
	for _, status := range []transactions.TransactionStatus{
		transactions.StatusPaymentRequired,
		transactions.StatusFulfilled,
		transactions.StatusFailed,
		transactions.StatusDisputed,
	} {
		if !statuses[status] {
			t.Fatalf("transaction status %s is missing from %#v", status, statuses)
		}
	}
}

func assertSeedEvidence(t *testing.T, fixture seedFixture, metadata Metadata) {
	t.Helper()
	transactionService := transactions.NewService(
		fixture.transactions,
		fixture.evidence,
		fixture.signer,
		fixture.catalog,
	)
	validDetail, err := transactionService.Get(context.Background(), metadata.ValidEvidenceTransactionID)
	if err != nil || !validDetail.Evidence.Valid || len(validDetail.Evidence.Events) == 0 {
		t.Fatalf("valid evidence = (%#v, %v)", validDetail.Evidence, err)
	}
	invalidDetail, err := transactionService.Get(context.Background(), metadata.InvalidEvidenceTransactionID)
	if err != nil || invalidDetail.Evidence.Valid || len(invalidDetail.Evidence.Events) == 0 {
		t.Fatalf("invalid evidence = (%#v, %v)", invalidDetail.Evidence, err)
	}
}

func assertSeedWebhookAttempts(t *testing.T, fixture seedFixture, metadata Metadata) {
	t.Helper()
	deliveries, _, err := fixture.webhookDeliveries.ListBySeller(
		context.Background(),
		metadata.LaunchReadySellerID,
		20,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	statuses := make(map[notifications.DeliveryStatus]uint32)
	for _, delivery := range deliveries {
		statuses[delivery.Status()] = delivery.AttemptCount()
	}
	if statuses[notifications.DeliveryStatusDelivered] != 1 ||
		statuses[notifications.DeliveryStatusRetryScheduled] != 1 ||
		statuses[notifications.DeliveryStatusDeadLetter] < 2 {
		t.Fatalf("webhook attempts = %#v", statuses)
	}
}

func assertSeedAnalyticsPairs(
	t *testing.T,
	sellerID domain.ID,
	transactionPage []transactions.Transaction,
) {
	t.Helper()
	aggregates, err := analytics.NewService().AggregateSeller(sellerID, transactionPage)
	if err != nil {
		t.Fatal(err)
	}
	pairs := make(map[string]bool)
	for _, aggregate := range aggregates {
		if aggregate.RouteID == "" {
			pairs[aggregate.Asset+"/"+aggregate.Network] = true
		}
	}
	if !pairs["USDC/eip155:84532"] || !pairs["EURC/eip155:84532"] {
		t.Fatalf("analytics pairs = %#v", pairs)
	}
}

func mustSeedID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
