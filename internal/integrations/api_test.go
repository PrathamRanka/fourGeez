package integrations_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

// TestCredentialHTTPLifecycle verifies create, list, and revoke operations.
func TestCredentialHTTPLifecycle(t *testing.T) {
	t.Parallel()

	service, handler := newIntegrationHandler(t)
	createResponse := performIntegrationRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+testIntegrationSellerID+"/integration-credentials",
		"credential-create-1",
		`{"label":"Claude Code","scopes":["read","configure"]}`,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}
	if createResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache control = %q", createResponse.Header().Get("Cache-Control"))
	}
	var created integrations.CredentialCreated
	decodeIntegrationResponse(t, createResponse, &created)
	if created.Token == "" {
		t.Fatal("create response omitted one-time token")
	}
	replayResponse := performIntegrationRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+testIntegrationSellerID+"/integration-credentials",
		"credential-create-1",
		`{"label":"Claude Code","scopes":["read","configure"]}`,
	)
	if replayResponse.Code != http.StatusConflict ||
		strings.Contains(replayResponse.Body.String(), created.Token) {
		t.Fatalf(
			"credential replay = status %d body %s",
			replayResponse.Code,
			replayResponse.Body.String(),
		)
	}

	listResponse := performIntegrationRequest(
		t,
		handler,
		http.MethodGet,
		"/v1/sellers/"+testIntegrationSellerID+"/integration-credentials",
		"",
		"",
	)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	if strings.Contains(listResponse.Body.String(), created.Token) ||
		strings.Contains(listResponse.Body.String(), "tokenHash") {
		t.Fatal("list response exposed credential material")
	}

	revokeResponse := performIntegrationRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+testIntegrationSellerID+
			"/integration-credentials/"+created.CredentialID.String()+"/revoke",
		"credential-revoke-1",
		`{"expectedVersion":1}`,
	)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, body = %s", revokeResponse.Code, revokeResponse.Body.String())
	}
	if _, err := service.Authenticate(
		t.Context(),
		created.Token,
		integrations.ScopeRead,
	); err == nil {
		t.Fatal("revoked HTTP credential still authenticated")
	}
}

// TestCredentialHTTPRejectsUnknownFields verifies strict control-plane JSON.
func TestCredentialHTTPRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, handler := newIntegrationHandler(t)
	response := performIntegrationRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+testIntegrationSellerID+"/integration-credentials",
		"credential-create-2",
		`{"label":"Codex","scopes":["read"],"admin":true}`,
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

// TestCredentialHTTPRejectsMalformedSellerID verifies path validation is a client error.
func TestCredentialHTTPRejectsMalformedSellerID(t *testing.T) {
	t.Parallel()

	_, handler := newIntegrationHandler(t)
	response := performIntegrationRequest(
		t,
		handler,
		http.MethodGet,
		"/v1/sellers/not-a-seller/integration-credentials",
		"",
		"",
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			response.Code,
			http.StatusBadRequest,
			response.Body.String(),
		)
	}
}

const testIntegrationSellerID = "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"

// newIntegrationHandler creates the AUT-002 HTTP test server.
func newIntegrationHandler(t *testing.T) (*integrations.Service, http.Handler) {
	t.Helper()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	service := integrations.NewService(
		memory.NewIntegrationCredentialRepository(),
		integrationSellerAuthorizer{},
		&integrationIDGenerator{},
		&integrationTokenGenerator{},
		clock,
		audit.NoopRecorder{},
	)
	controller := integrations.NewHTTPController(
		service,
		memory.NewIdempotencyStore(),
	)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	return service, api.Middleware(
		api.Config{
			Authenticator: api.NewStaticAuthenticator(
				"seller-secret",
				"agent-secret",
			),
		},
		mux,
	)
}

// performIntegrationRequest sends one seller-authenticated HTTP request.
func performIntegrationRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	idempotencyKey string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(
		method,
		path,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Authorization", "Bearer seller-secret")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// decodeIntegrationResponse decodes one successful JSON response.
func decodeIntegrationResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
	destination any,
) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), destination); err != nil {
		t.Fatal(err)
	}
}

type integrationSellerAuthorizer struct{}

// AuthorizeSeller accepts the local seller test principal.
func (integrationSellerAuthorizer) AuthorizeSeller(
	_ context.Context,
	ownerSubject string,
	sellerID domain.ID,
) error {
	if ownerSubject != "local-seller" || sellerID.String() != testIntegrationSellerID {
		return errors.New("seller not found")
	}
	return nil
}

type integrationIDGenerator struct{}

// New returns a stable credential identifier for API tests.
func (*integrationIDGenerator) New(
	_ domain.IDPrefix,
) (domain.ID, error) {
	return domain.ID("key_01K5D09YJ0C0M7RJM4FWQ0K9H8"), nil
}

type integrationTokenGenerator struct{}

// NewToken returns stable test-only credential secret material.
func (*integrationTokenGenerator) NewToken() (string, error) {
	return strings.Repeat("s", 43), nil
}
