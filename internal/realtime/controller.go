package realtime

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"

	"github.com/coder/websocket"
	"github.com/fourgeez/agentpay/internal/domain"
)

const websocketConnectionIDBytes = 16

// Controller handles WebSocket lifecycle commands at the transport boundary.
type Controller struct {
	service *Service
}

// NewController creates the WebSocket lifecycle controller.
func NewController(service *Service) *Controller {
	return &Controller{service: service}
}

// Connect validates transport values and registers an authenticated connection.
func (controller *Controller) Connect(
	ctx context.Context,
	request ConnectRequest,
) error {
	sessionID, err := domain.ParseID(
		request.SessionID,
		domain.ApprovalIDPrefix,
	)
	if err != nil {
		return err
	}
	return controller.service.Register(
		ctx,
		request.ConnectionID,
		sessionID,
		request.Token,
	)
}

// Disconnect removes a connection registration after transport closure.
func (controller *Controller) Disconnect(
	ctx context.Context,
	request DisconnectRequest,
) error {
	return controller.service.Unregister(ctx, request.ConnectionID)
}

// HTTPController serves the local WebSocket transport described by AsyncAPI.
type HTTPController struct {
	lifecycle     *Controller
	hub           *LocalHub
	allowedOrigin string
}

// NewHTTPController creates the local WebSocket transport controller.
func NewHTTPController(
	lifecycle *Controller,
	hub *LocalHub,
	allowedOrigin ...string,
) *HTTPController {
	origin := ""
	if len(allowedOrigin) > 0 {
		origin = normalizeOriginPattern(allowedOrigin[0])
	}
	return &HTTPController{
		lifecycle:     lifecycle,
		hub:           hub,
		allowedOrigin: origin,
	}
}

// normalizeOriginPattern converts a configured web URL into a host pattern.
func normalizeOriginPattern(rawOrigin string) string {
	parsedOrigin, err := url.Parse(rawOrigin)
	if err == nil && parsedOrigin.Host != "" {
		return parsedOrigin.Host
	}
	return rawOrigin
}

// RegisterRoutes registers the local AsyncAPI WebSocket endpoint.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /ws/approval-sessions/{sessionId}",
		controller.connect,
	)
}

// connect upgrades, authenticates, and keeps one WebSocket registration alive.
func (controller *HTTPController) connect(
	response http.ResponseWriter,
	request *http.Request,
) {
	options := &websocket.AcceptOptions{}
	if controller.allowedOrigin != "" {
		options.OriginPatterns = []string{controller.allowedOrigin}
	}
	connection, err := websocket.Accept(response, request, options)
	if err != nil {
		return
	}
	connectionID, err := newWebSocketConnectionID()
	if err != nil {
		_ = connection.Close(
			websocket.StatusInternalError,
			"connection initialization failed",
		)
		return
	}

	controller.hub.Attach(connectionID, connection)
	defer func() {
		_ = controller.lifecycle.Disconnect(
			context.Background(),
			DisconnectRequest{ConnectionID: connectionID},
		)
		_ = connection.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	if err := controller.lifecycle.Connect(
		request.Context(),
		ConnectRequest{
			ConnectionID: connectionID,
			SessionID:    request.PathValue("sessionId"),
			Token:        request.URL.Query().Get("token"),
		},
	); err != nil {
		_ = connection.Close(
			websocket.StatusPolicyViolation,
			"approval invitation rejected",
		)
		return
	}

	for {
		if _, _, err := connection.Read(request.Context()); err != nil {
			return
		}
	}
}

// newWebSocketConnectionID creates an opaque local connection identifier.
func newWebSocketConnectionID() (string, error) {
	randomBytes := make([]byte, websocketConnectionIDBytes)
	if _, err := cryptorand.Read(randomBytes); err != nil {
		return "", err
	}
	return "wsc_" + hex.EncodeToString(randomBytes), nil
}
