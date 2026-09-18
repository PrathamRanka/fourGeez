package browserpurchase

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/api"
)

func TestHTTPControllerCreatesSecureHostCookies(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	controller := NewHTTPController(fixture.service, CookiePolicy{AllowedOrigin: "https://app.agentpay.test", Secure: true})
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, "/v1/storefronts/demo-store/products/research-report/purchase-sessions", strings.NewReader(`{"requestBodyHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","maximumAmount":"150"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "checkout-0001")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookies = %#v", cookies)
	}
	assertCookie(t, cookies[0], ProductionPurchaseCookieName, true, true)
	assertCookie(t, cookies[1], ProductionCSRFCookieName, false, true)
	if strings.Contains(response.Body.String(), fixture.browserGrant) || strings.Contains(response.Body.String(), fixture.csrfToken) {
		t.Fatal("response body exposed browser credentials")
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestHTTPControllerUsesUnprefixedCookiesForLocalHTTP(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	controller := NewHTTPController(fixture.service, CookiePolicy{AllowedOrigin: "http://localhost:3000", Secure: false})
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, "/v1/storefronts/demo-store/products/research-report/purchase-sessions", strings.NewReader(`{"requestBodyHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","maximumAmount":"150"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "checkout-0001")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	cookies := response.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookies = %#v", cookies)
	}
	assertCookie(t, cookies[0], LocalPurchaseCookieName, true, false)
	assertCookie(t, cookies[1], LocalCSRFCookieName, false, false)
}

