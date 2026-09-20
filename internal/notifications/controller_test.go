package notifications

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
	"github.com/fourgeez/agentpay/internal/domain"
)

func TestWebhookSubscriptionHTTPOverlapRotationDisablesPredecessor(t *testing.T) {
	t.Parallel()
	sellerID := mustNotificationID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	repository := newNotificationRepository()
	clock := domain.FixedClock{Value: time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)}
	service := NewService(repository, deliveryControllerAuthorizer{sellerID: sellerID}, domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("w", 256))), fixedWebhookSecretGenerator{secret: strings.Repeat("s", 43)}, &notificationSecretStore{secrets: make(map[string][]byte)}, clock, audit.NoopRecorder{})
	controller := NewHTTPController(service, newDeliveryIdempotencyStore())
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	handler := api.Middleware(api.Config{Authenticator: api.NewStaticAuthenticator("seller-token", "agent-token")}, mux)

	create := performWebhookRequest(handler, http.MethodPost, "/v1/sellers/"+sellerID.String()+"/webhook-subscriptions", "webhook-create-1", `{"endpointUrl":"https://seller.example/webhook","eventTypes":["payment.verified"]}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", create.Code, create.Body.String())
	}
	var created SubscriptionCreated
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	disable := performWebhookRequest(handler, http.MethodPost, "/v1/sellers/"+sellerID.String()+"/webhook-subscriptions/"+created.SubscriptionID.String()+"/disable", "webhook-disable-1", `{"expectedVersion":1}`)
	if disable.Code != http.StatusOK || !strings.Contains(disable.Body.String(), `"status":"disabled"`) {
		t.Fatalf("disable = %d %s", disable.Code, disable.Body.String())
	}
	replay := performWebhookRequest(handler, http.MethodPost, "/v1/sellers/"+sellerID.String()+"/webhook-subscriptions/"+created.SubscriptionID.String()+"/disable", "webhook-disable-1", `{"expectedVersion":1}`)
	if replay.Code != http.StatusOK || replay.Body.String() != disable.Body.String() {
		t.Fatalf("replay = %d %s", replay.Code, replay.Body.String())
	}
}

func performWebhookRequest(handler http.Handler, method, target, idempotencyKey, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer seller-token")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
