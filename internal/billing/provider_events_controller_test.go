package billing

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestStripeProviderEventControllerPersistsBeforeAcknowledging(t *testing.T) {
	t.Parallel()
	repository := newProviderEventRepository()
	service := NewStripeProviderEventService(
		StripeProviderEventConfig{ExpectedAccountID: "acct_123", ExpectedAPIVersion: "2026-08-27.basil"},
		&stubStripeEventVerifier{event: VerifiedStripeEvent{EventID: "evt_123", EventType: "invoice.paid", AccountID: "acct_123", APIVersion: "2026-08-27.basil", ProviderCreatedAt: entitlementTime(2026, time.September, 18, 9)}},
		repository, nil, nil, domain.FixedClock{Value: entitlementTime(2026, time.September, 18, 10).Time()},
	)
	mux := http.NewServeMux()
	NewStripeProviderEventHTTPController(service).RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, "/v1/billing/providers/stripe/webhooks", strings.NewReader(`{"id":"evt_123"}`))
	request.Header.Set("Stripe-Signature", "signature")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if _, err := repository.Get(t.Context(), "evt_123"); err != nil {
		t.Fatalf("event was not durable before response: %v", err)
	}
}

func TestStripeProviderEventControllerRejectsInvalidSignature(t *testing.T) {
	t.Parallel()
	service := NewStripeProviderEventService(
		StripeProviderEventConfig{}, &stubStripeEventVerifier{err: errors.New("invalid signature")},
		newProviderEventRepository(), nil, nil, domain.SystemClock{},
	)
	mux := http.NewServeMux()
	NewStripeProviderEventHTTPController(service).RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, "/v1/billing/providers/stripe/webhooks", strings.NewReader(`{}`))
	request.Header.Set("Stripe-Signature", "invalid")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
