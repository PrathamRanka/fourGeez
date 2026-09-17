package audit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

// TestHTTPControllerReturnsAuthorizedBoundedHistory verifies the public audit API.
func TestHTTPControllerReturnsAuthorizedBoundedHistory(t *testing.T) {
	t.Parallel()

	sellerID := mustControllerAuditID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		domain.SellerIDPrefix,
	)
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC),
	}
	repository := memory.NewAuditEventRepository()
	service := audit.NewService(
		repository,
		controllerAuditAuthorizer{sellerID: sellerID},
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("a", 128))),
		clock,
	)
	if err := service.Record(t.Context(), audit.RecordRequest{
		SellerID:   sellerID,
		ActorType:  audit.ActorTypeSellerUser,
		ActorID:    "local-seller",
		Action:     audit.ActionCredentialRevoked,
		TargetType: audit.TargetTypeIntegrationCredential,
		TargetID:   "key_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		Outcome:    audit.OutcomeSucceeded,
		RequestID:  "request-seed",
		ChangedFields: []string{
			"revokedAt",
		},
	}); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	audit.NewHTTPController(service).RegisterRoutes(mux)
	handler := api.Middleware(
		api.Config{
			Authenticator: api.NewStaticAuthenticator(
				"seller-secret",
				"agent-secret",
			),
		},
		mux,
	)
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/sellers/"+sellerID.String()+"/audit-events?limit=1",
		nil,
	)
	request.Header.Set("Authorization", "Bearer seller-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	var page audit.EventPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].RequestID != "request-seed" {
		t.Fatalf("page = %#v", page)
	}
}

// mustControllerAuditID parses one canonical API test identifier.
func mustControllerAuditID(
	t *testing.T,
	raw string,
	prefix domain.IDPrefix,
) domain.ID {
	t.Helper()

	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}

type controllerAuditAuthorizer struct {
	sellerID domain.ID
}

// AuthorizeSeller permits the local seller fixture only.
func (authorizer controllerAuditAuthorizer) AuthorizeSeller(
	_ context.Context,
	ownerSubject string,
	sellerID domain.ID,
) error {
	if ownerSubject != "local-seller" || sellerID != authorizer.sellerID {
		return persistence.ErrNotFound
	}
	return nil
}
