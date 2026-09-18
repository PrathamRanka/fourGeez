package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fourgeez/agentpay/internal/domain"
)

// TestMiddlewareMapsSellerQuotaFailures verifies stable permission responses.
func TestMiddlewareMapsSellerQuotaFailures(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "permission", err: domain.ErrPermissionDenied, wantStatus: http.StatusForbidden, wantCode: ErrorCodePermissionDenied},
		{name: "rate", err: domain.ErrRateLimitExceeded, wantStatus: http.StatusTooManyRequests, wantCode: ErrorCodeRateLimited},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			limiter := &quotaLimiter{err: testCase.err}
			handler := Middleware(
				Config{Authenticator: quotaAuthenticator{}, SellerRequestLimiter: limiter},
				RequireSeller(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
					response.WriteHeader(http.StatusNoContent)
				})),
			)
			request := httptest.NewRequest(http.MethodGet, "/v1/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/plan", nil)
			request.Header.Set("Authorization", "Bearer valid")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			var body ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != testCase.wantStatus || body.Error.Code != testCase.wantCode || limiter.calls != 1 {
				t.Fatalf("status/body/calls = %d %#v %d", response.Code, body, limiter.calls)
			}
		})
	}
}

type quotaLimiter struct {
	calls int
	err   error
}

// ConsumeAPIRequest records one quota attempt.
func (limiter *quotaLimiter) ConsumeAPIRequest(context.Context, domain.ID) error {
	limiter.calls++
	return limiter.err
}

type quotaAuthenticator struct{}

// AuthenticateSeller accepts the test token.
func (quotaAuthenticator) AuthenticateSeller(_ context.Context, token string) (Principal, bool) {
	return Principal{Kind: PrincipalSeller, Subject: "seller"}, token == "valid"
}

// AuthenticateAgent rejects agent credentials.
func (quotaAuthenticator) AuthenticateAgent(context.Context, string) (Principal, bool) {
	return Principal{}, false
}
