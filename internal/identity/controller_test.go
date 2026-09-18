package identity_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/identity"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

func TestCurrentSellerUsesAuthenticatedSubjectNotCallerInput(t *testing.T) {
	t.Parallel()

	sellerID, _ := domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	finder := &currentSellerFinder{response: catalog.SellerResponse{SellerID: sellerID, Name: "Owned store", Slug: "owned-store"}}
	handler := newIdentityHandler(t, finder)
	request := httptest.NewRequest(http.MethodGet, "/v1/me/seller?sellerId=sel_01K5D09YJ0C0M7RJM4FWQ0K9ZZ", nil)
	request.Header.Set("Authorization", "Bearer seller-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if finder.ownerSubject != "seller-subject" {
		t.Fatalf("owner subject = %q", finder.ownerSubject)
	}
	var body catalog.SellerResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SellerID != sellerID || body.OwnerSubject != "" {
		t.Fatalf("seller = %#v", body)
	}
}

func TestIdentityControllerReturnsStableAuthenticationAndLookupErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		token      string
		finderErr  error
		wantStatus int
		wantCode   string
	}{
		{name: "missing token", wantStatus: http.StatusUnauthorized, wantCode: api.ErrorCodeUnauthorized},
		{name: "seller absent", token: "seller-token", finderErr: persistence.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: api.ErrorCodeNotFound},
		{name: "lookup unavailable", token: "seller-token", finderErr: context.DeadlineExceeded, wantStatus: http.StatusServiceUnavailable, wantCode: api.ErrorCodeDependencyUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := newIdentityHandler(t, &currentSellerFinder{err: test.finderErr})
			request := httptest.NewRequest(http.MethodGet, "/v1/me/seller", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			var body api.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != test.wantCode {
				t.Fatalf("code = %q, want %q", body.Error.Code, test.wantCode)
			}
		})
	}
}

func TestRevokeSellerSessionInvalidatesSubsequentRequests(t *testing.T) {
	t.Parallel()

	handler := newIdentityHandler(t, &currentSellerFinder{})
	revokeRequest := httptest.NewRequest(http.MethodDelete, "/v1/me/session", nil)
	revokeRequest.Header.Set("Authorization", "Bearer seller-token")
	revokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, body = %s", revokeResponse.Code, revokeResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/me/seller", nil)
	request.Header.Set("Authorization", "Bearer seller-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status after revoke = %d, body = %s", response.Code, response.Body.String())
	}
}

func newIdentityHandler(t *testing.T, finder *currentSellerFinder) http.Handler {
	t.Helper()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	adapter := identity.NewLocalAdapter(domain.FixedClock{Value: now})
	if err := adapter.Register("seller-token", identity.Claims{
		Subject: "seller-subject", TokenID: "token-id", SessionID: "session-id", ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	revocations := memory.NewSellerSessionRevocationRepository()
	service := identity.NewService(adapter, revocations, domain.FixedClock{Value: now})
	mux := http.NewServeMux()
	identity.NewHTTPController(service, finder).RegisterRoutes(mux)
	return api.Middleware(api.Config{
		Authenticator: identity.NewHTTPAuthenticator(service, api.NewStaticAuthenticator("", "agent-token")),
	}, mux)
}

type currentSellerFinder struct {
	response     catalog.SellerResponse
	err          error
	ownerSubject string
}

func (finder *currentSellerFinder) GetCurrentSeller(_ context.Context, ownerSubject string) (catalog.SellerResponse, error) {
	finder.ownerSubject = ownerSubject
	return finder.response, finder.err
}
