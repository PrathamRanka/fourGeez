package integrations

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestServiceAuditsCredentialCreationAndRevocation verifies credential lifecycle events.
func TestServiceAuditsCredentialCreationAndRevocation(t *testing.T) {
	t.Parallel()

	recorder := &integrationAuditRecorder{}
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	}
	service := NewService(
		newCredentialRepository(),
		&sellerAuthorizer{},
		&credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)},
		clock,
		recorder,
	)

	created, err := service.Create(
		t.Context(),
		"owner-123",
		domain.ID(testSellerID),
		CreateCredentialRequest{
			Label:  "Codex workstation",
			Scopes: []Scope{ScopeRead, ScopeConfigure},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Revoke(
		t.Context(),
		"owner-123",
		domain.ID(testSellerID),
		created.CredentialID,
		created.Version,
	); err != nil {
		t.Fatal(err)
	}

	if len(recorder.requests) != 2 ||
		recorder.requests[0].Action != audit.ActionCredentialCreated ||
		recorder.requests[1].Action != audit.ActionCredentialRevoked {
		t.Fatalf("audit requests = %#v", recorder.requests)
	}
}

type integrationAuditRecorder struct {
	requests []audit.RecordRequest
}

// Record captures one credential audit request.
func (recorder *integrationAuditRecorder) Record(
	_ context.Context,
	request audit.RecordRequest,
) error {
	recorder.requests = append(recorder.requests, request)
	return nil
}
