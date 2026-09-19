package main

import (
	cryptorand "crypto/rand"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/fourgeez/agentpay/internal/analytics"
	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/identity"
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
	"github.com/fourgeez/agentpay/internal/sellerworkspace"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/storefront"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// main starts the local AgentPay HTTP API.
func main() {
	if err := validateRuntimeComposition(
		os.Getenv("AGENTPAY_ENV"),
		os.Getenv("AGENTPAY_REPOSITORY_MODE"),
	); err != nil {
		slog.Error("unsafe runtime composition", "error", err)
		os.Exit(1)
	}

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
	manualRefundRecordRepository := memory.NewManualRefundRecordRepository()
	browserPurchaseRepository := memory.NewBrowserPurchaseSessionRepository()
	integrationCredentialRepository := memory.NewIntegrationCredentialRepository()
	confirmationGrantRepository := memory.NewConfirmationGrantRepository()
	paymentDestinationRepository := memory.NewPaymentDestinationRepository()
	webhookSubscriptionRepository := memory.NewWebhookSubscriptionRepository()
	webhookDeliveryRepository := memory.NewWebhookDeliveryRepository()
	sellerPlanRepository := memory.NewSellerPlanRepository()
	providerEventRepository := memory.NewProviderEventRepository()
	usageMeterEventRepository := memory.NewUsageMeterEventRepository()
	quotaCounterRepository := memory.NewQuotaCounterRepository()
	sellerSessionRevocationRepository := memory.NewSellerSessionRevocationRepository()
	sellerWorkspaceRepository := memory.NewSellerWorkspaceRepository()
	storefrontPublicationRepository := memory.NewStorefrontPublicationRepository()
	auditEventRepository := memory.NewAuditEventRepository()
	webhookSecretStore := memory.NewWebhookSecretStore()
	idempotencyStore := memory.NewIdempotencyStore()
	idGenerator := domain.NewULIDGenerator(nil, nil)
	clock := domain.SystemClock{}
	sellerIdentityService, err := newSellerIdentityService(
		sellerIdentityConfig{
			Environment: os.Getenv("AGENTPAY_ENV"), LocalToken: os.Getenv("AGENTPAY_LOCAL_SELLER_TOKEN"),
			LocalSigningSecret: os.Getenv("AGENTPAY_LOCAL_IDENTITY_SIGNING_SECRET"),
			LocalSubject:       os.Getenv("AGENTPAY_LOCAL_SELLER_SUBJECT"), AWSRegion: os.Getenv("AWS_REGION"),
			UserPoolID: os.Getenv("AGENTPAY_SELLER_USER_POOL_ID"), ClientID: os.Getenv("AGENTPAY_SELLER_USER_POOL_CLIENT_ID"),
		},
		sellerSessionRevocationRepository,
		clock,
	)
	if err != nil {
		slog.Error("invalid seller identity configuration", "error", err)
		os.Exit(1)
	}
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
	sellerForwarder, err := newSellerForwarder(
		os.Getenv("AGENTPAY_ENV"),
		os.Getenv("AGENTPAY_SELLER_READINESS_URL"),
	)
	if err != nil {
		slog.Error("invalid local seller forwarding configuration", "error", err)
		os.Exit(1)
	}
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
	catalogController.RegisterControlRoutes(mux)
	identity.NewHTTPController(sellerIdentityService, catalogService).RegisterRoutes(mux)
	audit.NewHTTPController(
		audit.NewService(
			auditEventRepository,
			catalogService,
			idGenerator,
			clock,
		),
	).RegisterRoutes(mux)
	billingService := billing.NewService(
		configureOnboardingEntitlementRepository(
			os.Getenv("AGENTPAY_ENV"),
			sellerPlanRepository,
			catalogRepository,
			clock,
		),
		catalogService,
		clock,
	)
	billingService.SetAuditRecorder(auditAppender)
	quotaService := operations.NewService(
		quotaCounterRepository,
		billingService,
		clock,
	)
	catalogService.SetQuotaEnforcer(quotaService)
	billing.NewHTTPController(billingService).RegisterRoutes(mux)
	billing.NewStripeProviderEventHTTPController(
		billing.NewStripeProviderEventService(
			billing.StripeProviderEventConfig{
				ExpectedLivemode:   os.Getenv("AGENTPAY_STRIPE_LIVEMODE") == "true",
				ExpectedAccountID:  os.Getenv("AGENTPAY_STRIPE_ACCOUNT_ID"),
				ExpectedAPIVersion: os.Getenv("AGENTPAY_STRIPE_API_VERSION"),
			},
			billing.NewStripeWebhookVerifier(os.Getenv("AGENTPAY_STRIPE_WEBHOOK_SECRET"), 0),
			providerEventRepository,
			nil,
			billingService,
			clock,
		),
	).RegisterRoutes(mux)
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
		integrations.WithCredentialDigester(integrations.NewHMACCredentialDigester(
			integrations.StaticCredentialPepperProvider{Value: mustRandomSecret("credential pepper")},
		)),
		integrations.WithExchangeAuthorization(
			billingService,
			memory.NewProjectKeyExchangeRateLimiter(
				clock,
				integrations.DefaultProjectKeyExchangeAttempts,
				integrations.DefaultProjectKeyExchangeWindow,
			),
			quotaService,
		),
	)
	integrations.NewHTTPController(
		integrationService,
		idempotencyStore,
	).RegisterRoutes(mux)
	capabilityKeys, err := authorization.NewLocalES256KeyRing(clock)
	if err != nil {
		slog.Error("failed to initialize local capability signer", "error", err)
		os.Exit(1)
	}
	apiOrigin := os.Getenv("AGENTPAY_API_ORIGIN")
	if apiOrigin == "" {
		apiOrigin = "http://localhost:8080"
	}
	webOrigin := os.Getenv("AGENTPAY_WEB_ORIGIN")
	if webOrigin == "" {
		webOrigin = "http://localhost:3000"
	}
	accessTokenService := authorization.NewAccessTokenService(
		authorization.AccessTokenConfig{
			Issuer: apiOrigin, Audience: authorization.MCPAudience,
			Lifetime: authorization.MinimumAccessTokenLifetime,
		},
		integrationService,
		integrationCredentialRepository,
		billingService,
		capabilityKeys,
		capabilityKeys,
		idGenerator,
		clock,
	)
	authorization.NewHTTPController(accessTokenService).RegisterRoutes(mux)
	confirmationGrantService := authorization.NewConfirmationGrantService(
		confirmationGrantRepository,
		catalogService,
		integrationCredentialRepository,
		billingService,
		catalogRepository,
		idGenerator,
		integrations.NewSecureTokenGenerator(nil),
		integrations.NewHMACCredentialDigester(
			integrations.StaticCredentialPepperProvider{Value: mustRandomSecret("confirmation grant pepper")},
		),
		clock,
		auditAppender,
	)
	authorization.NewConfirmationGrantHTTPController(confirmationGrantService).RegisterRoutes(mux)
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
	workspaceService := sellerworkspace.NewService(sellerworkspace.Dependencies{
		Workspaces: sellerWorkspaceRepository, Sellers: catalogRepository, Products: catalogRepository,
		PaymentDestinations: sellerworkspace.NewPaymentDestinationRepositoryReader(paymentDestinationRepository),
		Credentials:         sellerworkspace.NewCredentialRepositoryReader(integrationCredentialRepository),
		Transactions:        sellerworkspace.NewTransactionRepositoryReader(transactionRepository), Evidence: evidenceRepository,
		WebhookSubscriptions: sellerworkspace.NewWebhookSubscriptionRepositoryReader(webhookSubscriptionRepository),
		WebhookDeliveries:    sellerworkspace.NewWebhookDeliveryRepositoryReader(webhookDeliveryRepository),
		Billing:              billingService, BillingPortal: sellerworkspace.UnavailableBillingPortal{},
		AccountVerification: sellerworkspace.StaticAccountVerification(os.Getenv("AGENTPAY_ENV") == "local"), Clock: clock,
	})
	integrationService.SetCredentialIssuanceAuthorizer(workspaceService)
	storefrontService := storefront.NewService(storefront.Dependencies{
		Catalog: catalogRepository, Destinations: paymentDestinationRepository,
		Entitlements: billingService, PublicationReadiness: workspaceService,
		Publications: storefrontPublicationRepository, Signer: capabilityKeys,
		Clock: clock, CanonicalOrigin: webOrigin, APIOrigin: apiOrigin, AuditRecorder: auditAppender,
	})
	catalogService.SetPublicationAuthorizer(storefrontService)
	sandboxService.SetEndpointVerificationRecorder(storefrontService)
	storefront.NewHTTPController(storefrontService).RegisterRoutes(mux)
	browserPurchaseService := browserpurchase.NewService(browserpurchase.Dependencies{
		Products:      browserPurchaseProductResolver{products: storefrontService},
		Repository:    browserPurchaseRepository,
		ProofVerifier: browserpurchase.NewEVMPersonalSignVerifier(),
		Clock:         clock,
	})
	browserPurchaseCookiePolicy := browserpurchase.CookiePolicy{
		AllowedOrigin: webOrigin,
		Secure:        os.Getenv("AGENTPAY_ENV") != "local",
	}
	browserpurchase.NewHTTPController(browserPurchaseService, browserPurchaseCookiePolicy).RegisterRoutes(mux)
	browserPurchaseAuthorizer := browserpurchase.NewRequestAuthorizer(browserPurchaseService, browserPurchaseCookiePolicy)
	sellerworkspace.NewHTTPController(
		workspaceService,
		sellerworkspace.NewContextPrincipalSource(
			catalogService,
			sellerworkspace.StaticAccountVerification(os.Getenv("AGENTPAY_ENV") == "local"),
		),
	).RegisterRoutes(mux)
	mcpController := mcpserver.NewHTTPController(
		accessTokenService,
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
			confirmationGrantService,
		),
		analyzer.NewService(),
	)
	mcpController.SetQuotaEnforcer(quotaService)
	mcpController.SetConnectorVerificationRecorder(workspaceService)
	mcpController.SetDiscoveryValidator(discovery.NewService())
	mcpController.RegisterRoutes(mux)
	intentService := intents.NewServiceWithCommerceAuthorizer(
		intentRepository,
		catalogRepository,
		storefrontService,
		idGenerator,
		clock,
	)
	intentController := intents.NewHTTPController(intentService, idempotencyStore)
	intentController.SetBrowserPurchaseAuthorizer(browserPurchaseAuthorizer)
	intentController.RegisterRoutes(mux)
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
			Catalog:                  catalogRepository,
			PurchaseIntents:          intentRepository,
			Approvals:                approvalRepository,
			Transactions:             transactionRepository,
			Evidence:                 evidenceRepository,
			Disputes:                 disputeRepository,
			ManualRefundRecords:      manualRefundRecordRepository,
			BrowserPurchaseSessions:  browserPurchaseRepository,
			PaymentDestinations:      paymentDestinationRepository,
			WebhookSubscriptions:     webhookSubscriptionRepository,
			WebhookDeliveries:        webhookDeliveryRepository,
			WebhookSecrets:           webhookSecretStore,
			IntegrationCredentials:   integrationCredentialRepository,
			ConfirmationGrants:       confirmationGrantRepository,
			SellerEntitlements:       sellerPlanRepository,
			ProviderEvents:           providerEventRepository,
			AuditEvents:              auditEventRepository,
			Idempotency:              idempotencyStore,
			SellerSessionRevocations: sellerSessionRevocationRepository,
			SellerWorkspaces:         sellerWorkspaceRepository,
			StorefrontPublications:   storefrontPublicationRepository,
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
	paidRouteService := payments.NewAuthorizedPaidRouteService(
		catalogRepository,
		intentRepository,
		nil,
		nil,
		storefrontService,
		clock,
		os.Getenv("AGENTPAY_PUBLIC_BASE_URL"),
	)
	var paymentAdapter payments.Adapter = payments.NewX402Adapter()
	if os.Getenv("AGENTPAY_USE_MOCK_PAYMENT") == "true" {
		paymentAdapter = payments.NewMockAdapter()
	}
	executionCapabilitySigner, err := proxy.NewES256ExecutionCapabilitySigner(
		proxy.ExecutionCapabilityConfig{Issuer: apiOrigin, Lifetime: 45 * time.Second},
		capabilityKeys,
		clock,
		cryptorand.Reader,
	)
	if err != nil {
		slog.Error("invalid execution capability configuration", "error", err)
		os.Exit(1)
	}
	executionService := proxy.NewExecutionService(
		transactionRepository,
		executionCapabilitySigner,
		sellerForwarder,
		evidenceRecorder,
		clock,
	)
	executionService.SetUsageRecorder(usageService)
	checkoutService := payments.NewCheckoutServiceWithCommerceAuthorizer(
		paidRouteService,
		paymentAdapter,
		transactionRepository,
		evidenceRecorder,
		executionService,
		clock,
		payments.NewAuthoritativeCommerceAuthorizer(sellerPlanRepository, clock),
	)
	checkoutService.SetBrowserPurchaseLifecycle(browserPurchaseService)
	checkoutController := payments.NewHTTPController(checkoutService)
	checkoutController.SetBrowserPurchaseAuthorizer(browserPurchaseAuthorizer)
	checkoutController.RegisterRoutes(mux)
	transactionService := transactions.NewService(
		transactionRepository,
		evidenceRepository,
		evidenceSigner,
		catalogRepository,
	)
	transactionController := transactions.NewHTTPController(transactionService)
	transactionController.SetBrowserPurchaseAuthorizer(browserPurchaseAuthorizer)
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
	disputeController.SetBrowserPurchaseAuthorizer(browserPurchaseAuthorizer)
	disputeController.RegisterRoutes(mux)
	manualRemediationService := disputes.NewManualRemediationService(
		disputeRepository,
		transactionRepository,
		manualRefundRecordRepository,
		clock,
	)
	manualRemediationService.SetAuditRecorder(auditAppender)
	disputes.NewManualRemediationHTTPController(
		manualRemediationService,
		idempotencyStore,
	).RegisterRoutes(mux)
	handler := api.Middleware(api.Config{
		AllowedOrigin: os.Getenv("AGENTPAY_WEB_ORIGIN"),
		Authenticator: identity.NewHTTPAuthenticator(
			sellerIdentityService,
			api.NewStaticAuthenticator("", os.Getenv("AGENTPAY_LOCAL_AGENT_KEY")),
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

func mustRandomSecret(name string) []byte {
	secret := make([]byte, 32)
	if _, err := cryptorand.Read(secret); err != nil {
		slog.Error("failed to initialize local secret", "name", name, "error", err)
		os.Exit(1)
	}
	return secret
}
