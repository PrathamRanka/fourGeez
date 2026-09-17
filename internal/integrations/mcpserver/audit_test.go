package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

// TestMutationServiceAuditsCredentialBoundRouteChanges verifies MCP actor identity.
func TestMutationServiceAuditsCredentialBoundRouteChanges(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	}
	recorder := &mcpAuditRecorder{}
	service := NewMutationService(
		&testCatalogMutator{},
		memory.NewIdempotencyStore(),
		clock,
		&testSandboxValidator{valid: true},
		recorder,
	)
	principal := integrations.Principal{
		SellerID:     domain.ID(testSellerID),
		CredentialID: domain.ID(testCredentialID),
		Scopes:       []integrations.Scope{integrations.ScopeConfigure},
	}

	input := ConfigureRouteInput{
		IdempotencyKey: "audit-route-create-001",
		Confirmation:   validConfirmation(clock.Now()),
		Route: RouteConfiguration{
			Method:                 catalog.RouteMethodPost,
			PathPattern:            "/research",
			Description:            "Research",
			MIMEType:               "application/json",
			Amount:                 "100",
			Asset:                  "USDC",
			Network:                "eip155:84532",
			PayTo:                  "0x123",
			UpstreamTimeoutSeconds: 20,
		},
	}
	_, err := service.ConfigureRoute(
		t.Context(),
		principal,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfigureRoute(t.Context(), principal, input); err != nil {
		t.Fatal(err)
	}
	if len(recorder.requests) != 1 ||
		recorder.requests[0].Action != audit.ActionRouteDraftCreated ||
		recorder.requests[0].ActorType != audit.ActorTypeIntegrationCredential ||
		recorder.requests[0].ActorID != testCredentialID {
		t.Fatalf("audit requests = %#v", recorder.requests)
	}
}

type mcpAuditRecorder struct {
	requests []audit.RecordRequest
}

// Record captures one MCP mutation audit request.
func (recorder *mcpAuditRecorder) Record(
	_ context.Context,
	request audit.RecordRequest,
) error {
	recorder.requests = append(recorder.requests, request)
	return nil
}
