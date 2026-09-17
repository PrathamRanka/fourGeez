package approvals

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// TestApprovalRoutesCompleteTwoPersonApproval verifies the API-005 happy path.
func TestApprovalRoutesCompleteTwoPersonApproval(t *testing.T) {
	t.Parallel()

	handler, purchaseIntent, _ := newApprovalHandler(t)
	createBody := `{"approvers":[{"label":"Finance"},{"label":"Security"}]}`
	createResponse := performApprovalRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/intents/"+purchaseIntent.IntentID().String()+"/approval-sessions",
		createBody,
		map[string]string{
			"Content-Type":     "application/json",
			api.AgentKeyHeader: "agent-secret",
			"Idempotency-Key":  "approval-create-1",
		},
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}

	var created SessionResponse
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("create response JSON error = %v", err)
	}
	if len(created.Invitations) != requiredApprovalsForHackathon {
		t.Fatalf("invitation count = %d, want %d", len(created.Invitations), requiredApprovalsForHackathon)
	}
	if created.IntentHash != purchaseIntent.IntentHash() || created.Status != SessionStatusPending {
		t.Fatalf("created session = %#v", created)
	}
	if createResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("create response did not prevent sensitive response caching")
	}

	firstToken := invitationToken(t, created.Invitations[0].URL)
	secondToken := invitationToken(t, created.Invitations[1].URL)
	getResponse := performApprovalRequest(
		t,
		handler,
		http.MethodGet,
		"/v1/approval-sessions/"+created.SessionID.String()+"?token="+url.QueryEscape(firstToken),
		"",
		nil,
	)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("invitation get status = %d, body = %s", getResponse.Code, getResponse.Body.String())
	}

	firstDecision := performApprovalRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/approval-sessions/"+created.SessionID.String()+"/decisions?token="+url.QueryEscape(firstToken),
		`{"decision":"approve"}`,
		map[string]string{
			"Content-Type":    "application/json",
			"Idempotency-Key": "approval-decision-1",
		},
	)
	if firstDecision.Code != http.StatusOK {
		t.Fatalf("first decision status = %d, body = %s", firstDecision.Code, firstDecision.Body.String())
	}
	var firstSnapshot SessionResponse
	if err := json.Unmarshal(firstDecision.Body.Bytes(), &firstSnapshot); err != nil {
		t.Fatalf("first decision JSON error = %v", err)
	}
	if firstSnapshot.Status != SessionStatusPending || firstSnapshot.ApprovalToken != "" {
		t.Fatalf("first decision snapshot = %#v", firstSnapshot)
	}

	replayResponse := performApprovalRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/approval-sessions/"+created.SessionID.String()+"/decisions?token="+url.QueryEscape(firstToken),
		`{"decision":"approve"}`,
		map[string]string{
			"Content-Type":    "application/json",
			"Idempotency-Key": "approval-decision-1",
		},
	)
	if replayResponse.Code != http.StatusOK || replayResponse.Body.String() != firstDecision.Body.String() {
		t.Fatal("idempotent decision did not replay the original response")
	}

	secondDecision := performApprovalRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/approval-sessions/"+created.SessionID.String()+"/decisions?token="+url.QueryEscape(secondToken),
		`{"decision":"approve"}`,
		map[string]string{
			"Content-Type":    "application/json",
			"Idempotency-Key": "approval-decision-2",
		},
	)
	if secondDecision.Code != http.StatusOK {
		t.Fatalf("second decision status = %d, body = %s", secondDecision.Code, secondDecision.Body.String())
	}
	var approved SessionResponse
	if err := json.Unmarshal(secondDecision.Body.Bytes(), &approved); err != nil {
		t.Fatalf("second decision JSON error = %v", err)
	}
	if approved.Status != SessionStatusApproved || approved.ApprovalToken == "" {
		t.Fatalf("approved snapshot = %#v", approved)
	}

	finalGet := performApprovalRequest(
		t,
		handler,
		http.MethodGet,
		"/v1/approval-sessions/"+created.SessionID.String(),
		"",
		map[string]string{api.AgentKeyHeader: "agent-secret"},
	)
	if finalGet.Code != http.StatusOK {
		t.Fatalf("final get status = %d, body = %s", finalGet.Code, finalGet.Body.String())
	}
	var finalSnapshot SessionResponse
	if err := json.Unmarshal(finalGet.Body.Bytes(), &finalSnapshot); err != nil {
		t.Fatalf("final get JSON error = %v", err)
	}
	if finalSnapshot.ApprovalToken != "" || len(finalSnapshot.Invitations) != 0 {
		t.Fatal("sensitive one-time values were returned by the snapshot endpoint")
	}
}

