package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// TestMiddlewareAddsRequestIDAndRecoversPanics verifies stable error responses.
func TestMiddlewareAddsRequestIDAndRecoversPanics(t *testing.T) {
	t.Parallel()

	handler := Middleware(Config{}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("sensitive panic")
	}))
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	requestID := response.Header().Get(RequestIDHeader)
	if requestID == "" {
		t.Fatal("response did not include a request ID")
	}
	var body ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if body.Error.Code != ErrorCodeInternal || body.Error.RequestID != requestID {
		t.Fatalf("error response = %#v", body)
	}
	if strings.Contains(response.Body.String(), "sensitive panic") {
		t.Fatal("panic details leaked to the client")
	}
}

// TestMiddlewareRestrictsCORS verifies only the configured origin is allowed.
func TestMiddlewareRestrictsCORS(t *testing.T) {
	t.Parallel()

	handler := Middleware(
		Config{AllowedOrigin: "https://app.example"},
		http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusNoContent)
		}),
	)

	allowedRequest := httptest.NewRequest(http.MethodOptions, "/v1/intents", nil)
	allowedRequest.Header.Set("Origin", "https://app.example")
	allowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowedResponse, allowedRequest)
	if allowedResponse.Code != http.StatusNoContent {
		t.Fatalf("allowed preflight status = %d", allowedResponse.Code)
	}
	if allowedResponse.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatal("configured origin was not allowed")
	}
	if !strings.Contains(allowedResponse.Header().Get("Access-Control-Allow-Methods"), http.MethodDelete) {
		t.Fatal("seller session revocation method was not allowed")
	}

	rejectedRequest := httptest.NewRequest(http.MethodOptions, "/v1/intents", nil)
	rejectedRequest.Header.Set("Origin", "https://evil.example")
	rejectedResponse := httptest.NewRecorder()
	handler.ServeHTTP(rejectedResponse, rejectedRequest)
	if rejectedResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unconfigured origin was allowed")
	}
}

// TestAuthenticationMiddleware verifies seller and agent credentials.
func TestAuthenticationMiddleware(t *testing.T) {
	t.Parallel()

	authenticator := NewStaticAuthenticator("seller-secret", "agent-secret")
	handler := Middleware(
		Config{Authenticator: authenticator},
		RequireAgent(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			principal, ok := PrincipalFromContext(request.Context())
			if !ok || principal.Kind != PrincipalAgent {
				t.Fatal("agent principal was not added to context")
			}
			response.WriteHeader(http.StatusNoContent)
		})),
	)

	unauthorizedRequest := httptest.NewRequest(http.MethodGet, "/v1/intents/int_123", nil)
	unauthorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResponse, unauthorizedRequest)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorizedResponse.Code, http.StatusUnauthorized)
	}

	authorizedRequest := httptest.NewRequest(http.MethodGet, "/v1/intents/int_123", nil)
	authorizedRequest.Header.Set(AgentKeyHeader, "agent-secret")
	authorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(authorizedResponse, authorizedRequest)
	if authorizedResponse.Code != http.StatusNoContent {
		t.Fatalf("authorized status = %d, want %d", authorizedResponse.Code, http.StatusNoContent)
	}
}

// TestMiddlewareLogsMetadataWithoutCredentials verifies sensitive headers are omitted.
func TestMiddlewareLogsMetadataWithoutCredentials(t *testing.T) {
	t.Parallel()

	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
	handler := Middleware(
		Config{Logger: logger},
		http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusNoContent)
		}),
	)
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Authorization", "Bearer seller-secret")
	request.Header.Set("Cookie", "session=secret")
	request.Header.Set("PAYMENT-SIGNATURE", "payment-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	logged := logOutput.String()
	for _, secret := range []string{"seller-secret", "session=secret", "payment-secret"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("log contains protected value %q", secret)
		}
	}
}

// TestDecodeJSONRejectsUnknownFieldsAndOversizedBodies verifies strict input handling.
func TestDecodeJSONRejectsUnknownFieldsAndOversizedBodies(t *testing.T) {
	t.Parallel()

	type requestBody struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"name":"seller","unexpected":true}`},
		{name: "oversized", body: `{"name":"` + strings.Repeat("a", int(MaximumJSONBodyBytes)) + `"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/sellers", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			var decoded requestBody
			if err := DecodeJSON(response, request, &decoded); err == nil {
				t.Fatal("DecodeJSON() accepted invalid input")
			}
		})
	}
}

// TestRequestIDPreservesTrustedIncomingValue verifies correlation propagation.
func TestRequestIDPreservesTrustedIncomingValue(t *testing.T) {
	t.Parallel()

	handler := Middleware(Config{}, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestID, ok := RequestIDFromContext(request.Context())
		if !ok {
			t.Fatal("request ID missing from context")
		}
		_ = WriteJSON(response, http.StatusOK, map[string]string{"requestId": requestID})
	}))
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set(RequestIDHeader, "request-123")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Header().Get(RequestIDHeader) != "request-123" {
		t.Fatal("incoming request ID was not preserved")
	}
}

// contextWithAuthenticator attaches an authenticator for direct middleware tests.
func contextWithAuthenticator(ctx context.Context, authenticator Authenticator) context.Context {
	return context.WithValue(ctx, authenticatorContextKey{}, authenticator)
}

func TestMiddlewareRejectsCrossSellerPathBeforeController(t *testing.T) {
	t.Parallel()

	sellerID, err := domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		authorize  error
		wantStatus int
		wantCode   string
	}{
		{name: "other tenant concealed", authorize: persistence.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: ErrorCodeNotFound},
		{name: "seller forbidden", authorize: domain.ErrPermissionDenied, wantStatus: http.StatusForbidden, wantCode: ErrorCodePermissionDenied},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			called := false
			handler := Middleware(Config{
				Authenticator:    NewStaticAuthenticator("seller-secret", "agent-secret"),
				SellerAuthorizer: fixedSellerAuthorizer{sellerID: sellerID, err: test.authorize},
			}, RequireSeller(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })))
			request := httptest.NewRequest(http.MethodGet, "/v1/sellers/"+sellerID.String()+"/routes", nil)
			request.Header.Set("Authorization", "Bearer seller-secret")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if called {
				t.Fatal("controller was called")
			}
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			var body ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != test.wantCode {
				t.Fatalf("code = %q, want %q", body.Error.Code, test.wantCode)
			}
		})
	}
}

type fixedSellerAuthorizer struct {
	sellerID domain.ID
	err      error
}

func (authorizer fixedSellerAuthorizer) AuthorizeSeller(_ context.Context, _ string, sellerID domain.ID) error {
	if sellerID != authorizer.sellerID {
		return errors.New("unexpected seller")
	}
	return authorizer.err
}
