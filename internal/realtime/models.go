package realtime

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/domain"
)

// ErrConnectionGone reports a WebSocket connection that no longer exists.
var ErrConnectionGone = errors.New("websocket connection is gone")

// Connection is one session-scoped WebSocket registration.
type Connection struct {
	ConnectionID string           `json:"connectionId"`
	SessionID    domain.ID        `json:"sessionId"`
	ExpiresAt    domain.Timestamp `json:"expiresAt"`
	ConnectedAt  domain.Timestamp `json:"connectedAt"`
}

// ConnectRequest contains the authenticated WebSocket connect parameters.
type ConnectRequest struct {
	ConnectionID string
	SessionID    string
	Token        string
}

// DisconnectRequest identifies the connection being removed.
type DisconnectRequest struct {
	ConnectionID string
}

// EventEnvelope is the AsyncAPI approval event representation.
type EventEnvelope struct {
	EventID    domain.ID                 `json:"eventId"`
	Type       approvals.EventType       `json:"type"`
	SessionID  domain.ID                 `json:"sessionId"`
	OccurredAt domain.Timestamp          `json:"occurredAt"`
	Data       approvals.SessionResponse `json:"data"`
}

// Repository persists ephemeral WebSocket connection registrations.
type Repository interface {
	Register(ctx context.Context, connection Connection) error
	Delete(ctx context.Context, connectionID string) error
	ListBySession(ctx context.Context, sessionID domain.ID) ([]Connection, error)
}

// ApprovalSessionReader authorizes invitations and reads current snapshots.
type ApprovalSessionReader interface {
	AuthorizeInvitation(
		ctx context.Context,
		sessionID domain.ID,
		rawInvitationToken string,
	) error
	Get(
		ctx context.Context,
		sessionID domain.ID,
	) (approvals.SessionResponse, error)
}

// Client sends one event to a concrete WebSocket connection.
type Client interface {
	Send(
		ctx context.Context,
		connectionID string,
		event EventEnvelope,
	) error
}
