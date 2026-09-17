package catalog_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

// TestSellerRoutesCreateAndUpdateCatalog verifies the API-002 happy path.
func TestSellerRoutesCreateAndUpdateCatalog(t *testing.T) {
	t.Parallel()

	handler := newCatalogHandler(t)
	sellerResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers",
		"seller-create-1",
		`{"name":"Demo","slug":"demo-shop","upstreamBaseUrl":"https://seller.example"}`,
	)
	if sellerResponse.Code != http.StatusCreated {
		t.Fatalf("seller status = %d, body = %s", sellerResponse.Code, sellerResponse.Body.String())
	}

	var seller catalog.SellerResponse
	decodeCatalogResponse(t, sellerResponse, &seller)
	if seller.OwnerSubject != "" || seller.SigningSecretRef != "" {
		t.Fatal("seller response exposed private fields")
	}

	routeResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes",
		"route-create-1",
		`{"method":"POST","pathPattern":"/research","description":"Research","mimeType":"application/json","amount":"35000000","asset":"test-usdc","network":"test-network","payTo":"0x123","upstreamTimeoutSeconds":20}`,
	)
	if routeResponse.Code != http.StatusCreated {
		t.Fatalf("route status = %d, body = %s", routeResponse.Code, routeResponse.Body.String())
	}

	var route catalog.PaidRoute
	decodeCatalogResponse(t, routeResponse, &route)
	updateResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPatch,
		"/v1/sellers/"+seller.SellerID.String()+"/routes/"+route.RouteID.String(),
		"route-update-1",
		`{"amount":"36000000","expectedVersion":1}`,
	)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateResponse.Code, updateResponse.Body.String())
	}
}

// TestSellerCreationReplaysIdempotentResponse verifies stable mutation replay.
func TestSellerCreationReplaysIdempotentResponse(t *testing.T) {
	t.Parallel()

	handler := newCatalogHandler(t)
	requestBody := `{"name":"Demo","slug":"demo-shop","upstreamBaseUrl":"https://seller.example"}`
	first := performCatalogRequest(t, handler, http.MethodPost, "/v1/sellers", "seller-create-1", requestBody)
	second := performCatalogRequest(t, handler, http.MethodPost, "/v1/sellers", "seller-create-1", requestBody)

	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("statuses = (%d, %d)", first.Code, second.Code)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatal("idempotent replay returned a different response")
	}

	conflict := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers",
		"seller-create-1",
		`{"name":"Changed","slug":"changed-shop","upstreamBaseUrl":"https://seller.example"}`,
	)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d, want %d", conflict.Code, http.StatusConflict)
	}
}

// TestSellerCreationRejectsUnknownFields verifies strict control-plane JSON.
func TestSellerCreationRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	handler := newCatalogHandler(t)
	response := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers",
		"seller-create-1",
		`{"name":"Demo","slug":"demo-shop","upstreamBaseUrl":"https://seller.example","unknown":true}`,
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

// TestStorefrontDiscoveryReturnsManifestAndLLMSText verifies API-003 routes.
func TestStorefrontDiscoveryReturnsManifestAndLLMSText(t *testing.T) {
	t.Parallel()

	handler := newCatalogHandler(t)
	sellerResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers",
		"seller-create-1",
		`{"name":"Demo","slug":"demo-shop","upstreamBaseUrl":"https://seller.example"}`,
	)
	var seller catalog.SellerResponse
	decodeCatalogResponse(t, sellerResponse, &seller)
	performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes",
		"route-create-1",
		`{"method":"POST","pathPattern":"/research","description":"Research","mimeType":"application/json","amount":"35000000","asset":"test-usdc","network":"test-network","payTo":"0x123","upstreamTimeoutSeconds":20}`,
	)

	manifestRequest := httptest.NewRequest(http.MethodGet, "/store/demo-shop/manifest.json", nil)
	manifestResponse := httptest.NewRecorder()
	handler.ServeHTTP(manifestResponse, manifestRequest)
	if manifestResponse.Code != http.StatusOK {
		t.Fatalf("manifest status = %d, body = %s", manifestResponse.Code, manifestResponse.Body.String())
	}
	var manifest catalog.StorefrontManifest
	decodeCatalogResponse(t, manifestResponse, &manifest)
	if manifest.Seller.Slug != "demo-shop" || len(manifest.Routes) != 1 {
		t.Fatalf("manifest = %#v", manifest)
	}

	textRequest := httptest.NewRequest(http.MethodGet, "/store/demo-shop/llms.txt", nil)
	textResponse := httptest.NewRecorder()
	handler.ServeHTTP(textResponse, textRequest)
	if textResponse.Code != http.StatusOK {
		t.Fatalf("llms.txt status = %d", textResponse.Code)
	}
	if !strings.Contains(textResponse.Body.String(), "POST /research") {
		t.Fatalf("llms.txt = %q", textResponse.Body.String())
	}
}

// TestStorefrontDiscoveryReturnsNotFound verifies unknown storefront handling.
func TestStorefrontDiscoveryReturnsNotFound(t *testing.T) {
	t.Parallel()

	handler := newCatalogHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/store/missing/manifest.json", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

// newCatalogHandler creates the API-002 test server.
func newCatalogHandler(t *testing.T) http.Handler {
	t.Helper()

	repository := memory.NewCatalogRepository()
	idempotencyStore := memory.NewIdempotencyStore()
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	service := catalog.NewService(repository, domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("a", 256))), clock)
	controller := catalog.NewHTTPController(service, idempotencyStore)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	return api.Middleware(
		api.Config{Authenticator: api.NewStaticAuthenticator("seller-secret", "agent-secret")},
		mux,
	)
}

// performCatalogRequest sends an authenticated catalog mutation.
func performCatalogRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	idempotencyKey string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer seller-secret")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// decodeCatalogResponse decodes a successful JSON response.
func decodeCatalogResponse(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), destination); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
}
