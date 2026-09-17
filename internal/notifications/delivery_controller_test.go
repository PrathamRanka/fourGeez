package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestDeliveryRoutesListHistoryAndRedeliverDeadLetter verifies EVT-002 HTTP behavior.
func TestDeliveryRoutesListHistoryAndRedeliverDeadLetter(t *testing.T) {
	t.Parallel()

	sellerID := mustNotificationID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	deliveryRepository := newDeliveryRepository()
	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	delivery, err := NewDelivery(validDeliveryParams(t, createdAt))
	if err != nil {
		t.Fatal(err)
	}
	if err := delivery.RecordFailure(
		createdAt,
		DeliveryFailurePermanentResponse,
		http.StatusBadRequest,
		strings.Repeat("a", 64),
	); err != nil {
		t.Fatal(err)
	}
	if _, _, err := deliveryRepository.CreateIfAbsent(t.Context(), delivery); err != nil {
		t.Fatal(err)
	}
	service := NewDeliveryService(
		deliveryRepository,
		newNotificationRepository(),
		deliveryControllerAuthorizer{sellerID: sellerID},
		domain.NewULIDGenerator(nil, nil),
		&deliverySigner{},
		&deliverySender{},
		domain.FixedClock{Value: createdAt.Add(time.Hour).Time()},
	)
	mux := http.NewServeMux()
	NewDeliveryHTTPController(
		service,
		newDeliveryIdempotencyStore(),
	).RegisterRoutes(mux)
	handler := api.Middleware(api.Config{
		Authenticator: api.NewStaticAuthenticator("seller-token", "agent-key"),
	}, mux)

	listResponse := performDeliveryRequest(
		handler,
		http.MethodGet,
		"/v1/sellers/"+sellerID.String()+"/webhook-deliveries?limit=10",
		"",
	)
	if listResponse.Code != http.StatusOK ||
		!strings.Contains(listResponse.Body.String(), delivery.DeliveryID().String()) {
		t.Fatalf("list response = %d %s", listResponse.Code, listResponse.Body.String())
	}

	redeliveryResponse := performDeliveryRequest(
		handler,
		http.MethodPost,
		"/v1/sellers/"+sellerID.String()+"/webhook-deliveries/"+
			delivery.DeliveryID().String()+"/redeliver",
		"delivery-redelivery-1",
	)
	if redeliveryResponse.Code != http.StatusOK ||
		!strings.Contains(redeliveryResponse.Body.String(), `"status":"pending"`) {
		t.Fatalf("redelivery response = %d %s", redeliveryResponse.Code, redeliveryResponse.Body.String())
	}

	replayResponse := performDeliveryRequest(
		handler,
		http.MethodPost,
		"/v1/sellers/"+sellerID.String()+"/webhook-deliveries/"+
			delivery.DeliveryID().String()+"/redeliver",
		"delivery-redelivery-1",
	)
	if replayResponse.Code != http.StatusOK || replayResponse.Body.String() != redeliveryResponse.Body.String() {
		t.Fatalf("replay response = %d %s", replayResponse.Code, replayResponse.Body.String())
	}
}

type deliveryControllerAuthorizer struct {
	sellerID domain.ID
}

// AuthorizeSeller verifies the local seller principal used by HTTP middleware.
func (authorizer deliveryControllerAuthorizer) AuthorizeSeller(
	_ context.Context,
	ownerSubject string,
	sellerID domain.ID,
) error {
	if ownerSubject != "local-seller" || sellerID != authorizer.sellerID {
		return ErrDeliveryNotFound
	}
	return nil
}

type deliveryIdempotencyStore struct {
	records map[string]domain.IdempotencyRecord
}

// newDeliveryIdempotencyStore creates an empty controller-test idempotency store.
func newDeliveryIdempotencyStore() *deliveryIdempotencyStore {
	return &deliveryIdempotencyStore{
		records: make(map[string]domain.IdempotencyRecord),
	}
}

// Load returns one stored idempotency response.
func (store *deliveryIdempotencyStore) Load(
	_ context.Context,
	scope string,
	key domain.IdempotencyKey,
) (domain.IdempotencyRecord, bool, error) {
	record, exists := store.records[scope+"\x00"+string(key)]
	return record, exists, nil
}

// SaveIfAbsent stores one response when its scope and key are unused.
func (store *deliveryIdempotencyStore) SaveIfAbsent(
	_ context.Context,
	record domain.IdempotencyRecord,
) (bool, error) {
	key := record.Scope + "\x00" + string(record.Key)
	if _, exists := store.records[key]; exists {
		return false, nil
	}
	store.records[key] = record
	return true, nil
}

// performDeliveryRequest executes one seller-authenticated delivery request.
func performDeliveryRequest(
	handler http.Handler,
	method string,
	target string,
	idempotencyKey string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer seller-token")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
