package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/fourgeez/agentpay/internal/analytics"
	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/integrations/analyzer"
	"github.com/fourgeez/agentpay/internal/integrations/discovery"
	"github.com/fourgeez/agentpay/internal/integrations/mcpserver"
	"github.com/fourgeez/agentpay/internal/integrations/sandbox"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/operations"
	"github.com/fourgeez/agentpay/internal/payments"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/realtime"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// main starts the local AgentPay HTTP API.
func main() {
	addr := os.Getenv("AGENTPAY_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()

	catalogRepository := memory.NewCatalogRepository()
	intentRepository := memory.NewPurchaseIntentRepository()
	approvalRepository := memory.NewApprovalRepository()
	transactionRepository := memory.NewTransactionRepository()
	evidenceRepository := memory.NewEvidenceRepository()
	disputeRepository := memory.NewDisputeRepository()
	integrationCredentialRepository := memory.NewIntegrationCredentialRepository()
	paymentDestinationRepository := memory.NewPaymentDestinationRepository()
	webhookSubscriptionRepository := memory.NewWebhookSubscriptionRepository()
	webhookDeliveryRepository := memory.NewWebhookDeliveryRepository()
	sellerPlanRepository := memory.NewSellerPlanRepository()
	usageMeterEventRepository := memory.NewUsageMeterEventRepository()
	quotaCounterRepository := memory.NewQuotaCounterRepository()
	auditEventRepository := memory.NewAuditEventRepository()
	webhookSecretStore := memory.NewWebhookSecretStore()
	idempotencyStore := memory.NewIdempotencyStore()
	idGenerator := domain.NewULIDGenerator(nil, nil)
	clock := domain.SystemClock{}
	auditAppender := audit.NewAppender(
		auditEventRepository,
		idGenerator,
		clock,
	)
	sellerSigner := proxy.NewHMACSigner(
		proxy.NewLocalSecretProvider(
			[]byte(os.Getenv("AGENTPAY_LOCAL_SELLER_SIGNING_SECRET")),
		),
		clock,
	)
	sellerForwarder := proxy.NewForwarder(nil)
	sandboxService := sandbox.NewService(
		catalogRepository,
		idGenerator,
		sellerSigner,
		sellerForwarder,
		clock,
	)
	catalogService := catalog.NewService(
		catalogRepository,
		idGenerator,
		clock,
		auditAppender,
	)
	catalogController := catalog.NewHTTPController(catalogService, idempotencyStore)
	catalogController.RegisterRoutes(mux)
	audit.NewHTTPController(
		audit.NewService(
			auditEventRepository,
			catalogService,
			idGenerator,
			clock,
		),
	).RegisterRoutes(mux)
	billingService := billing.NewService(
		sellerPlanRepository,
		catalogService,
		clock,
	)
	quotaService := operations.NewService(
		quotaCounterRepository,
		billingService,
		clock,
	)
	catalogService.SetQuotaEnforcer(quotaService)
	billing.NewHTTPController(billingService).RegisterRoutes(mux)
	usageService := billing.NewUsageService(
		usageMeterEventRepository,
		billingService,
		transactionRepository,
		catalogService,
		idGenerator,
		clock,
	)
	billing.NewUsageHTTPController(usageService).RegisterRoutes(mux)
	settlementService := settlement.NewService(
		paymentDestinationRepository,
		catalogService,
		idGenerator,
		settlement.NewSecureOwnershipNonceGenerator(nil),
		settlement.NewEVMPersonalSignOwnershipVerifier(),
		clock,
		auditAppender,
	)
	settlement.NewHTTPController(
		settlementService,
		idempotencyStore,
	).RegisterRoutes(mux)
	integrationService := integrations.NewService(
		integrationCredentialRepository,
		catalogService,
		idGenerator,
		integrations.NewSecureTokenGenerator(nil),
		clock,
		auditAppender,
	)
	integrations.NewHTTPController(
		integrationService,
		idempotencyStore,
	).RegisterRoutes(mux)
	notificationService := notifications.NewService(
		webhookSubscriptionRepository,
		catalogService,
		idGenerator,
		notifications.NewSecureSecretGenerator(nil),
		webhookSecretStore,
		clock,
		auditAppender,
	)
	notificationService.SetQuotaEnforcer(quotaService)
	notifications.NewHTTPController(
		notificationService,
		idempotencyStore,
	).RegisterRoutes(mux)
	deliveryService := notifications.NewDeliveryService(
		webhookDeliveryRepository,
		webhookSubscriptionRepository,
		catalogService,
		idGenerator,
		notifications.NewHMACEventSigner(webhookSecretStore, clock),
		notifications.NewWebhookSender(nil),
		clock,
	)
	deliveryService.SetQuotaEnforcer(quotaService)
	notifications.NewDeliveryHTTPController(
		deliveryService,
		idempotencyStore,
	).RegisterRoutes(mux)
	mcpController := mcpserver.NewHTTPController(
		integrationService,
		mcpserver.NewService(
			catalogRepository,
			transactionRepository,
		),
		mcpserver.NewMutationService(
			catalogService,
			idempotencyStore,
			clock,
			sandboxService,
			auditAppender,
		),
		analyzer.NewService(),
	)
	mcpController.SetQuotaEnforcer(quotaService)
	mcpController.SetDiscoveryValidator(discovery.NewService())
	mcpController.RegisterRoutes(mux)
	intentService := intents.NewService(
		intentRepository,
		catalogRepository,
		idGenerator,
		clock,
	)
	intentController := intents.NewHTTPController(intentService, idempotencyStore)
	intentController.RegisterRoutes(mux)
	approvalTokenSigner, err := approvals.NewApprovalTokenSigner(
		[]byte(os.Getenv("AGENTPAY_LOCAL_APPROVAL_TOKEN_SECRET")),
	)
	if err != nil {
		slog.Error("invalid approval token secret", "error", err)
		os.Exit(1)
	}
	approvalService := approvals.NewService(
		approvalRepository,
		intentRepository,
		idGenerator,
		approvals.NewSecureTokenGenerator(nil),
		approvalTokenSigner,
		clock,
		os.Getenv("AGENTPAY_PUBLIC_BASE_URL"),
	)
	realtimeHub := realtime.NewLocalHub()
	realtimeService := realtime.NewService(
		realtimeHub,
		approvalService,
		realtimeHub,
		idGenerator,
		clock,
	)
	approvalService.SetEventPublisher(realtimeService)
	approvalController := approvals.NewHTTPController(
		approvalService,
		idempotencyStore,
	)
	approvalController.RegisterRoutes(mux)
	realtimeController := realtime.NewHTTPController(
		realtime.NewController(realtimeService),
		realtimeHub,
		os.Getenv("AGENTPAY_WEB_ORIGIN"),
	)
	realtimeController.RegisterRoutes(mux)
	evidenceSigner, err := evidence.NewLocalHMACSigner(
		os.Getenv("AGENTPAY_LOCAL_EVIDENCE_KEY_ID"),
		[]byte(os.Getenv("AGENTPAY_LOCAL_EVIDENCE_SIGNING_SECRET")),
	)
	if err != nil {
		slog.Error("invalid local evidence signing configuration", "error", err)
		os.Exit(1)
	}
	if err := configureDevelopmentSeed(
		mux,
		developmentSeedConfig{
			Environment:          os.Getenv("AGENTPAY_ENV"),
			RepositoryMode:       os.Getenv("AGENTPAY_REPOSITORY_MODE"),
			HTTPAddress:          addr,
			ProfileName:          os.Getenv("AGENTPAY_LOCAL_SEED_PROFILE"),
			WebhookSigningSecret: os.Getenv("AGENTPAY_LOCAL_WEBHOOK_SIGNING_SECRET"),
		},
		developmentSeedRepositories{
			Catalog:                catalogRepository,
			PurchaseIntents:        intentRepository,
			Approvals:              approvalRepository,
			Transactions:           transactionRepository,
			Evidence:               evidenceRepository,
			Disputes:               disputeRepository,
			PaymentDestinations:    paymentDestinationRepository,
			WebhookSubscriptions:   webhookSubscriptionRepository,
			WebhookDeliveries:      webhookDeliveryRepository,
			WebhookSecrets:         webhookSecretStore,
			IntegrationCredentials: integrationCredentialRepository,
			AuditEvents:            auditEventRepository,
			Idempotency:            idempotencyStore,
		},
		evidenceSigner,
	); err != nil {
		slog.Error("invalid local seed configuration", "error", err)
		os.Exit(1)
	}
	healthController, err := newDependencyHealthController(
		dependencyHealthConfig{
			RepositoryMode:      os.Getenv("AGENTPAY_REPOSITORY_MODE"),
			PaymentReadinessURL: os.Getenv("AGENTPAY_PAYMENT_READINESS_URL"),
			SellerReadinessURL:  os.Getenv("AGENTPAY_SELLER_READINESS_URL"),
			Timeout:             defaultDependencyHealthTimeout,
		},
		dependencyHealthDependencies{
			Catalog:             catalogRepository,
			EvidenceSigner:      evidenceSigner,
			SellerSigner:        sellerSigner,
			SellerSigningSecret: []byte(os.Getenv("AGENTPAY_LOCAL_SELLER_SIGNING_SECRET")),
			WebSocketRepository: realtimeHub,
		},
	)
	if err != nil {
		slog.Error("invalid dependency health configuration", "error", err)
		os.Exit(1)
	}
	healthController.RegisterRoutes(mux)
	evidenceRecorder := evidence.NewRecorder(
		evidenceRepository,
		idGenerator,
		evidenceSigner,
		clock,
	)
	paidRouteService := payments.NewPaidRouteService(
		catalogRepository,
		intentRepository,
		approvalRepository,
		approvalTokenSigner,
		clock,
		os.Getenv("AGENTPAY_PUBLIC_BASE_URL"),
	)
	var paymentAdapter payments.Adapter = payments.NewX402Adapter()
	if os.Getenv("AGENTPAY_USE_MOCK_PAYMENT") == "true" {
		paymentAdapter = payments.NewMockAdapter()
	}
	executionService := proxy.NewExecutionService(
		transactionRepository,
		sellerSigner,
		sellerForwarder,
		evidenceRecorder,
		clock,
	)
	executionService.SetUsageRecorder(usageService)
	checkoutService := payments.NewCheckoutService(
		paidRouteService,
		paymentAdapter,
		transactionRepository,
		evidenceRecorder,
		executionService,
		clock,
	)
	payments.NewHTTPController(checkoutService).RegisterRoutes(mux)
	transactionService := transactions.NewService(
		transactionRepository,
		evidenceRepository,
		evidenceSigner,
		catalogRepository,
	)
	transactionController := transactions.NewHTTPController(transactionService)
	transactionController.RegisterRoutes(mux)
	analytics.NewHTTPController(
		analytics.NewDashboardService(transactionService, analytics.NewService()),
	).RegisterRoutes(mux)
	disputeService := disputes.NewService(
		disputeRepository,
		transactionRepository,
		catalogRepository,
		idGenerator,
		clock,
	)
	disputeController := disputes.NewHTTPController(
		disputeService,
		idempotencyStore,
	)
	disputeController.RegisterRoutes(mux)
	handler := api.Middleware(api.Config{
		AllowedOrigin: os.Getenv("AGENTPAY_WEB_ORIGIN"),
		Authenticator: api.NewStaticAuthenticator(
			os.Getenv("AGENTPAY_LOCAL_SELLER_TOKEN"),
			os.Getenv("AGENTPAY_LOCAL_AGENT_KEY"),
		),
		SellerRequestLimiter: quotaService,
		SellerAuthorizer:     catalogService,
	}, mux)

	slog.Info("starting AgentPay API", "address", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("AgentPay API stopped", "error", err)
		os.Exit(1)
	}
}
