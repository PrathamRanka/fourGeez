package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/health"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/realtime"
)

func TestDependencyHealthReportsConfiguredRuntimeReady(t *testing.T) {
	t.Parallel()

	paymentServer := readinessServer(t, http.StatusOK)
	sellerServer := readinessServer(t, http.StatusOK)
	controller := newTestDependencyHealthController(
		t,
		dependencyHealthConfig{
			RepositoryMode:      "memory",
			PaymentReadinessURL: paymentServer.URL,
			SellerReadinessURL:  sellerServer.URL,
			Timeout:             time.Second,
		},
		memory.NewCatalogRepository(),
		realtime.NewLocalHub(),
		[]byte("seller-signing-secret-at-least-32-bytes"),
	)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, health.ReadyPath, nil)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, body = %s", response.Code, response.Body.String())
	}
	statuses := decodeDependencyStatuses(t, response)
	for _, name := range []string{"persistence", "signing", "payment", "seller_forwarding", "websocket"} {
		if statuses[name] != health.DependencyReady {
			t.Fatalf("readiness response missing ready %s dependency: %s", name, response.Body.String())
		}
	}
}

func TestDependencyHealthFailsClosedForEachUnavailableBoundary(t *testing.T) {
	t.Parallel()

	successServer := readinessServer(t, http.StatusOK)
	failureServer := readinessServer(t, http.StatusServiceUnavailable)
	validSecret := []byte("seller-signing-secret-at-least-32-bytes")
	tests := []struct {
		name       string
		config     dependencyHealthConfig
		catalog    readinessCatalog
		websocket  readinessWebSocketRepository
		secret     []byte
		dependency string
	}{
		{
			name:    "persistence",
			config:  dependencyHealthConfig{RepositoryMode: "memory", PaymentReadinessURL: successServer.URL, SellerReadinessURL: successServer.URL, Timeout: time.Second},
			catalog: failingReadinessCatalog{}, websocket: realtime.NewLocalHub(), secret: validSecret,
			dependency: "persistence",
		},
		{
			name:    "signing",
			config:  dependencyHealthConfig{RepositoryMode: "memory", PaymentReadinessURL: successServer.URL, SellerReadinessURL: successServer.URL, Timeout: time.Second},
			catalog: memory.NewCatalogRepository(), websocket: realtime.NewLocalHub(), secret: []byte("short"),
			dependency: "signing",
		},
		{
			name:    "payment",
			config:  dependencyHealthConfig{RepositoryMode: "memory", PaymentReadinessURL: failureServer.URL, SellerReadinessURL: successServer.URL, Timeout: time.Second},
			catalog: memory.NewCatalogRepository(), websocket: realtime.NewLocalHub(), secret: validSecret,
			dependency: "payment",
		},
		{
			name:    "seller forwarding",
			config:  dependencyHealthConfig{RepositoryMode: "memory", PaymentReadinessURL: successServer.URL, SellerReadinessURL: failureServer.URL, Timeout: time.Second},
			catalog: memory.NewCatalogRepository(), websocket: realtime.NewLocalHub(), secret: validSecret,
			dependency: "seller_forwarding",
		},
		{
			name:    "websocket",
			config:  dependencyHealthConfig{RepositoryMode: "memory", PaymentReadinessURL: successServer.URL, SellerReadinessURL: successServer.URL, Timeout: time.Second},
			catalog: memory.NewCatalogRepository(), websocket: failingReadinessWebSocketRepository{}, secret: validSecret,
			dependency: "websocket",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := newTestDependencyHealthController(t, test.config, test.catalog, test.websocket, test.secret)
			mux := http.NewServeMux()
			controller.RegisterRoutes(mux)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, health.ReadyPath, nil))

			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("readiness status = %d, body = %s", response.Code, response.Body.String())
			}
			if decodeDependencyStatuses(t, response)[test.dependency] != health.DependencyUnavailable {
				t.Fatalf("readiness response missing unavailable %s dependency: %s", test.dependency, response.Body.String())
			}
		})
	}
}

func TestDependencyHealthRejectsUnsafeProbeConfiguration(t *testing.T) {
	t.Parallel()

	dependencies := dependencyHealthDependencies{
		Catalog:             memory.NewCatalogRepository(),
		EvidenceSigner:      mustEvidenceSigner(t),
		SellerSigner:        proxy.NewHMACSigner(proxy.NewLocalSecretProvider([]byte("seller-signing-secret-at-least-32-bytes")), domain.SystemClock{}),
		SellerSigningSecret: []byte("seller-signing-secret-at-least-32-bytes"),
		WebSocketRepository: realtime.NewLocalHub(),
	}
	for _, config := range []dependencyHealthConfig{
		{RepositoryMode: "dynamodb", PaymentReadinessURL: "http://127.0.0.1/verify", SellerReadinessURL: "http://127.0.0.1/research", Timeout: time.Second},
		{RepositoryMode: "memory", PaymentReadinessURL: "", SellerReadinessURL: "http://127.0.0.1/research", Timeout: time.Second},
		{RepositoryMode: "memory", PaymentReadinessURL: "file:///tmp/payment", SellerReadinessURL: "http://127.0.0.1/research", Timeout: time.Second},
		{RepositoryMode: "memory", PaymentReadinessURL: "http://127.0.0.1/verify", SellerReadinessURL: "http://user:pass@127.0.0.1/research", Timeout: time.Second},
	} {
		if _, err := newDependencyHealthController(config, dependencies); err == nil {
			t.Fatalf("newDependencyHealthController(%+v) error = nil", config)
		}
	}
}

func newTestDependencyHealthController(
	t *testing.T,
	config dependencyHealthConfig,
	catalogRepository readinessCatalog,
	webSocketRepository readinessWebSocketRepository,
	sellerSigningSecret []byte,
) *health.Controller {
	t.Helper()
	controller, err := newDependencyHealthController(config, dependencyHealthDependencies{
		Catalog:             catalogRepository,
		EvidenceSigner:      mustEvidenceSigner(t),
		SellerSigner:        proxy.NewHMACSigner(proxy.NewLocalSecretProvider(sellerSigningSecret), domain.SystemClock{}),
		SellerSigningSecret: sellerSigningSecret,
		WebSocketRepository: webSocketRepository,
	})
	if err != nil {
		t.Fatal(err)
	}
	return controller
}

func mustEvidenceSigner(t *testing.T) evidence.Signer {
	t.Helper()
	signer, err := evidence.NewLocalHMACSigner("health-test-key", []byte("evidence-signing-secret-at-least-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func readinessServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("readiness method = %s, want POST", request.Method)
		}
		response.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server
}

func decodeDependencyStatuses(t *testing.T, response *httptest.ResponseRecorder) map[string]health.DependencyStatus {
	t.Helper()
	var report health.Report
	if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	statuses := make(map[string]health.DependencyStatus, len(report.Dependencies))
	for _, dependency := range report.Dependencies {
		statuses[dependency.Name] = dependency.Status
	}
	return statuses
}

type failingReadinessCatalog struct{}

func (failingReadinessCatalog) ListRoutesBySeller(context.Context, domain.ID) ([]catalog.PaidRoute, error) {
	return nil, errors.New("persistence unavailable")
}

type failingReadinessWebSocketRepository struct{}

func (failingReadinessWebSocketRepository) ListBySession(context.Context, domain.ID) ([]realtime.Connection, error) {
	return nil, errors.New("websocket repository unavailable")
}
