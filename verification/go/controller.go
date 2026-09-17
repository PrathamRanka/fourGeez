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
			http.Error(response, "AgentPay verification failed", http.StatusUnauthorized)
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
			http.Error(response, "AgentPay transaction replay", http.StatusConflict)
			return
		}
		if err != nil {
			status := http.StatusUnauthorized
			if !errors.Is(err, ErrInvalidSignature) && !errors.Is(err, ErrStaleRequest) {
				status = http.StatusServiceUnavailable
			}
			http.Error(response, "AgentPay verification failed", status)
			return
		}
		next.ServeHTTP(response, request)
	})
}
