package authorization

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
)

func TestAccessTokenHTTPExchangeAndJWKS(t *testing.T) {
	t.Parallel()
	service := testAccessTokenService(t)
	mux := http.NewServeMux()
	NewHTTPController(service).RegisterRoutes(mux)
	handler := api.Middleware(api.Config{}, mux)

	request := httptest.NewRequest(http.MethodPost, "/v1/integration-access-tokens", strings.NewReader(`{"audience":"urn:agentpay:mcp","scopes":["read"]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(ProjectKeyHeader, "apc2.project.secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("exchange response = %d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	var issued AccessTokenResponse
	if err := json.Unmarshal(response.Body.Bytes(), &issued); err != nil || issued.AccessToken == "" {
		t.Fatalf("issued response = %#v, error = %v", issued, err)
	}

	jwksRequest := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	jwksResponse := httptest.NewRecorder()
	handler.ServeHTTP(jwksResponse, jwksRequest)
	if jwksResponse.Code != http.StatusOK || jwksResponse.Header().Get("Content-Type") != "application/jwk-set+json" {
		t.Fatalf("JWKS response = %d headers=%v body=%s", jwksResponse.Code, jwksResponse.Header(), jwksResponse.Body.String())
	}

	oauthRequest := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil)
	oauthResponse := httptest.NewRecorder()
	handler.ServeHTTP(oauthResponse, oauthRequest)
	if oauthResponse.Code != http.StatusNotFound {
		t.Fatalf("OAuth metadata status = %d, want 404", oauthResponse.Code)
	}
}

func TestAccessTokenHTTPUsesStableErrors(t *testing.T) {
	t.Parallel()
	service := testAccessTokenService(t)
	mux := http.NewServeMux()
	NewHTTPController(service).RegisterRoutes(mux)
	handler := api.Middleware(api.Config{}, mux)

	request := httptest.NewRequest(http.MethodPost, "/v1/integration-access-tokens", strings.NewReader(`{"audience":"urn:agentpay:mcp","scopes":["read"],"grant_type":"client_credentials"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(ProjectKeyHeader, "apc2.project.secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"bad_request"`) {
		t.Fatalf("unknown-field response = %d %s", response.Code, response.Body.String())
	}

	missing := httptest.NewRequest(http.MethodPost, "/v1/integration-access-tokens", strings.NewReader(`{"audience":"urn:agentpay:mcp","scopes":["read"]}`))
	missing.Header.Set("Content-Type", "application/json")
	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, missing)
	if missingResponse.Code != http.StatusUnauthorized || !strings.Contains(missingResponse.Body.String(), `"code":"invalid_credential"`) {
		t.Fatalf("missing-key response = %d %s", missingResponse.Code, missingResponse.Body.String())
	}
}

func TestConfirmationGrantHTTPReturnsOneTimeSecret(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	service := NewConfirmationGrantService(
		newConfirmationRepository(), &confirmationSellerAuthorizer{},
		&credentialReader{credential: testCredential(now)}, &entitlementReader{response: activeEntitlement(now.Add(time.Hour), 7)},
		&confirmationTargetReader{seller: catalog.Seller{SellerID: domain.ID(testSellerID), Version: 3}},
		&fixedIDGenerator{id: domain.ID("mcg_01K5D09YJ0C0M7RJM4FWQ0K9HA")}, &fixedTokenGenerator{token: strings.Repeat("s", 43)},
		integrations.NewHMACCredentialDigester(integrations.StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))}),
		&mutableClock{now: now}, audit.NoopRecorder{},
	)
	mux := http.NewServeMux()
	NewConfirmationGrantHTTPController(service).RegisterRoutes(mux)
	handler := api.Middleware(api.Config{Authenticator: api.NewStaticAuthenticator("seller-secret", "agent-secret")}, mux)
	request := httptest.NewRequest(http.MethodPost, "/v1/sellers/"+testSellerID+"/mcp-confirmation-grants", strings.NewReader(`{"credentialId":"`+testCredentialID+`","tool":"configure_storefront","targetType":"seller","targetId":"`+testSellerID+`","argumentsSha256":"`+strings.Repeat("a", 64)+`","expectedResourceVersion":3,"summary":"Update the reviewed seller storefront settings"}`))
	request.Header.Set("Authorization", "Bearer seller-secret")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"confirmationGrant":"mcg1.`) {
		t.Fatalf("confirmation response = %d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}

func testAccessTokenService(t *testing.T) *AccessTokenService {
	t.Helper()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	clock := &mutableClock{now: now}
	keys, err := NewLocalES256KeyRing(clock)
	if err != nil {
		t.Fatal(err)
	}
	credential := testCredential(now)
	return NewAccessTokenService(
		AccessTokenConfig{Issuer: "https://api.agentpay.test", Audience: MCPAudience, Lifetime: MinimumAccessTokenLifetime},
		&exchangeAuthorizer{authorization: integrations.ExchangeAuthorization{
			Principal:        integrations.Principal{SellerID: credential.SellerID(), CredentialID: credential.CredentialID(), Scopes: []integrations.Scope{integrations.ScopeRead}},
			EntitlementEpoch: 7,
		}},
		&credentialReader{credential: credential},
		&entitlementReader{response: activeEntitlement(now.Add(time.Hour), 7)},
		keys,
		keys,
		&fixedIDGenerator{id: domain.ID(testCapabilityID)},
		clock,
	)
}
