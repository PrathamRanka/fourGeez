package realtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/fourgeez/agentpay/internal/domain"
)

const websocketWriteTimeout = 2 * time.Second

// LocalHub stores local connection registrations and their active sockets.
type LocalHub struct {
	mutex       sync.RWMutex
	connections map[string]Connection
	sockets     map[string]*websocket.Conn
}

// NewLocalHub creates an empty local WebSocket hub.
func NewLocalHub() *LocalHub {
	return &LocalHub{
		connections: make(map[string]Connection),
		sockets:     make(map[string]*websocket.Conn),
	}
}

// Attach associates a newly upgraded socket with its generated connection ID.
func (hub *LocalHub) Attach(
	connectionID string,
	connection *websocket.Conn,
) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	hub.sockets[connectionID] = connection
}

// Register stores the authorized session registration for an attached socket.
func (hub *LocalHub) Register(
	_ context.Context,
	connection Connection,
) error {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	if _, exists := hub.sockets[connection.ConnectionID]; !exists {
		return ErrConnectionGone
	}
	hub.connections[connection.ConnectionID] = connection
	return nil
}

// Delete removes connection metadata and its socket reference.
func (hub *LocalHub) Delete(
	_ context.Context,
	connectionID string,
) error {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	delete(hub.connections, connectionID)
	delete(hub.sockets, connectionID)
	return nil
}

// ListBySession returns isolated registrations for one approval session.
func (hub *LocalHub) ListBySession(
	_ context.Context,
	sessionID domain.ID,
) ([]Connection, error) {
	hub.mutex.RLock()
	defer hub.mutex.RUnlock()
	connections := make([]Connection, 0)
	for _, connection := range hub.connections {
		if connection.SessionID == sessionID {
			connections = append(connections, connection)
		}
	}
	return connections, nil
}

// Send writes one JSON event to an active local WebSocket connection.
func (hub *LocalHub) Send(
	ctx context.Context,
	connectionID string,
	event EventEnvelope,
) error {
	hub.mutex.RLock()
	connection, exists := hub.sockets[connectionID]
	hub.mutex.RUnlock()
	if !exists {
		return ErrConnectionGone
	}

	writeContext, cancel := context.WithTimeout(ctx, websocketWriteTimeout)
	defer cancel()
	if err := wsjson.Write(writeContext, connection, event); err != nil {
		if websocket.CloseStatus(err) != -1 || errors.Is(err, context.Canceled) {
			return ErrConnectionGone
		}
		return err
	}
	return nil
}