func TestHTTPControllerRejectsUnknownJSONAndMissingIdempotencyKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		key  string
	}{
		{name: "unknown field", body: `{"requestBodyHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","maximumAmount":"150","secret":"no"}`, key: "checkout-0001"},
		{name: "missing key", body: `{"requestBodyHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","maximumAmount":"150"}`},
		{name: "oversized body", body: `{"requestBodyHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","maximumAmount":"150","padding":"` + strings.Repeat("x", int(api.MaximumJSONBodyBytes)) + `"}`, key: "checkout-0001"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newServiceFixture()
			controller := NewHTTPController(fixture.service, CookiePolicy{AllowedOrigin: "http://localhost:3000"})
			mux := http.NewServeMux()
			controller.RegisterRoutes(mux)
			request := httptest.NewRequest(http.MethodPost, "/v1/storefronts/demo-store/products/research-report/purchase-sessions", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", test.key)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRequestAuthorizerRequiresExactOriginCSRFAndFetchSite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		origin    string
		header    string
		cookie    string
		fetchSite string
		wantErr   error
	}{
		{name: "valid", origin: "https://app.agentpay.test", header: "csrf-token-value-with-at-least-32-bytes", cookie: "csrf-token-value-with-at-least-32-bytes", fetchSite: "same-origin"},
		{name: "valid same site", origin: "https://app.agentpay.test", header: "csrf-token-value-with-at-least-32-bytes", cookie: "csrf-token-value-with-at-least-32-bytes", fetchSite: "same-site"},
		{name: "wrong origin", origin: "https://evil.test", header: "csrf-token-value-with-at-least-32-bytes", cookie: "csrf-token-value-with-at-least-32-bytes", fetchSite: "same-origin", wantErr: ErrOriginDenied},
		{name: "missing header", origin: "https://app.agentpay.test", cookie: "csrf-token-value-with-at-least-32-bytes", fetchSite: "same-origin", wantErr: ErrCSRFInvalid},
		{name: "cookie mismatch", origin: "https://app.agentpay.test", header: "csrf-token-value-with-at-least-32-bytes", cookie: "different-csrf-token-with-at-least-32-bytes", fetchSite: "same-origin", wantErr: ErrCSRFInvalid},
		{name: "cross site", origin: "https://app.agentpay.test", header: "csrf-token-value-with-at-least-32-bytes", cookie: "csrf-token-value-with-at-least-32-bytes", fetchSite: "cross-site", wantErr: ErrOriginDenied},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newServiceFixture()
			_, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"})
			if err != nil {
				t.Fatal(err)
			}
			authorizer := NewRequestAuthorizer(fixture.service, CookiePolicy{AllowedOrigin: "https://app.agentpay.test", Secure: true})
			request := httptest.NewRequest(http.MethodPost, "/v1/intents", nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set(CSRFHeaderName, test.header)
			request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			request.AddCookie(&http.Cookie{Name: ProductionPurchaseCookieName, Value: fixture.browserGrant})
			request.AddCookie(&http.Cookie{Name: ProductionCSRFCookieName, Value: test.cookie})
			_, err = authorizer.Authorize(request, AuthorizationRequirement{Authority: AuthorityCommerce, Mutation: true})
			if !errorsIs(err, test.wantErr) {
				t.Fatalf("Authorize() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestRequireAgentSellerOrBrowserAcceptsBrowserReadGrant(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	if _, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"}); err != nil {
		t.Fatal(err)
	}
	authorizer := NewRequestAuthorizer(fixture.service, CookiePolicy{AllowedOrigin: "http://localhost:3000", Secure: false})
	handler := RequireAgentSellerOrBrowser(
		authorizer,
		func(*http.Request) (AuthorizationRequirement, error) {
			return AuthorizationRequirement{Authority: AuthorityRead}, nil
		},
		http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			principal, ok := api.PrincipalFromContext(request.Context())
			if !ok || principal.Kind != api.PrincipalBrowser {
				t.Fatalf("principal = %#v, %v", principal, ok)
			}
			response.WriteHeader(http.StatusNoContent)
		}),
	)
	request := httptest.NewRequest(http.MethodGet, "/v1/transactions/txn_01K5D09YJ0C0M7RJM4FWQ0K9HA", nil)
	request.AddCookie(&http.Cookie{Name: LocalPurchaseCookieName, Value: fixture.browserGrant})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHTTPControllerRecoveryEndpointsRotateCookies(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	created, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.Complete(t.Context(), created.Session.PurchaseSessionID, "eip155:84532", "0x1111111111111111111111111111111111111111", "txn_01K5D09YJ0C0M7RJM4FWQ0K9HA", fixture.now); err != nil {
		t.Fatal(err)
	}
	controller := NewHTTPController(fixture.service, CookiePolicy{AllowedOrigin: "http://localhost:3000", Secure: false})
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	challengeRequest := httptest.NewRequest(http.MethodPost, "/v1/browser-purchases/"+created.Session.PurchaseSessionID.String()+"/recovery-challenges", strings.NewReader(`{"network":"eip155:84532","address":"0x1111111111111111111111111111111111111111"}`))
	challengeRequest.Header.Set("Content-Type", "application/json")
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusCreated {
		t.Fatalf("challenge status = %d, body = %s", challengeResponse.Code, challengeResponse.Body.String())
	}
	challengeID := "bpr_01K5D09YJ0C0M7RJM4FWQ0K9HB"
	fixture.proofVerifier.expectedMessage = recoveryMessage(created.Session.PurchaseSessionID, RecoveryChallengeID(challengeID), "eip155:84532", "0x1111111111111111111111111111111111111111", "recovery-nonce-value-with-at-least-32-bytes", fixture.now.Add(RecoveryChallengeLifetime))
	fixture.tokenGenerator.values = append(fixture.tokenGenerator.values, fixture.recoveredGrant, fixture.recoveredCSRF)
	recoverRequest := httptest.NewRequest(http.MethodPost, "/v1/browser-purchases/"+created.Session.PurchaseSessionID.String()+"/recover", strings.NewReader(`{"challengeId":"`+challengeID+`","network":"eip155:84532","address":"0x1111111111111111111111111111111111111111","proof":"0xsigned-proof"}`))
	recoverRequest.Header.Set("Content-Type", "application/json")
	recoverResponse := httptest.NewRecorder()
	mux.ServeHTTP(recoverResponse, recoverRequest)
	if recoverResponse.Code != http.StatusNoContent {
		t.Fatalf("recover status = %d, body = %s", recoverResponse.Code, recoverResponse.Body.String())
	}
	if len(recoverResponse.Result().Cookies()) != 2 {
		t.Fatalf("recovery cookies = %#v", recoverResponse.Result().Cookies())
	}
}

func assertCookie(t *testing.T, cookie *http.Cookie, name string, httpOnly, secure bool) {
	t.Helper()
	if cookie.Name != name || cookie.Path != "/" || cookie.Domain != "" || cookie.HttpOnly != httpOnly || cookie.Secure != secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie = %#v", cookie)
	}
}

func errorsIs(actual, expected error) bool {
	if expected == nil {
		return actual == nil
	}
	return errors.Is(actual, expected)
}
