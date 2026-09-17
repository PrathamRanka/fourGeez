package realtime

import (
	"context"
	"errors"
	"strings"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/domain"
)

const maximumConnectionIDLength = 256

// Service coordinates connection registration and approval event delivery.
type Service struct {
	repository    Repository
	sessionReader ApprovalSessionReader
	client        Client
	idGenerator   domain.IDGenerator
	clock         domain.Clock
}

// NewService creates the realtime approval service.
func NewService(
	repository Repository,
	sessionReader ApprovalSessionReader,
	client Client,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
) *Service {
	return &Service{
		repository:    repository,
		sessionReader: sessionReader,
		client:        client,
		idGenerator:   idGenerator,
		clock:         clock,
	}
}

// Register authorizes and stores a connection before sending its current snapshot.
func (service *Service) Register(
	ctx context.Context,
	connectionID string,
	sessionID domain.ID,
	rawInvitationToken string,
) error {
	trimmedConnectionID := strings.TrimSpace(connectionID)
	if trimmedConnectionID == "" ||
		len(trimmedConnectionID) > maximumConnectionIDLength {
		return domain.NewValidationError(
			"connectionId",
			"length",
			"must contain 1-256 characters",
		)
	}
	if err := service.sessionReader.AuthorizeInvitation(
		ctx,
		sessionID,
		rawInvitationToken,
	); err != nil {
		return err
	}
	snapshot, err := service.sessionReader.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	now := domain.NewTimestamp(service.clock.Now())
	if !now.Time().Before(snapshot.ExpiresAt.Time()) {
		return approvals.ErrSessionExpired
	}

	connection := Connection{
		ConnectionID: trimmedConnectionID,
		SessionID:    sessionID,
		ExpiresAt:    snapshot.ExpiresAt,
		ConnectedAt:  now,
	}
	if err := service.repository.Register(ctx, connection); err != nil {
		return err
	}
	event, err := service.newEvent(
		approvals.EventTypeSessionSnapshot,
		snapshot,
	)
	if err != nil {
		_ = service.repository.Delete(ctx, trimmedConnectionID)
		return err
	}
	if err := service.client.Send(ctx, trimmedConnectionID, event); err != nil {
		_ = service.repository.Delete(ctx, trimmedConnectionID)
		return err
	}
	return nil
}

// Unregister removes a disconnected WebSocket registration.
func (service *Service) Unregister(
	ctx context.Context,
	connectionID string,
) error {
	trimmedConnectionID := strings.TrimSpace(connectionID)
	if trimmedConnectionID == "" {
		return domain.NewValidationError(
			"connectionId",
			"required",
			"is required",
		)
	}
	return service.repository.Delete(ctx, trimmedConnectionID)
}

// PublishApprovalEvent broadcasts a redacted snapshot to active connections.
func (service *Service) PublishApprovalEvent(
	ctx context.Context,
	eventType approvals.EventType,
	snapshot approvals.SessionResponse,
) {
	connections, err := service.repository.ListBySession(
		ctx,
		snapshot.SessionID,
	)
	if err != nil {
		return
	}
	event, err := service.newEvent(eventType, snapshot)
	if err != nil {
		return
	}
	now := service.clock.Now()
	for _, connection := range connections {
		if !now.Before(connection.ExpiresAt.Time()) {
			_ = service.repository.Delete(ctx, connection.ConnectionID)
			continue
		}
		if err := service.client.Send(ctx, connection.ConnectionID, event); err != nil &&
			errors.Is(err, ErrConnectionGone) {
			_ = service.repository.Delete(ctx, connection.ConnectionID)
		}
	}
}

// newEvent creates one redacted AsyncAPI event envelope.
func (service *Service) newEvent(
	eventType approvals.EventType,
	snapshot approvals.SessionResponse,
) (EventEnvelope, error) {
	if !supportedEventType(eventType) {
		return EventEnvelope{}, domain.NewValidationError(
			"eventType",
			"supported",
			"is not an approval event type",
		)
	}
	eventID, err := service.idGenerator.New(domain.EvidenceIDPrefix)
	if err != nil {
		return EventEnvelope{}, err
	}
	snapshot.Invitations = nil
	snapshot.ApprovalToken = ""
	return EventEnvelope{
		EventID:    eventID,
		Type:       eventType,
		SessionID:  snapshot.SessionID,
		OccurredAt: domain.NewTimestamp(service.clock.Now()),
		Data:       snapshot,
	}, nil
}

// supportedEventType validates the AsyncAPI event vocabulary.
func supportedEventType(eventType approvals.EventType) bool {
	switch eventType {
	case approvals.EventTypeSessionSnapshot,
		approvals.EventTypeApproverJoined,
		approvals.EventTypeApprovalDecided,
		approvals.EventTypeSessionResolved:
		return true
	default:
		return false
	}
}
