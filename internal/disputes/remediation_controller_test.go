package disputes

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestManualRefundHTTPControllerRecordsSellerScopedRefundIdempotently(t *testing.T) {
	t.Parallel()
	dispute := mustRefundDispute(t)
	transaction := mustRefundTransaction(t)
	service := NewManualRemediationService(
		&refundDisputeRepository{dispute: dispute},
		&refundTransactionRepository{transaction: transaction},
		&memoryRefundRecordRepository{},
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 5, 0, 0, time.UTC)},
	)
	mux := http.NewServeMux()
	NewManualRemediationHTTPController(service, newRemediationIdempotencyStore()).RegisterRoutes(mux)
	handler := api.Middleware(api.Config{
		Authenticator:    refundAuthenticator{},
		SellerAuthorizer: refundSellerAuthorizer{sellerID: transaction.SellerID()},
	}, mux)
	body := []byte(`{"amount":"100","asset":"USDC","network":"eip155:84532","reference":"0xrefund-reference"}`)
	path := "/v1/sellers/" + transaction.SellerID().String() + "/disputes/" + dispute.DisputeID.String() + "/refund-records"
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer seller-secret")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "refund-record-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	replay := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	replay.Header = request.Header.Clone()
	replayResponse := httptest.NewRecorder()
	handler.ServeHTTP(replayResponse, replay)
	if replayResponse.Code != http.StatusCreated || replayResponse.Body.String() != response.Body.String() {
		t.Fatalf("replay = %d %s", replayResponse.Code, replayResponse.Body.String())
	}
}

type remediationIdempotencyStore struct {
	records map[string]domain.IdempotencyRecord
}

func newRemediationIdempotencyStore() *remediationIdempotencyStore {
	return &remediationIdempotencyStore{records: make(map[string]domain.IdempotencyRecord)}
}

func (store *remediationIdempotencyStore) Load(_ context.Context, scope string, key domain.IdempotencyKey) (domain.IdempotencyRecord, bool, error) {
	record, found := store.records[scope+"\x00"+string(key)]
	return record, found, nil
}

func (store *remediationIdempotencyStore) SaveIfAbsent(_ context.Context, record domain.IdempotencyRecord) (bool, error) {
	key := record.Scope + "\x00" + string(record.Key)
	if _, found := store.records[key]; found {
		return false, nil
	}
	store.records[key] = record
	return true, nil
}

type refundAuthenticator struct{}

func (refundAuthenticator) AuthenticateSeller(context.Context, string) (api.Principal, bool) {
	return api.Principal{Kind: api.PrincipalSeller, Subject: "seller-owner"}, true
}
func (refundAuthenticator) AuthenticateAgent(context.Context, string) (api.Principal, bool) {
	return api.Principal{}, false
}

type refundSellerAuthorizer struct{ sellerID domain.ID }

func (authorizer refundSellerAuthorizer) AuthorizeSeller(_ context.Context, _ string, sellerID domain.ID) error {
	if sellerID != authorizer.sellerID {
		return persistence.ErrNotFound
	}
	return nil
}
