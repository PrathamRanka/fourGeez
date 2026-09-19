package main

import (
	"context"
	cryptorand "crypto/rand"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

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
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/sellerworkspace"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/storefront"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// main starts AgentPay through Lambda or the local HTTP transport.
func main() {
	config, err := loadRuntimeConfig()
	if err != nil {
		slog.Error("invalid runtime configuration", "error", err)
		os.Exit(1)
	}
	if err := validateRuntimeComposition(
		config.Environment,
		config.RepositoryMode,
	); err != nil {
		slog.Error("unsafe runtime composition", "error", err)
		os.Exit(1)
	}
	idGenerator := domain.NewULIDGenerator(nil, nil)
	clock := domain.SystemClock{}
	runtime, err := newRuntimeDependencies(context.Background(), config, clock)
	if err != nil {
		slog.Error("failed to initialize runtime dependencies", "error", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	catalogRepository := runtime.catalog
	intentRepository := runtime.intents
	transactionRepository := runtime.transactions
	evidenceRepository := runtime.evidence
	disputeRepository := runtime.disputes
	manualRefundRecordRepository := runtime.manualRefunds
	browserPurchaseRepository := runtime.browserPurchases
	integrationCredentialRepository := runtime.integrationCredentials
	confirmationGrantRepository := runtime.confirmationGrants
	paymentDestinationRepository := runtime.paymentDestinations
	webhookSubscriptionRepository := runtime.webhookSubscriptions
	webhookDeliveryRepository := runtime.webhookDeliveries
	sellerPlanRepository := runtime.sellerEntitlements
	providerEventRepository := runtime.providerEvents
	usageMeterEventRepository := runtime.usageMeterEvents
	quotaCounterRepository := runtime.quotaCounters
	sellerSessionRevocationRepository := runtime.sellerSessionRevocations
	sellerWorkspaceRepository := runtime.sellerWorkspaces
	storefrontPublicationRepository := runtime.storefrontPublications
	auditEventRepository := runtime.auditEvents
	webhookSecretStore := runtime.webhookSecrets
	idempotencyStore := runtime.idempotency
	sellerIdentityService, err := newSellerIdentityService(
		sellerIdentityConfig{
			Environment: config.Environment, LocalToken: os.Getenv("AGENTPAY_LOCAL_SELLER_TOKEN"),
			LocalSigningSecret: os.Getenv("AGENTPAY_LOCAL_IDENTITY_SIGNING_SECRET"),
			LocalSubject:       os.Getenv("AGENTPAY_LOCAL_SELLER_SUBJECT"), AWSRegion: config.AWSRegion,
			UserPoolID: config.SellerUserPoolID, ClientID: config.SellerUserPoolClientID,
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
	sellerSigner := runtime.sellerSigner
	sellerForwarder := runtime.sellerForwarder
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
			config.Environment,
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
	if runtime.stripeProviderEventEnabled {
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
	}
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
		integrations.WithCredentialDigester(integrations.NewHMACCredentialDigester(runtime.credentialPepper)),
		integrations.WithExchangeAuthorization(
			billingService,
			runtime.projectKeyExchangeLimiter,
			quotaService,
		),
		integrations.WithRotationReplayProtector(runtime.rotationReplayProtector),
	)
	integrations.NewHTTPController(
		integrationService,
		idempotencyStore,
	).RegisterRoutes(mux)
	capabilityKeys := runtime.capabilityKeys
	apiOrigin := config.APIOrigin
	webOrigin := config.WebOrigin
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
		integrations.NewHMACCredentialDigester(runtime.confirmationGrantPepper),
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
		AccountVerification: sellerworkspace.AuthenticatedAccountVerification{}, Clock: clock,
		AllowLocalDevelopmentService: config.Environment == "local",
	})
	integrationService.SetCredentialIssuanceAuthorizer(workspaceService)
	storefrontService := storefront.NewService(storefront.Dependencies{
		Catalog: catalogRepository, Directory: catalogRepository, Destinations: paymentDestinationRepository,
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
			sellerworkspace.AuthenticatedAccountVerification{},
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
	evidenceSigner := runtime.evidenceSigner
	if runtime.configureDevelopmentSeed != nil {
		if err := runtime.configureDevelopmentSeed(mux); err != nil {
			slog.Error("invalid local seed configuration", "error", err)
			os.Exit(1)
		}
	}
	runtime.healthController.RegisterRoutes(mux)
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
		config.PublicBaseURL,
	)
	paymentAdapter := runtime.paymentAdapter
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

	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
		lambda.Start(httpadapter.NewV2(handler).ProxyWithContext)
		return
	}
	slog.Info("starting AgentPay API", "address", config.HTTPAddress)
	if err := http.ListenAndServe(config.HTTPAddress, handler); err != nil {
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
