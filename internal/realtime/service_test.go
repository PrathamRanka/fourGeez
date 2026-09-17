package realtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

// TestControllerReconnectSendsCurrentRedactedSnapshot verifies reconnect recovery.
func TestControllerReconnectSendsCurrentRedactedSnapshot(t *testing.T) {
	t.Parallel()

	fixture := newRealtimeFixture(t)
	if err := fixture.controller.Connect(
		t.Context(),
		ConnectRequest{
			ConnectionID: "connection-one",
			SessionID:    fixture.snapshot.SessionID.String(),
			Token:        "invite-secret",
		},
	); err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}

	fixture.snapshot.Decisions = []approvals.DecisionView{
		{
			Label:     "Finance",
			Decision:  approvals.DecisionApprove,
			DecidedAt: domain.NewTimestamp(fixture.clock.Now()),
		},
	}
	fixture.reader.snapshot = fixture.snapshot
	if err := fixture.controller.Connect(
		t.Context(),
		ConnectRequest{
			ConnectionID: "connection-two",
			SessionID:    fixture.snapshot.SessionID.String(),
			Token:        "invite-secret",
		},
	); err != nil {
		t.Fatalf("reconnect Connect() error = %v", err)
	}

	latest := fixture.client.events["connection-two"]
	if len(latest) != 1 || latest[0].Type != approvals.EventTypeSessionSnapshot {
		t.Fatalf("reconnect events = %#v", latest)
	}
	if len(latest[0].Data.Decisions) != 1 {
		t.Fatal("reconnect snapshot did not contain the latest decision")
	}
	if len(latest[0].Data.Invitations) != 0 || latest[0].Data.ApprovalToken != "" {
		t.Fatal("realtime snapshot exposed one-time credentials")
	}
}

// TestServicePublishesApprovalEventsAndPrunesGoneConnections verifies fan-out.
func TestServicePublishesApprovalEventsAndPrunesGoneConnections(t *testing.T) {
	t.Parallel()

	fixture := newRealtimeFixture(t)
	for _, connectionID := range []string{"connection-one", "connection-two"} {
		if err := fixture.controller.Connect(
			t.Context(),
			ConnectRequest{
				ConnectionID: connectionID,
				SessionID:    fixture.snapshot.SessionID.String(),
				Token:        "invite-secret",
			},
		); err != nil {
			t.Fatal(err)
		}
	}
	fixture.client.goneConnectionID = "connection-one"

	fixture.service.PublishApprovalEvent(
		t.Context(),
		approvals.EventTypeApprovalDecided,
		fixture.snapshot,
	)

	if _, exists := fixture.repository.connections["connection-one"]; exists {
		t.Fatal("gone connection was not removed")
	}
	events := fixture.client.events["connection-two"]
	if len(events) != 2 || events[1].Type != approvals.EventTypeApprovalDecided {
		t.Fatalf("published events = %#v", events)
	}
}

// TestControllerRejectsInvalidOrExpiredConnections verifies connection guards.
func TestControllerRejectsInvalidOrExpiredConnections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request ConnectRequest
		expire  bool
	}{
		{
			name: "invalid session ID",
			request: ConnectRequest{
				ConnectionID: "connection-one",
				SessionID:    "invalid",
				Token:        "invite-secret",
			},
		},
		{
			name: "invalid invitation",
			request: ConnectRequest{
				ConnectionID: "connection-one",
				SessionID:    "aps_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				Token:        "wrong-token",
			},
		},
		{
			name: "expired session",
			request: ConnectRequest{
				ConnectionID: "connection-one",
				SessionID:    "aps_01K5D09YJ0C0M7RJM4FWQ0K9H7",
				Token:        "invite-secret",
			},
			expire: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRealtimeFixture(t)
			if test.expire {
				fixture.clock.value = fixture.snapshot.ExpiresAt.Time()
			}
			if err := fixture.controller.Connect(t.Context(), test.request); err == nil {
				t.Fatal("Connect() accepted an invalid connection")
			}
		})
	}
}

