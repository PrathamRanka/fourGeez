package agentpayverify

import (
	"bytes"
	"errors"
	"io"
	"net/http"
)

// Middleware verifies the raw request before invoking seller fulfillment.
func (verifier *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(
			response,
			request.Body,
			verifier.maximumBodyBytes,
		)
		body, err := io.ReadAll(request.Body)
		if err != nil {
			writeVerificationError(response, http.StatusUnauthorized)
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		err = verifier.Verify(
			request.Context(),
			request.Method,
			request.URL.Path,
			body,
			request.Header.Get(SignatureHeader),
			request.Header.Get(TimestampHeader),
			request.Header.Get(TransactionIDHeader),
		)
		if errors.Is(err, ErrReplay) {
			writeVerificationError(response, http.StatusConflict)
			return
		}
		if err != nil {
			status := http.StatusUnauthorized
			if !errors.Is(err, ErrInvalidSignature) && !errors.Is(err, ErrStaleRequest) {
				status = http.StatusServiceUnavailable
			}
			writeVerificationError(response, status)
			return
		}
		next.ServeHTTP(response, request)
	})
}

// writeVerificationError returns the same redacted JSON shape across adapters.
func writeVerificationError(response http.ResponseWriter, status int) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_, _ = response.Write([]byte(`{"error":"AgentPay verification failed"}`))
}
