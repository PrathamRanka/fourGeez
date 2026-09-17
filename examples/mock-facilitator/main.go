package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
)

type request struct {
	PaymentSignature string `json:"paymentSignature"`
}

type response struct {
	Valid             bool   `json:"valid"`
	Settled           bool   `json:"settled"`
	PaymentIdentifier string `json:"paymentIdentifier,omitempty"`
	Reason            string `json:"reason,omitempty"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /verify", handle(false))
	mux.HandleFunc("POST /settle", handle(true))

	slog.Info("starting local-only mock facilitator", "address", ":8091")
	if err := http.ListenAndServe(":8091", mux); err != nil {
		slog.Error("mock facilitator stopped", "error", err)
	}
}

func handle(settled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var input request
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(response{Reason: "invalid_json"})
			return
		}

		if input.PaymentSignature != "mock:approved" {
			w.WriteHeader(http.StatusPaymentRequired)
			_ = json.NewEncoder(w).Encode(response{Reason: "invalid_mock_signature"})
			return
		}

		digest := sha256.Sum256([]byte(input.PaymentSignature))
		_ = json.NewEncoder(w).Encode(response{
			Valid:             true,
			Settled:           settled,
			PaymentIdentifier: "mock_" + hex.EncodeToString(digest[:8]),
		})
	}
}
