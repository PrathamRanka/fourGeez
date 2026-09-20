package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

func TestRevokedProjectKeyImmediatelyBlocksIssuedUnexpiredMCPToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)
	clock := &revocationClock{now: now}
	credentialRepository := memory.NewIntegrationCredentialRepository()
	entitlements := revocationEntitlementResolver{accessEndsAt: now.Add(time.Hour), epoch: 7}
	integrationService := integrations.NewService(
		credentialRepository,
		revocationSellerAuthorizer{},
		revocationIDGenerator{id: domain.ID(testCredentialID)},
		revocationTokenGenerator{},
		clock,
		audit.NoopRecorder{},
		integrations.WithCredentialDigester(integrations.NewHMACCredentialDigester(
			integrations.StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))},
		)),
		integrations.WithExchangeAuthorization(entitlements, revocationExchangeLimiter{}, revocationQuota{}),
		integrations.WithCredentialIssuanceAuthorization(revocationCredentialIssuanceAuthorizer{}),
	)
	created, err := integrationService.Create(
		t.Context(),
		"owner-subject",
		domain.ID(testSellerID),
		integrations.CreateCredentialRequest{Label: "Revocation regression", Scopes: []integrations.Scope{integrations.ScopeRead}},
	)
	if err != nil {
		t.Fatal(err)
	}

	keys, err := authorization.NewLocalES256KeyRing(clock)
	if err != nil {
		t.Fatal(err)
	}
	accessTokenService := authorization.NewAccessTokenService(
		authorization.AccessTokenConfig{
			Issuer: "https://api.agentpay.test", Audience: authorization.MCPAudience,
			Lifetime: authorization.MinimumAccessTokenLifetime,
		},
		integrationService,
		credentialRepository,
		entitlements,
		keys,
		keys,
		revocationIDGenerator{id: domain.ID("cap_01K5D09YJ0C0M7RJM4FWQ0K9HZ")},
		clock,
	)
	issued, err := accessTokenService.Exchange(t.Context(), created.Token, authorization.AccessTokenRequest{
		Audience: authorization.MCPAudience,
		Scopes:   []integrations.Scope{integrations.ScopeRead},
	})
	if err != nil {
		t.Fatal(err)
	}

	operationQuota := &mcpQuotaEnforcer{}
	controller := NewHTTPController(accessTokenService, newTestResourceService(t), nil, nil)
	controller.SetQuotaEnforcer(operationQuota)
	if response := performMCPInitialize(t, controller, issued.AccessToken); response.Code != http.StatusOK {
		t.Fatalf("initial MCP status = %d, body = %s", response.Code, response.Body.String())
	}

	currentCredential, err := credentialRepository.GetByID(t.Context(), created.CredentialID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := integrationService.Revoke(
		t.Context(),
		"owner-subject",
		domain.ID(testSellerID),
		created.CredentialID,
		currentCredential.Version(),
	); err != nil {
		t.Fatal(err)
	}

	response := performMCPInitialize(t, controller, issued.AccessToken)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked MCP status = %d, body = %s", response.Code, response.Body.String())
	}
	var errorResponse api.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &errorResponse); err != nil {
		t.Fatal(err)
	}
	if errorResponse.Error.Code != api.ErrorCodeTokenRevoked {
		t.Fatalf("revoked MCP error code = %q", errorResponse.Error.Code)
	}
	if operationQuota.calls != 1 {
		t.Fatalf("MCP quota calls = %d, want only the pre-revocation request", operationQuota.calls)
	}
	if strings.Contains(response.Body.String(), issued.AccessToken) || strings.Contains(response.Body.String(), created.Token) {
		t.Fatal("revocation response exposed credential material")
	}
}

func performMCPInitialize(t *testing.T, handler http.Handler, accessToken string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"revocation-test","version":"1"}}}`))
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type revocationClock struct{ now time.Time }

func (clock *revocationClock) Now() time.Time { return clock.now }

type revocationIDGenerator struct{ id domain.ID }

func (generator revocationIDGenerator) New(domain.IDPrefix) (domain.ID, error) {
	return generator.id, nil
}

type revocationTokenGenerator struct{}

func (revocationTokenGenerator) NewToken() (string, error) { return strings.Repeat("s", 43), nil }

type revocationSellerAuthorizer struct{}

func (revocationSellerAuthorizer) AuthorizeSeller(context.Context, string, domain.ID) error {
	return nil
}

type revocationCredentialIssuanceAuthorizer struct{}

func (revocationCredentialIssuanceAuthorizer) AuthorizeCredentialIssuance(context.Context, string, domain.ID) error {
	return nil
}

type revocationEntitlementResolver struct {
	accessEndsAt time.Time
	epoch        uint64
}

func (resolver revocationEntitlementResolver) ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error) {
	return billing.SellerPlanResponse{Assignment: billing.SellerEntitlementView{
		SellerID: domain.ID(testSellerID), Status: billing.EntitlementStatusActive,
		AccessEndsAt: domain.NewTimestamp(resolver.accessEndsAt), EntitlementEpoch: resolver.epoch,
	}}, nil
}

type revocationExchangeLimiter struct{}

func (revocationExchangeLimiter) AllowProjectKeyExchange(context.Context, domain.ID) error {
	return nil
}

type revocationQuota struct{}

func (revocationQuota) ConsumeAPIRequest(context.Context, domain.ID) error { return nil }
