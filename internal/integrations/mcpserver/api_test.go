package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestHTTPControllerServesAuthenticatedResources verifies real MCP transport behavior.
func TestHTTPControllerServesAuthenticatedResources(t *testing.T) {
	t.Parallel()

	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
	)
	server := httptest.NewServer(controller)
	t.Cleanup(server.Close)

	client := protocol.NewClient(
		&protocol.Implementation{Name: "agentpay-test", Version: "1.0.0"},
		nil,
	)
	transport := &protocol.StreamableClientTransport{
		Endpoint:             server.URL,
		HTTPClient:           authenticatedHTTPClient("valid-token"),
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(t.Context(), transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})

	listed, err := session.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Resources) != 5 {
		t.Fatalf("resource count = %d, want 5", len(listed.Resources))
	}
	read, err := session.ReadResource(
		t.Context(),
		&protocol.ReadResourceParams{URI: SellerResourceURI},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Contents) != 1 || read.Contents[0].URI != SellerResourceURI {
		t.Fatalf("seller resource = %#v", read.Contents)
	}
}

// TestHTTPControllerRequiresReadableCredential verifies authentication outcomes.
func TestHTTPControllerRequiresReadableCredential(t *testing.T) {
	t.Parallel()

	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
	)
	requestBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`

	testCases := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "invalid", token: "invalid-token", wantStatus: http.StatusUnauthorized},
		{name: "scope denied", token: "no-read-token", wantStatus: http.StatusForbidden},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(
				http.MethodPost,
				"/mcp",
				strings.NewReader(requestBody),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			if testCase.token != "" {
				request.Header.Set("Authorization", "Bearer "+testCase.token)
			}
			response := httptest.NewRecorder()
			controller.ServeHTTP(response, request)
			if response.Code != testCase.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					response.Code,
					testCase.wantStatus,
					response.Body.String(),
				)
			}
		})
	}
}

// TestHTTPControllerRedactsCredentialRepositoryFailures verifies safe errors.
func TestHTTPControllerRedactsCredentialRepositoryFailures(t *testing.T) {
	t.Parallel()

	repositoryError := errors.New("dynamodb endpoint and table details")
	controller := NewHTTPController(
		&testCredentialAuthenticator{err: repositoryError},
		newTestResourceService(t),
	)
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("{}"))
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()

	controller.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if strings.Contains(response.Body.String(), repositoryError.Error()) {
		t.Fatalf("response exposed repository failure: %s", response.Body.String())
	}
}

// TestHTTPControllerLimitsRequestBodies verifies the MCP body-size boundary.
func TestHTTPControllerLimitsRequestBodies(t *testing.T) {
	t.Parallel()

	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/mcp",
		strings.NewReader(strings.Repeat("x", int(api.MaximumJSONBodyBytes)+1)),
	)
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response := httptest.NewRecorder()

	controller.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

type testCredentialAuthenticator struct {
	err error
}

// Authenticate returns deterministic credential outcomes for transport tests.
func (authenticator *testCredentialAuthenticator) Authenticate(
	_ context.Context,
	rawToken string,
	_ integrations.Scope,
) (integrations.Principal, error) {
	if authenticator.err != nil {
		return integrations.Principal{}, authenticator.err
	}
	switch rawToken {
	case "valid-token":
		return integrations.Principal{
			SellerID:     domain.ID(testSellerID),
			CredentialID: domain.ID(testCredentialID),
			Scopes:       []integrations.Scope{integrations.ScopeRead},
		}, nil
	case "no-read-token":
		return integrations.Principal{}, integrations.ErrScopeDenied
	default:
		return integrations.Principal{}, integrations.ErrCredentialInvalid
	}
}

type bearerRoundTripper struct {
	token string
}

// RoundTrip adds the test credential without changing production clients.
func (transport bearerRoundTripper) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	requestClone := request.Clone(request.Context())
	requestClone.Header.Set("Authorization", "Bearer "+transport.token)
	return http.DefaultTransport.RoundTrip(requestClone)
}

// authenticatedHTTPClient creates a client that sends one bearer credential.
func authenticatedHTTPClient(token string) *http.Client {
	return &http.Client{Transport: bearerRoundTripper{token: token}}
}
