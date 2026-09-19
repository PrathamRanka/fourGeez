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
	"github.com/fourgeez/agentpay/internal/audit"
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
		`{"displayName":"Research Report","productSlug":"research-report","method":"POST","pathPattern":"/research","description":"Research","mimeType":"application/json","amount":"35000000","asset":"test-usdc","network":"test-network","payTo":"0x123","upstreamTimeoutSeconds":20}`,
	)
	if routeResponse.Code != http.StatusCreated {
		t.Fatalf("route status = %d, body = %s", routeResponse.Code, routeResponse.Body.String())
	}

	var route catalog.PaidRoute
	decodeCatalogResponse(t, routeResponse, &route)
	if route.DisplayName != "Research Report" || route.ProductSlug != "research-report" {
		t.Fatalf("product identity = %q/%q", route.DisplayName, route.ProductSlug)
	}
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

func TestSellerServiceActivationUnlocksES256ReadinessWithoutSharedSecret(t *testing.T) {
	t.Parallel()

	handler, repository := newCatalogHandlerWithRepository(t)
	created := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers",
		"seller-service-create",
		`{"name":"Demo","slug":"service-demo","upstreamBaseUrl":"https://seller.example"}`,
	)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	var seller catalog.SellerResponse
	decodeCatalogResponse(t, created, &seller)

	body := `{"expectedVersion":1}`
	activatedResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/service-activation",
		"seller-service-activate",
		body,
	)
	if activatedResponse.Code != http.StatusOK {
		t.Fatalf("activation status = %d, body = %s", activatedResponse.Code, activatedResponse.Body.String())
	}
	if activatedResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", activatedResponse.Header().Get("Cache-Control"))
	}
	var activated catalog.SellerResponse
	decodeCatalogResponse(t, activatedResponse, &activated)
	if activated.Status != catalog.SellerStatusActive || activated.Version != 2 {
		t.Fatalf("activated seller = %#v", activated)
	}
	stored, err := repository.GetSeller(t.Context(), seller.SellerID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SigningSecretRef != "" {
		t.Fatalf("production activation created a legacy signing secret reference: %q", stored.SigningSecretRef)
	}

	replay := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/service-activation",
		"seller-service-activate",
		body,
	)
	if replay.Code != http.StatusOK || replay.Body.String() != activatedResponse.Body.String() {
		t.Fatalf("idempotent replay = %d %s", replay.Code, replay.Body.String())
	}
}

func TestSellerRouteCreationRejectsNormalizedProductSlugCollision(t *testing.T) {
	t.Parallel()

	handler := newCatalogHandler(t)
	sellerResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers",
		"seller-product-slug-create",
		`{"name":"Demo","slug":"product-slug-demo","upstreamBaseUrl":"https://seller.example"}`,
	)
	var seller catalog.SellerResponse
	decodeCatalogResponse(t, sellerResponse, &seller)

	first := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes",
		"product-slug-first",
		`{"displayName":"Market Report","productSlug":"Market_Report","method":"POST","pathPattern":"/research","description":"Research","mimeType":"application/json","amount":"35000000","asset":"test-usdc","network":"test-network","payTo":"0x123","upstreamTimeoutSeconds":20}`,
	)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	second := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes",
		"product-slug-second",
		`{"displayName":"Another Report","productSlug":"market--report","method":"POST","pathPattern":"/research/another","description":"Another report","mimeType":"application/json","amount":"35000000","asset":"test-usdc","network":"test-network","payTo":"0x123","upstreamTimeoutSeconds":20}`,
	)
	if second.Code != http.StatusConflict {
		t.Fatalf("second status = %d, want %d, body = %s", second.Code, http.StatusConflict, second.Body.String())
	}
}