// TestControllerDisconnectRemovesRegistration verifies disconnect cleanup.
func TestControllerDisconnectRemovesRegistration(t *testing.T) {
	t.Parallel()

	fixture := newRealtimeFixture(t)
	request := ConnectRequest{
		ConnectionID: "connection-one",
		SessionID:    fixture.snapshot.SessionID.String(),
		Token:        "invite-secret",
	}
	if err := fixture.controller.Connect(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if err := fixture.controller.Disconnect(
		t.Context(),
		DisconnectRequest{ConnectionID: request.ConnectionID},
	); err != nil {
		t.Fatal(err)
	}
	if len(fixture.repository.connections) != 0 {
		t.Fatal("Disconnect() left a registered connection")
	}
}

// realtimeFixture contains deterministic realtime service dependencies.
type realtimeFixture struct {
	controller *Controller
	service    *Service
	repository *realtimeTestRepository
	reader     *realtimeTestSessionReader
	client     *realtimeTestClient
	clock      *realtimeTestClock
	snapshot   approvals.SessionResponse
}

// newRealtimeFixture creates an API-006 test fixture.
func newRealtimeFixture(t *testing.T) realtimeFixture {
	t.Helper()
	clock := &realtimeTestClock{
		value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	snapshot := approvals.SessionResponse{
		SessionID:         mustRealtimeID(t, "aps_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.ApprovalIDPrefix),
		IntentID:          mustRealtimeID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		IntentHash:        mustRealtimeDigest(t, strings.Repeat("a", 64)),
		RequiredApprovals: 2,
		Decisions:         []approvals.DecisionView{},
		Status:            approvals.SessionStatusPending,
		ExpiresAt:         domain.NewTimestamp(clock.Now().Add(10 * time.Minute)),
		CreatedAt:         domain.NewTimestamp(clock.Now()),
		UpdatedAt:         domain.NewTimestamp(clock.Now()),
		Version:           1,
		Invitations: []approvals.InvitationLink{
			{Label: "Finance", URL: "https://example.test/secret"},
		},
		ApprovalToken: "approval-secret",
	}
	repository := &realtimeTestRepository{
		connections: make(map[string]Connection),
	}
	reader := &realtimeTestSessionReader{snapshot: snapshot}
	client := &realtimeTestClient{events: make(map[string][]EventEnvelope)}
	service := NewService(
		repository,
		reader,
		client,
		domain.NewULIDGenerator(clock, strings.NewReader(strings.Repeat("e", 512))),
		clock,
	)
	return realtimeFixture{
		controller: NewController(service),
		service:    service,
		repository: repository,
		reader:     reader,
		client:     client,
		clock:      clock,
		snapshot:   snapshot,
	}
}

// realtimeTestRepository stores connection registrations in memory.
type realtimeTestRepository struct {
	connections map[string]Connection
}

// Register stores or replaces one connection registration.
func (repository *realtimeTestRepository) Register(
	_ context.Context,
	connection Connection,
) error {
	repository.connections[connection.ConnectionID] = connection
	return nil
}

// Delete removes one connection registration.
func (repository *realtimeTestRepository) Delete(
	_ context.Context,
	connectionID string,
) error {
	delete(repository.connections, connectionID)
	return nil
}

// ListBySession returns active registrations for one approval session.
func (repository *realtimeTestRepository) ListBySession(
	_ context.Context,
	sessionID domain.ID,
) ([]Connection, error) {
	connections := make([]Connection, 0)
	for _, connection := range repository.connections {
		if connection.SessionID == sessionID {
			connections = append(connections, connection)
		}
	}
	return connections, nil
}

// realtimeTestSessionReader authorizes one token and returns a mutable snapshot.
type realtimeTestSessionReader struct {
	snapshot approvals.SessionResponse
}

// AuthorizeInvitation validates the deterministic invitation token.
func (reader *realtimeTestSessionReader) AuthorizeInvitation(
	_ context.Context,
	_ domain.ID,
	rawInvitationToken string,
) error {
	if rawInvitationToken != "invite-secret" {
		return approvals.ErrInvitationInvalid
	}
	return nil
}

// Get returns the current approval snapshot.
func (reader *realtimeTestSessionReader) Get(
	_ context.Context,
	_ domain.ID,
) (approvals.SessionResponse, error) {
	return reader.snapshot, nil
}

// realtimeTestClient records events and simulates gone connections.
type realtimeTestClient struct {
	events           map[string][]EventEnvelope
	goneConnectionID string
}

// Send records one event or reports a stale connection.
func (client *realtimeTestClient) Send(
	_ context.Context,
	connectionID string,
	event EventEnvelope,
) error {
	if connectionID == client.goneConnectionID {
		return ErrConnectionGone
	}
	client.events[connectionID] = append(client.events[connectionID], event)
	return nil
}

// realtimeTestClock allows tests to advance connection time.
type realtimeTestClock struct {
	value time.Time
}

// Now returns the current test-controlled time.
func (clock *realtimeTestClock) Now() time.Time {
	return clock.value
}

// mustRealtimeID parses a test domain identifier.
func mustRealtimeID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}

// mustRealtimeDigest parses a test intent digest.
func mustRealtimeDigest(t *testing.T, raw string) intents.SHA256Digest {
	t.Helper()
	digest, err := intents.ParseSHA256Digest(raw)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
