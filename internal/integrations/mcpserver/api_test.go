package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/integrations/analyzer"
	"github.com/fourgeez/agentpay/internal/integrations/discovery"
	"github.com/fourgeez/agentpay/internal/integrations/stacks"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestHTTPControllerServesAuthenticatedResources verifies real MCP transport behavior.
func TestHTTPControllerServesAuthenticatedResources(t *testing.T) {
	t.Parallel()

	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
		nil,
		nil,
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
	if len(listed.Resources) != 11 {
		t.Fatalf("resource count = %d, want 11", len(listed.Resources))
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

// TestHTTPControllerConsumesAuthenticatedMCPQuota verifies per-operation enforcement.
func TestHTTPControllerConsumesAuthenticatedMCPQuota(t *testing.T) {
	t.Parallel()

	quota := &mcpQuotaEnforcer{err: domain.ErrRateLimitExceeded}
	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
		nil,
		nil,
	)
	controller.SetQuotaEnforcer(quota)
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response := httptest.NewRecorder()

	controller.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests || quota.calls != 1 {
		t.Fatalf("status/calls = %d/%d", response.Code, quota.calls)
	}
	if !strings.Contains(response.Body.String(), `"code":"rate_limited"`) {
		t.Fatalf("quota error body = %s", response.Body.String())
	}
}

type mcpQuotaEnforcer struct {
	calls int
	err   error
}

// ConsumeAPIRequest permits seller API requests.
func (enforcer *mcpQuotaEnforcer) ConsumeAPIRequest(context.Context, domain.ID) error {
	return nil
}

// ConsumeMCPOperation records one authenticated MCP request.
func (enforcer *mcpQuotaEnforcer) ConsumeMCPOperation(context.Context, domain.ID) error {
	enforcer.calls++
	return enforcer.err
}

// ConsumeWebhookDelivery permits webhook delivery.
func (enforcer *mcpQuotaEnforcer) ConsumeWebhookDelivery(context.Context, domain.ID, string) error {
	return nil
}

// AllowPublishedRoute permits route publication.
func (enforcer *mcpQuotaEnforcer) AllowPublishedRoute(context.Context, domain.ID, uint64) error {
	return nil
}

// AllowWebhookSubscription permits webhook subscription creation.
func (enforcer *mcpQuotaEnforcer) AllowWebhookSubscription(context.Context, domain.ID, uint64) error {
	return nil
}

// TestHTTPControllerPublishesSetupPrompt verifies the coding-agent workflow.
func TestHTTPControllerPublishesSetupPrompt(t *testing.T) {
	t.Parallel()

	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
		nil,
		nil,
	)
	server := httptest.NewServer(controller)
	t.Cleanup(server.Close)

	client := protocol.NewClient(
		&protocol.Implementation{Name: "agentpay-prompt-test", Version: "1.0.0"},
		nil,
	)
	session, err := client.Connect(
		t.Context(),
		&protocol.StreamableClientTransport{
			Endpoint:             server.URL,
			HTTPClient:           authenticatedHTTPClient("valid-token"),
			DisableStandaloneSSE: true,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})

	prompts, err := session.ListPrompts(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts.Prompts) != 1 || prompts.Prompts[0].Name != SetupPromptName {
		t.Fatalf("prompts = %#v", prompts.Prompts)
	}
	result, err := session.GetPrompt(
		t.Context(),
		&protocol.GetPromptParams{
			Name: SetupPromptName,
			Arguments: map[string]string{
				"host":  "claude-code",
				"stack": "express",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("message count = %d, want 1", len(result.Messages))
	}
	content, ok := result.Messages[0].Content.(*protocol.TextContent)
	if !ok {
		t.Fatalf("prompt content = %T, want *mcp.TextContent", result.Messages[0].Content)
	}
	for _, fragment := range []string{
		"Express",
		"@agentpay/verify-node",
		"npm test",
		"canonical",
		"llms.txt",
		"cannot guarantee ranking",
		"explicit seller confirmation",
	} {
		if !strings.Contains(content.Text, fragment) {
			t.Fatalf("prompt omitted %q: %s", fragment, content.Text)
		}
	}
}

// TestHTTPControllerRequiresReadableCredential verifies authentication outcomes.
func TestHTTPControllerRequiresReadableCredential(t *testing.T) {
	t.Parallel()

	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
		nil,
		nil,
	)
	requestBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`

	testCases := []struct {
		name       string
		token      string
		wantStatus int
		wantCode   string
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized, wantCode: api.ErrorCodeInvalidCredential},
		{name: "invalid", token: "invalid-token", wantStatus: http.StatusUnauthorized, wantCode: api.ErrorCodeInvalidCredential},
		{name: "project key rejected", token: "apc2.key.secret", wantStatus: http.StatusUnauthorized, wantCode: api.ErrorCodeInvalidCredential},
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
			if !strings.Contains(response.Body.String(), `"code":"`+testCase.wantCode+`"`) {
				t.Fatalf("error body = %s, want code %q", response.Body.String(), testCase.wantCode)
			}
			if response.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing Bearer challenge")
			}
		})
	}
}

// TestHTTPControllerEnforcesPerOperationScopes verifies scoped resources and tools.
func TestHTTPControllerEnforcesPerOperationScopes(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
		NewMutationService(
			&testCatalogMutator{},
			memory.NewIdempotencyStore(),
			clock,
			&testSandboxValidator{valid: true},
			audit.NoopRecorder{},
			&testConfirmationConsumer{},
		),
		nil,
	)
	server := httptest.NewServer(controller)
	t.Cleanup(server.Close)
	client := protocol.NewClient(
		&protocol.Implementation{Name: "agentpay-scope-test", Version: "1.0.0"},
		nil,
	)
	session, err := client.Connect(
		t.Context(),
		&protocol.StreamableClientTransport{
			Endpoint:             server.URL,
			HTTPClient:           authenticatedHTTPClient("no-read-token"),
			DisableStandaloneSSE: true,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})

	if _, err := session.ReadResource(
		t.Context(),
		&protocol.ReadResourceParams{URI: SellerResourceURI},
	); err == nil {
		t.Fatal("configure-only credential read a seller resource")
	}
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 6 {
		t.Fatalf("tool count = %d, want 6", len(tools.Tools))
	}
	result, err := session.CallTool(
		t.Context(),
		&protocol.CallToolParams{
			Name: "configure_route",
			Arguments: map[string]any{
				"idempotencyKey":        "route-create-003",
				"confirmationGrant":     "mcg1.test.secret",
				"expectedSellerVersion": 1,
				"route": map[string]any{
					"displayName":             "Research Report",
					"productSlug":             "research-report",
					"method":                  "POST",
					"pathPattern":             "/research",
					"description":             "Research",
					"mimeType":                "application/json",
					"amount":                  "100",
					"asset":                   "USDC",
					"network":                 "eip155:84532",
					"payTo":                   "0x123",
					"approvalThresholdAmount": nil,
					"upstreamTimeoutSeconds":  20,
				},
			},
		},
	)
	if err != nil || result.IsError {
		message := ""
		if result != nil && len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*protocol.TextContent); ok {
				message = textContent.Text
			}
		}
		t.Fatalf("CallTool() error = %v, message = %s", err, message)
	}
	unknownFieldResult, err := session.CallTool(
		t.Context(),
		&protocol.CallToolParams{
			Name: "configure_route",
			Arguments: map[string]any{
				"idempotencyKey":        "route-create-004",
				"confirmationGrant":     "mcg1.test.secret",
				"expectedSellerVersion": 1,
				"route":                 map[string]any{},
				"admin":                 true,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !unknownFieldResult.IsError {
		t.Fatal("configure_route accepted an unknown argument")
	}
}

// TestHTTPControllerAnalyzesRepositoryWithoutPublishing verifies AUT-005 integration.
func TestHTTPControllerAnalyzesRepositoryWithoutPublishing(t *testing.T) {
	t.Parallel()

	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
		nil,
		analyzer.NewService(),
	)
	server := httptest.NewServer(controller)
	t.Cleanup(server.Close)
	client := protocol.NewClient(
		&protocol.Implementation{Name: "agentpay-analyzer-test", Version: "1.0.0"},
		nil,
	)
	session, err := client.Connect(
		t.Context(),
		&protocol.StreamableClientTransport{
			Endpoint:             server.URL,
			HTTPClient:           authenticatedHTTPClient("validate-token"),
			DisableStandaloneSSE: true,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})

	result, err := session.CallTool(
		t.Context(),
		&protocol.CallToolParams{
			Name: "analyze_repository",
			Arguments: map[string]any{
				"manifest": map[string]any{
					"schemaVersion": "agentpay.repository.v1",
					"serviceName":   "Research API",
					"framework":     "go",
					"openapiPath":   "openapi.yaml",
				},
				"openapi": "openapi: 3.1.0\npaths:\n  /research:\n    post:\n      summary: Research\n",
			},
		},
	)
	if err != nil || result.IsError {
		t.Fatalf("CallTool() = (%#v, %v)", result, err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured result = %#v", result.StructuredContent)
	}
	proposals, ok := structured["proposals"].([]any)
	if !ok || len(proposals) != 1 {
		t.Fatalf("proposals = %#v", structured["proposals"])
	}
}

// TestHTTPControllerDetectsMaintainedStacks verifies the coding agent can
// select a setup prompt from bounded repository evidence rather than guessing.
func TestHTTPControllerDetectsMaintainedStacks(t *testing.T) {
	t.Parallel()

	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
		nil,
		analyzer.NewService(),
	)
	server := httptest.NewServer(controller)
	t.Cleanup(server.Close)
	client := protocol.NewClient(
		&protocol.Implementation{Name: "agentpay-stack-test", Version: "1.0.0"},
		nil,
	)
	session, err := client.Connect(
		t.Context(),
		&protocol.StreamableClientTransport{
			Endpoint:             server.URL,
			HTTPClient:           authenticatedHTTPClient("validate-token"),
			DisableStandaloneSSE: true,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(
		t.Context(),
		&protocol.CallToolParams{
			Name: "detect_repository_stacks",
			Arguments: map[string]any{
				"files": map[string]any{
					"package.json": `{"dependencies":{"next":"16.0.0","express":"5.0.0"}}`,
				},
			},
		},
	)
	if err != nil || result.IsError {
		t.Fatalf("CallTool() = (%#v, %v)", result, err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok || structured["schemaVersion"] != stacks.DetectionSchemaVersion {
		t.Fatalf("structured result = %#v", result.StructuredContent)
	}
	detections, ok := structured["detections"].([]any)
	if !ok || len(detections) != 2 {
		t.Fatalf("detections = %#v", structured["detections"])
	}
	first, ok := detections[0].(map[string]any)
	if !ok || first["stack"] != string(stacks.StackNextJS) {
		t.Fatalf("first detection = %#v", detections[0])
	}
}

// TestHTTPControllerRunsSandboxValidation verifies the MCP transport result.
func TestHTTPControllerRunsSandboxValidation(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	}
	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
		NewMutationService(
			&testCatalogMutator{},
			memory.NewIdempotencyStore(),
			clock,
			&testSandboxValidator{valid: true},
			audit.NoopRecorder{},
			&testConfirmationConsumer{},
		),
		nil,
	)
	server := httptest.NewServer(controller)
	t.Cleanup(server.Close)
	client := protocol.NewClient(
		&protocol.Implementation{Name: "agentpay-sandbox-test", Version: "1.0.0"},
		nil,
	)
	session, err := client.Connect(
		t.Context(),
		&protocol.StreamableClientTransport{
			Endpoint:             server.URL,
			HTTPClient:           authenticatedHTTPClient("validate-token"),
			DisableStandaloneSSE: true,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})

	result, err := session.CallTool(
		t.Context(),
		&protocol.CallToolParams{
			Name: "sandbox_validate_route",
			Arguments: map[string]any{
				"routeId": testRouteID,
			},
		},
	)
	if err != nil || result.IsError {
		t.Fatalf("CallTool() = (%#v, %v)", result, err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok || structured["valid"] != true {
		t.Fatalf("structured result = %#v", result.StructuredContent)
	}
}

// TestHTTPControllerValidatesStorefrontArtifacts verifies SEO-003 MCP access.
func TestHTTPControllerValidatesStorefrontArtifacts(t *testing.T) {
	t.Parallel()

	controller := NewHTTPController(
		&testCredentialAuthenticator{},
		newTestResourceService(t),
		nil,
		nil,
	)
	controller.SetDiscoveryValidator(discovery.NewService())
	server := httptest.NewServer(controller)
	t.Cleanup(server.Close)
	client := protocol.NewClient(
		&protocol.Implementation{Name: "agentpay-discovery-test", Version: "1.0.0"},
		nil,
	)
	session, err := client.Connect(
		t.Context(),
		&protocol.StreamableClientTransport{
			Endpoint:             server.URL,
			HTTPClient:           authenticatedHTTPClient("validate-token"),
			DisableStandaloneSSE: true,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})

	result, err := session.CallTool(
		t.Context(),
		&protocol.CallToolParams{
			Name: "validate_storefront_artifacts",
			Arguments: map[string]any{
				"baseUrl": "https://seller.example",
				"pages": []any{map[string]any{
					"url":                      "https://seller.example/products/research",
					"routePath":                "/research",
					"title":                    "Research report API product",
					"description":              "Purchase a focused research report generated from the seller's published API route.",
					"canonicalUrl":             "https://seller.example/products/research",
					"robots":                   "index,follow",
					"headingOne":               "Research report API product",
					"visibleText":              strings.Repeat("Research report details and delivery terms. ", 4),
					"language":                 "en",
					"mainLandmarks":            1,
					"missingImageAltCount":     0,
					"unlabelledControls":       0,
					"structuredDataJson":       `{"@context":"https://schema.org","@type":"Product","name":"Research report API product","description":"Purchase a focused research report generated from the seller's published API route.","url":"https://seller.example/products/research"}`,
					"htmlBytes":                60000,
					"javaScriptBytes":          120000,
					"cssBytes":                 30000,
					"blockingScriptCount":      1,
					"largestContentfulPaintMs": 1800,
				}},
				"robotsTxt":    "User-agent: *\nAllow: /\nSitemap: https://seller.example/sitemap.xml\n",
				"sitemapXml":   `<urlset><url><loc>https://seller.example/products/research</loc></url></urlset>`,
				"llmsText":     "# Seller\n- [Research report API product](https://seller.example/products/research): Purchase a focused research report generated from the seller's published API route.\n",
				"manifestJson": `{"routes":[{"pathPattern":"/research","description":"Purchase a focused research report generated from the seller's published API route.","enabled":true}]}`,
			},
		},
	)
	if err != nil || result.IsError {
		t.Fatalf("CallTool() = (%#v, %v)", result, err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok || structured["valid"] != true {
		t.Fatalf("structured result = %#v", result.StructuredContent)
	}
}

// TestMutationToolResultRedactsUnexpectedErrors verifies safe MCP failures.
func TestMutationToolResultRedactsUnexpectedErrors(t *testing.T) {
	t.Parallel()

	repositoryError := errors.New("private dynamodb endpoint")
	_, _, err := mutationToolResult(MutationResult{}, repositoryError)
	if err == nil || strings.Contains(err.Error(), repositoryError.Error()) {
		t.Fatalf("mutationToolResult() error = %v", err)
	}
	for _, expected := range []error{authorization.ErrConfirmationDenied, authorization.ErrConfirmationReplayed} {
		_, _, err := mutationToolResult(MutationResult{}, expected)
		if !errors.Is(err, expected) {
			t.Fatalf("mutationToolResult() error = %v, want %v", err, expected)
		}
	}
}

// TestHTTPControllerRedactsCredentialRepositoryFailures verifies safe errors.
func TestHTTPControllerRedactsCredentialRepositoryFailures(t *testing.T) {
	t.Parallel()

	repositoryError := errors.New("dynamodb endpoint and table details")
	controller := NewHTTPController(
		&testCredentialAuthenticator{err: repositoryError},
		newTestResourceService(t),
		nil,
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("{}"))
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()

	controller.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
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
		nil,
		nil,
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
func (authenticator *testCredentialAuthenticator) AuthorizeAccessToken(
	_ context.Context,
	rawToken string,
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
		return integrations.Principal{
			SellerID:     domain.ID(testSellerID),
			CredentialID: domain.ID(testCredentialID),
			Scopes:       []integrations.Scope{integrations.ScopeConfigure},
		}, nil
	case "validate-token":
		return integrations.Principal{
			SellerID:     domain.ID(testSellerID),
			CredentialID: domain.ID(testCredentialID),
			Scopes:       []integrations.Scope{integrations.ScopeValidate},
		}, nil
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
