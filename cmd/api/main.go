package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
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
	idempotencyStore := memory.NewIdempotencyStore()
	catalogService := catalog.NewService(
		catalogRepository,
		domain.NewULIDGenerator(nil, nil),
		domain.SystemClock{},
	)
	catalogController := catalog.NewHTTPController(catalogService, idempotencyStore)
	catalogController.RegisterRoutes(mux)
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
