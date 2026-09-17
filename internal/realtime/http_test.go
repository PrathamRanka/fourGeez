package realtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestHTTPControllerUpgradesAndSendsInitialSnapshot verifies the local route.
func TestHTTPControllerUpgradesAndSendsInitialSnapshot(t *testing.T) {
	t.Parallel()

	fixture := newRealtimeFixture(t)
	hub := NewLocalHub()
	service := NewService(
		hub,
		fixture.reader,
		hub,
		domain.NewULIDGenerator(
			fixture.clock,
			strings.NewReader(strings.Repeat("f", 512)),
		),
		fixture.clock,
	)
	controller := NewHTTPController(NewController(service), hub)
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/ws/approval-sessions/" + fixture.snapshot.SessionID.String() +
		"?token=invite-secret"
	connection, _, err := websocket.Dial(t.Context(), websocketURL, nil)
	if err != nil {
		t.Fatalf("websocket dial error = %v", err)
	}
	defer connection.Close(websocket.StatusNormalClosure, "test complete")

	var event EventEnvelope
	if err := wsjson.Read(context.Background(), connection, &event); err != nil {
		t.Fatalf("websocket read error = %v", err)
	}
	if event.Type != approvals.EventTypeSessionSnapshot ||
		event.SessionID != fixture.snapshot.SessionID {
		t.Fatalf("initial event = %#v", event)
	}
}
