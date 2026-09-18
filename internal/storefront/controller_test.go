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
