package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/integrations/mcpserver"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/payments"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/realtime"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// main starts the local AgentPay HTTP API.
func main() {
	addr := os.Getenv("AGENTPAY_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		_ = api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	catalogRepository := memory.NewCatalogRepository()
	intentRepository := memory.NewPurchaseIntentRepository()
	approvalRepository := memory.NewApprovalRepository()
	transactionRepository := memory.NewTransactionRepository()
	evidenceRepository := memory.NewEvidenceRepository()
	disputeRepository := memory.NewDisputeRepository()
	integrationCredentialRepository := memory.NewIntegrationCredentialRepository()
	idempotencyStore := memory.NewIdempotencyStore()
	idGenerator := domain.NewULIDGenerator(nil, nil)
	clock := domain.SystemClock{}
	catalogService := catalog.NewService(
		catalogRepository,
		idGenerator,
		clock,
	)
	catalogController := catalog.NewHTTPController(catalogService, idempotencyStore)
	catalogController.RegisterRoutes(mux)
	integrationService := integrations.NewService(
		integrationCredentialRepository,
		catalogService,
		idGenerator,
		integrations.NewSecureTokenGenerator(nil),
		clock,
	)
	integrations.NewHTTPController(
		integrationService,
		idempotencyStore,
	).RegisterRoutes(mux)
	mcpserver.NewHTTPController(
		integrationService,
		mcpserver.NewService(
			catalogRepository,
			transactionRepository,
		),
	).RegisterRoutes(mux)
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
	sellerSigner := proxy.NewHMACSigner(
		proxy.NewLocalSecretProvider(
			[]byte(os.Getenv("AGENTPAY_LOCAL_SELLER_SIGNING_SECRET")),
		),
		clock,
	)
	executionService := proxy.NewExecutionService(
		transactionRepository,
		sellerSigner,
		proxy.NewForwarder(nil),
		evidenceRecorder,
		clock,
	)
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
	}, mux)

	slog.Info("starting AgentPay API", "address", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("AgentPay API stopped", "error", err)
		os.Exit(1)
	}
}
