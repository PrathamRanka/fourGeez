package storefront_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fourgeez/agentpay/internal/api"
	. "github.com/fourgeez/agentpay/internal/storefront"
)

func TestHTTPControllerReturnsSignedActiveManifestAndProduct(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.verifyEndpoint(t)
	handler := storefrontHandler(fixture.service)

	manifestResponse := httptest.NewRecorder()
	handler.ServeHTTP(manifestResponse, httptest.NewRequest(http.MethodGet, "/store/"+fixture.seller.Slug+"/manifest.json", nil))
	if manifestResponse.Code != http.StatusOK {
		t.Fatalf("manifest status = %d, body = %s", manifestResponse.Code, manifestResponse.Body.String())
	}
	var manifest SignedStorefrontManifest
	if err := json.Unmarshal(manifestResponse.Body.Bytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Signature.Value == "" || len(manifest.Document.Products) != 1 {
		t.Fatalf("manifest = %#v", manifest)
	}
	if manifestResponse.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" {
		t.Fatalf("manifest cache control = %q", manifestResponse.Header().Get("Cache-Control"))
	}

	productResponse := httptest.NewRecorder()
	handler.ServeHTTP(productResponse, httptest.NewRequest(http.MethodGet, "/v1/storefronts/"+fixture.seller.Slug+"/products/"+fixture.route.ProductSlug, nil))
	if productResponse.Code != http.StatusOK {
		t.Fatalf("product status = %d, body = %s", productResponse.Code, productResponse.Body.String())
	}
	var product SignedPublicProductDocument
	if err := json.Unmarshal(productResponse.Body.Bytes(), &product); err != nil {
		t.Fatal(err)
	}
	if product.Signature.Value == "" || product.Document.Product.RouteID != fixture.route.RouteID {
		t.Fatalf("product = %#v", product)
	}
	if productResponse.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" {
		t.Fatalf("product cache control = %q", productResponse.Header().Get("Cache-Control"))
	}

	llmsResponse := httptest.NewRecorder()
	handler.ServeHTTP(llmsResponse, httptest.NewRequest(http.MethodGet, "/store/"+fixture.seller.Slug+"/llms.txt", nil))
	if llmsResponse.Code != http.StatusOK || !contains(llmsResponse.Body.String(), "Publication revision: 1") {
		t.Fatalf("llms.txt status = %d, body = %s", llmsResponse.Code, llmsResponse.Body.String())
	}
	if llmsResponse.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" {
		t.Fatalf("llms.txt cache control = %q", llmsResponse.Header().Get("Cache-Control"))
	}
}

func TestHTTPControllerReturnsSignedInactiveTombstone(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.entitlements.response.Assignment.Status = "cancelled"
	fixture.entitlements.response.Assignment.NetworkAccess = "blocked"

	response := httptest.NewRecorder()
	storefrontHandler(fixture.service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/store/"+fixture.seller.Slug+"/manifest.json", nil))
	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var tombstone SignedStorefrontTombstone
	if err := json.Unmarshal(response.Body.Bytes(), &tombstone); err != nil {
		t.Fatal(err)
	}
	if tombstone.Document.Availability != AvailabilityInactive || tombstone.Signature.Value == "" {
		t.Fatalf("tombstone = %#v", tombstone)
	}
}

func TestHTTPControllerReturnsDependencyUnavailableWithoutLeakingCause(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.verifyEndpoint(t)
	fixture.entitlements.err = errors.New("private database failure")

	response := httptest.NewRecorder()
	storefrontHandler(fixture.service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/store/"+fixture.seller.Slug+"/manifest.json", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Body.String() == "" || contains(response.Body.String(), "private database failure") {
		t.Fatalf("unsafe dependency response = %s", response.Body.String())
	}
}

func TestHTTPControllerServesPlatformManifestAndDirectory(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	fixture.verifyEndpoint(t)
	handler := storefrontHandler(fixture.service)

	manifestResponse := httptest.NewRecorder()
	handler.ServeHTTP(manifestResponse, httptest.NewRequest(http.MethodGet, "/.well-known/agentpay", nil))
	if manifestResponse.Code != http.StatusOK {
		t.Fatalf("platform manifest status = %d, body = %s", manifestResponse.Code, manifestResponse.Body.String())
	}
	var platform AgentPayPlatformManifest
	if err := json.Unmarshal(manifestResponse.Body.Bytes(), &platform); err != nil {
		t.Fatal(err)
	}
	if platform.DirectoryEndpoint == "" || platform.Capabilities.Ranking {
		t.Fatalf("platform manifest = %#v", platform)
	}

	directoryResponse := httptest.NewRecorder()
	handler.ServeHTTP(directoryResponse, httptest.NewRequest(http.MethodGet, "/v1/discovery/products?q=research&limit=12", nil))
	if directoryResponse.Code != http.StatusOK {
		t.Fatalf("directory status = %d, body = %s", directoryResponse.Code, directoryResponse.Body.String())
	}
	var page PublicProductDirectoryPage
	if err := json.Unmarshal(directoryResponse.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Product.RouteID != fixture.route.RouteID {
		t.Fatalf("directory page = %#v", page)
	}
}

func TestHTTPControllerRejectsInvalidDirectoryQuery(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	tests := []string{
		"/v1/discovery/products?q=a",
		"/v1/discovery/products?q=",
		"/v1/discovery/products?asset=",
		"/v1/discovery/products?limit=",
	}
	for _, path := range tests {
		response := httptest.NewRecorder()
		storefrontHandler(fixture.service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("path %q status = %d, body = %s", path, response.Code, response.Body.String())
		}
	}
}

func storefrontHandler(service *Service) http.Handler {
	mux := http.NewServeMux()
	NewHTTPController(service).RegisterRoutes(mux)
	return api.Middleware(api.Config{}, mux)
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