// TestSellerRouteManagementLifecycle verifies dashboard route reads and guarded controls.
func TestSellerRouteManagementLifecycle(t *testing.T) {
	t.Parallel()

	handler, repository := newCatalogHandlerWithRepository(t)
	sellerResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers",
		"seller-lifecycle-create",
		`{"name":"Lifecycle Demo","slug":"lifecycle-demo","upstreamBaseUrl":"https://seller.example"}`,
	)
	var seller catalog.SellerResponse
	decodeCatalogResponse(t, sellerResponse, &seller)

	draftResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes",
		"route-draft-create",
		`{"displayName":"Research Report","productSlug":"research-report","method":"POST","pathPattern":"/research","description":"Research","mimeType":"application/json","amount":"35000000","asset":"test-usdc","network":"test-network","payTo":"0x123","upstreamTimeoutSeconds":20,"publishImmediately":false}`,
	)
	if draftResponse.Code != http.StatusCreated {
		t.Fatalf("draft status = %d, body = %s", draftResponse.Code, draftResponse.Body.String())
	}
	var draft catalog.PaidRoute
	decodeCatalogResponse(t, draftResponse, &draft)
	if draft.LifecycleStatus != catalog.RouteLifecycleDraft || draft.Enabled {
		t.Fatalf("draft route = %#v", draft)
	}

	listResponse := performCatalogRequest(
		t,
		handler,
		http.MethodGet,
		"/v1/sellers/"+seller.SellerID.String()+"/routes",
		"",
		"",
	)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	var routeList catalog.PaidRouteList
	decodeCatalogResponse(t, listResponse, &routeList)
	if len(routeList.Items) != 1 || routeList.Items[0].RouteID != draft.RouteID {
		t.Fatalf("route list = %#v", routeList)
	}

	storedSeller, err := repository.GetSeller(t.Context(), seller.SellerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := storedSeller.Activate(
		"secret/seller/lifecycle-demo",
		domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 1, 0, 0, time.UTC)),
	); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateSeller(
		t.Context(),
		storedSeller,
		seller.Version,
	); err != nil {
		t.Fatal(err)
	}

	validationResponse := performCatalogRequest(
		t,
		handler,
		http.MethodGet,
		"/v1/sellers/"+seller.SellerID.String()+"/routes/"+draft.RouteID.String()+"/validation",
		"",
		"",
	)
	if validationResponse.Code != http.StatusOK {
		t.Fatalf("validation status = %d, body = %s", validationResponse.Code, validationResponse.Body.String())
	}
	var validation catalog.RouteValidationResult
	decodeCatalogResponse(t, validationResponse, &validation)
	if !validation.Valid {
		t.Fatalf("validation = %#v", validation)
	}
	if validation.ContractHash == "" {
		t.Fatal("validation response omitted contractHash")
	}
	stalePublishResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes/"+draft.RouteID.String()+"/publish",
		"route-publish-stale",
		`{"expectedVersion":1,"contractHash":"`+strings.Repeat("0", 64)+`"}`,
	)
	if stalePublishResponse.Code != http.StatusConflict || !strings.Contains(stalePublishResponse.Body.String(), `"code":"route_contract_stale"`) {
		t.Fatalf("stale publish status = %d, body = %s", stalePublishResponse.Code, stalePublishResponse.Body.String())
	}

	publishedResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes/"+draft.RouteID.String()+"/publish",
		"route-publish",
		`{"expectedVersion":1,"contractHash":"`+validation.ContractHash+`"}`,
	)
	if publishedResponse.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishedResponse.Code, publishedResponse.Body.String())
	}
	var published catalog.PaidRoute
	decodeCatalogResponse(t, publishedResponse, &published)
	if published.LifecycleStatus != catalog.RouteLifecyclePublished || !published.Enabled {
		t.Fatalf("published route = %#v", published)
	}

	pausedResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes/"+draft.RouteID.String()+"/pause",
		"route-pause",
		`{"expectedVersion":2}`,
	)
	if pausedResponse.Code != http.StatusOK {
		t.Fatalf("pause status = %d, body = %s", pausedResponse.Code, pausedResponse.Body.String())
	}
	var paused catalog.PaidRoute
	decodeCatalogResponse(t, pausedResponse, &paused)
	if paused.LifecycleStatus != catalog.RouteLifecyclePaused || paused.Enabled {
		t.Fatalf("paused route = %#v", paused)
	}

	archivedResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes/"+draft.RouteID.String()+"/archive",
		"route-archive",
		`{"expectedVersion":3}`,
	)
	if archivedResponse.Code != http.StatusOK {
		t.Fatalf("archive status = %d, body = %s", archivedResponse.Code, archivedResponse.Body.String())
	}
	var archived catalog.PaidRoute
	decodeCatalogResponse(t, archivedResponse, &archived)
	if archived.LifecycleStatus != catalog.RouteLifecycleArchived {
		t.Fatalf("archived route = %#v", archived)
	}

	invalidResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes/"+draft.RouteID.String()+"/publish",
		"route-invalid-republish",
		`{"expectedVersion":4}`,
	)
	if invalidResponse.Code != http.StatusConflict {
		t.Fatalf("invalid transition status = %d, want %d", invalidResponse.Code, http.StatusConflict)
	}
}

// TestSellerEmergencyDisableRoute verifies the urgent route stop remains distinct from pause.
func TestSellerEmergencyDisableRoute(t *testing.T) {
	t.Parallel()

	handler := newCatalogHandler(t)
	sellerResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers",
		"seller-emergency-create",
		`{"name":"Emergency Demo","slug":"emergency-demo","upstreamBaseUrl":"https://seller.example"}`,
	)
	var seller catalog.SellerResponse
	decodeCatalogResponse(t, sellerResponse, &seller)
	routeResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes",
		"route-emergency-create",
		`{"displayName":"Research Report","productSlug":"research-report","method":"POST","pathPattern":"/research","description":"Research","mimeType":"application/json","amount":"35000000","asset":"test-usdc","network":"test-network","payTo":"0x123","upstreamTimeoutSeconds":20}`,
	)
	var route catalog.PaidRoute
	decodeCatalogResponse(t, routeResponse, &route)

	disableResponse := performCatalogRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/sellers/"+seller.SellerID.String()+"/routes/"+route.RouteID.String()+"/emergency-disable",
		"route-emergency-disable",
		`{"expectedVersion":1}`,
	)
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", disableResponse.Code, disableResponse.Body.String())
	}
	var disabled catalog.PaidRoute
	decodeCatalogResponse(t, disableResponse, &disabled)
	if disabled.LifecycleStatus != catalog.RouteLifecycleEmergencyDisabled || disabled.Enabled {
		t.Fatalf("disabled route = %#v", disabled)
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
		`{"displayName":"Research Report","productSlug":"research-report","method":"POST","pathPattern":"/research","description":"Research","mimeType":"application/json","amount":"35000000","asset":"test-usdc","network":"test-network","payTo":"0x123","upstreamTimeoutSeconds":20}`,
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

	handler, _ := newCatalogHandlerWithRepository(t)
	return handler
}

// newCatalogHandlerWithRepository exposes persistence for lifecycle fixture setup.
func newCatalogHandlerWithRepository(
	t *testing.T,
) (http.Handler, *memory.CatalogRepository) {
	t.Helper()

	repository := memory.NewCatalogRepository()
	idempotencyStore := memory.NewIdempotencyStore()
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	service := catalog.NewService(
		repository,
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("a", 256))),
		clock,
		audit.NoopRecorder{},
	)
	controller := catalog.NewHTTPController(service, idempotencyStore)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	handler := api.Middleware(
		api.Config{Authenticator: api.NewStaticAuthenticator("seller-secret", "agent-secret")},
		mux,
	)
	return handler, repository
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
