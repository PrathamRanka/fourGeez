package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	addr := os.Getenv("AGENTPAY_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	slog.Info("starting AgentPay API", "address", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("AgentPay API stopped", "error", err)
		os.Exit(1)
	}
}