// TestApprovalRoutesRejectInvalidRequests verifies documented approval failures.
func TestApprovalRoutesRejectInvalidRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		request    func(*testing.T, http.Handler, intents.PurchaseIntent, *approvalAPITestClock) *httptest.ResponseRecorder
		wantStatus int
	}{
		{
			name: "unknown create field",
			request: func(t *testing.T, handler http.Handler, purchaseIntent intents.PurchaseIntent, _ *approvalAPITestClock) *httptest.ResponseRecorder {
				return performApprovalRequest(t, handler, http.MethodPost, "/v1/intents/"+purchaseIntent.IntentID().String()+"/approval-sessions", `{"approvers":[{"label":"Finance"},{"label":"Security"}],"extra":true}`, map[string]string{"Content-Type": "application/json", api.AgentKeyHeader: "agent-secret", "Idempotency-Key": "approval-create-2"})
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "wrong invitation token",
			request: func(t *testing.T, handler http.Handler, purchaseIntent intents.PurchaseIntent, _ *approvalAPITestClock) *httptest.ResponseRecorder {
				created := createApprovalSession(t, handler, purchaseIntent, "approval-create-3")
				return performApprovalRequest(t, handler, http.MethodGet, "/v1/approval-sessions/"+created.SessionID.String()+"?token=wrong-token", "", nil)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "duplicate decision",
			request: func(t *testing.T, handler http.Handler, purchaseIntent intents.PurchaseIntent, _ *approvalAPITestClock) *httptest.ResponseRecorder {
				created := createApprovalSession(t, handler, purchaseIntent, "approval-create-4")
				token := invitationToken(t, created.Invitations[0].URL)
				path := "/v1/approval-sessions/" + created.SessionID.String() + "/decisions?token=" + url.QueryEscape(token)
				performApprovalRequest(t, handler, http.MethodPost, path, `{"decision":"approve"}`, map[string]string{"Content-Type": "application/json", "Idempotency-Key": "approval-decision-3"})
				return performApprovalRequest(t, handler, http.MethodPost, path, `{"decision":"approve"}`, map[string]string{"Content-Type": "application/json", "Idempotency-Key": "approval-decision-4"})
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "expired invitation",
			request: func(t *testing.T, handler http.Handler, purchaseIntent intents.PurchaseIntent, clock *approvalAPITestClock) *httptest.ResponseRecorder {
				created := createApprovalSession(t, handler, purchaseIntent, "approval-create-5")
				token := invitationToken(t, created.Invitations[0].URL)
				clock.value = purchaseIntent.ExpiresAt().Time()
				path := "/v1/approval-sessions/" + created.SessionID.String() + "/decisions?token=" + url.QueryEscape(token)
				return performApprovalRequest(t, handler, http.MethodPost, path, `{"decision":"approve"}`, map[string]string{"Content-Type": "application/json", "Idempotency-Key": "approval-decision-5"})
			},
			wantStatus: http.StatusGone,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, purchaseIntent, clock := newApprovalHandler(t)
			response := test.request(t, handler, purchaseIntent, clock)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

// newApprovalHandler creates a seeded API-005 test server.
func newApprovalHandler(t *testing.T) (http.Handler, intents.PurchaseIntent, *approvalAPITestClock) {
	t.Helper()

	clock := &approvalAPITestClock{
		value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	intentRepository := &approvalAPITestIntentRepository{}
	purchaseIntent, err := intents.NewPurchaseIntent(intents.PurchaseIntentParams{
		IntentID:         mustApprovalID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		SellerID:         mustApprovalID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		RouteID:          mustApprovalID(t, "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.RouteIDPrefix),
		BuyerID:          "local-agent",
		RequestMethod:    intents.RequestMethodPost,
		RequestPath:      "/research",
		RequestBodyHash:  mustApprovalDigest(t, strings.Repeat("a", 64)),
		Amount:           domain.MustParseAmount("35000000"),
		Asset:            "test-usdc",
		Network:          "test-network",
		MaximumAmount:    domain.MustParseAmount("40000000"),
		RequiresApproval: true,
		CreatedAt:        domain.NewTimestamp(clock.Now()),
		ExpiresAt:        domain.NewTimestamp(clock.Now().Add(10 * time.Minute)),
	})
	if err != nil {
		t.Fatal(err)
	}
	intentRepository.purchaseIntent = purchaseIntent
	signer, err := NewApprovalTokenSigner([]byte(strings.Repeat("s", 32)))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(
		&approvalAPITestRepository{sessions: make(map[domain.ID]Session)},
		intentRepository,
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("b", 256))),
		&sequenceTokenGenerator{tokens: []string{
			"invite-finance-secret",
			"invite-security-secret",
			"invite-finance-secret-2",
			"invite-security-secret-2",
		}},
		signer,
		clock,
		"http://localhost:3000",
	)
	controller := NewHTTPController(
		service,
		&approvalAPITestIdempotencyStore{
			records: make(map[string]domain.IdempotencyRecord),
		},
	)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	return api.Middleware(
		api.Config{
			Authenticator: api.NewStaticAuthenticator("seller-secret", "agent-secret"),
		},
		mux,
	), purchaseIntent, clock
}

// createApprovalSession creates one session and returns its decoded response.
func createApprovalSession(
	t *testing.T,
	handler http.Handler,
	purchaseIntent intents.PurchaseIntent,
	idempotencyKey string,
) SessionResponse {
	t.Helper()
	response := performApprovalRequest(
		t,
		handler,
		http.MethodPost,
		"/v1/intents/"+purchaseIntent.IntentID().String()+"/approval-sessions",
		`{"approvers":[{"label":"Finance"},{"label":"Security"}]}`,
		map[string]string{
			"Content-Type":     "application/json",
			api.AgentKeyHeader: "agent-secret",
			"Idempotency-Key":  idempotencyKey,
		},
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", response.Code, response.Body.String())
	}
	var created SessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created
}

// performApprovalRequest sends one request through the complete API middleware.
func performApprovalRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// invitationToken extracts the one-time token from an invitation URL.
func invitationToken(t *testing.T, invitationURL string) string {
	t.Helper()
	parsedURL, err := url.Parse(invitationURL)
	if err != nil {
		t.Fatal(err)
	}
	return parsedURL.Query().Get("token")
}

// mustApprovalDigest parses a test digest.
func mustApprovalDigest(t *testing.T, raw string) intents.SHA256Digest {
	t.Helper()
	digest, err := intents.ParseSHA256Digest(raw)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

// approvalAPITestClock allows an HTTP test to advance the current time.
type approvalAPITestClock struct {
	value time.Time
}

// Now returns the current test-controlled instant.
func (clock *approvalAPITestClock) Now() time.Time {
	return clock.value
}

// approvalAPITestIntentRepository returns the seeded immutable intent.
type approvalAPITestIntentRepository struct {
	purchaseIntent intents.PurchaseIntent
}

// Get loads the seeded purchase intent by identifier.
func (repository *approvalAPITestIntentRepository) Get(
	_ context.Context,
	intentID domain.ID,
) (intents.PurchaseIntent, error) {
	if repository.purchaseIntent.IntentID() != intentID {
		return intents.PurchaseIntent{}, persistence.ErrNotFound
	}
	return repository.purchaseIntent, nil
}

// approvalAPITestRepository stores isolated approval-session snapshots.
type approvalAPITestRepository struct {
	sessions map[domain.ID]Session
}

// Create persists a new approval session for an HTTP test.
func (repository *approvalAPITestRepository) Create(
	_ context.Context,
	session Session,
) error {
	if _, exists := repository.sessions[session.SessionID()]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.sessions[session.SessionID()] = session.Clone()
	return nil
}

// Get loads an isolated approval session for an HTTP test.
func (repository *approvalAPITestRepository) Get(
	_ context.Context,
	sessionID domain.ID,
) (Session, error) {
	session, exists := repository.sessions[sessionID]
	if !exists {
		return Session{}, persistence.ErrNotFound
	}
	return session.Clone(), nil
}

// Update applies one optimistic approval-session transition.
func (repository *approvalAPITestRepository) Update(
	_ context.Context,
	session Session,
	expectedVersion uint64,
) error {
	stored, exists := repository.sessions[session.SessionID()]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Version() != expectedVersion || session.Version() != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	repository.sessions[session.SessionID()] = session.Clone()
	return nil
}

// approvalAPITestIdempotencyStore stores mutation responses by scope and key.
type approvalAPITestIdempotencyStore struct {
	records map[string]domain.IdempotencyRecord
}

// Load returns a previously stored idempotency response.
func (store *approvalAPITestIdempotencyStore) Load(
	_ context.Context,
	scope string,
	key domain.IdempotencyKey,
) (domain.IdempotencyRecord, bool, error) {
	record, found := store.records[scope+"\x00"+string(key)]
	return record, found, nil
}

// SaveIfAbsent persists the first response for an idempotency scope and key.
func (store *approvalAPITestIdempotencyStore) SaveIfAbsent(
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
