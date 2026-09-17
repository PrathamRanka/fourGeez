package sandbox

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/proxy"
	agentpayverify "github.com/fourgeez/agentpay/verification/go"
)

// TestServiceIntegratesWithForwarderAndVerificationMiddleware verifies the live boundary.
func TestServiceIntegratesWithForwarderAndVerificationMiddleware(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	}
	secret := []byte("0123456789abcdef0123456789abcdef")
	verifier, err := agentpayverify.NewVerifier(agentpayverify.Config{
		Secret:      secret,
		ReplayStore: agentpayverify.NewMemoryReplayStore(),
		Clock:       clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	fulfillmentCalls := 0
	sellerHandler := verifier.Middleware(
		http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			fulfillmentCalls++
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"sandbox":"accepted"}`))
		}),
	)
	catalogReader := validCatalogReader()
	forwarder := proxy.NewForwarderWithClient(
		publicResolver{},
		handlerHTTPClient{handler: sellerHandler},
		1024,
	)
	service := NewService(
		catalogReader,
		testIDGenerator{},
		proxy.NewHMACSigner(proxy.NewLocalSecretProvider(secret), clock),
		forwarder,
		clock,
	)

	result, err := service.Validate(
		t.Context(),
		domain.ID(testSellerID),
		domain.ID(testRouteID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Fatalf("validation result = %#v", result)
	}
	if fulfillmentCalls != 1 {
		t.Fatalf("fulfillment calls = %d, want 1", fulfillmentCalls)
	}
}

type publicResolver struct{}

// LookupIP returns one public documentation address for SSRF validation.
func (publicResolver) LookupIP(
	_ context.Context,
	_ string,
	_ string,
) ([]net.IP, error) {
	return []net.IP{net.ParseIP("203.0.113.10")}, nil
}

type handlerHTTPClient struct {
	handler http.Handler
}

// Do dispatches an outbound request into the in-process seller application.
func (client handlerHTTPClient) Do(request *http.Request) (*http.Response, error) {
	responseRecorder := httptest.NewRecorder()
	client.handler.ServeHTTP(responseRecorder, request)
	result := responseRecorder.Result()
	responseBody, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}
	_ = result.Body.Close()
	result.Body = io.NopCloser(bytes.NewReader(responseBody))
	return result, nil
}
