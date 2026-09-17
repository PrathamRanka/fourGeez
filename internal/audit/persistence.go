package audit

import (
	"encoding/base64"
	"encoding/json"

	"github.com/fourgeez/agentpay/internal/domain"
)

// EventSnapshot is the complete immutable persistence representation.
type EventSnapshot struct {
	AuditEventID  domain.ID        `json:"auditEventId"`
	SellerID      domain.ID        `json:"sellerId"`
	ActorType     ActorType        `json:"actorType"`
	ActorID       string           `json:"actorId"`
	Action        Action           `json:"action"`
	TargetType    TargetType       `json:"targetType"`
	TargetID      string           `json:"targetId"`
	Outcome       Outcome          `json:"outcome"`
	RequestID     string           `json:"requestId"`
	ChangedFields []string         `json:"changedFields"`
	OccurredAt    domain.Timestamp `json:"occurredAt"`
}

// PageCursor identifies the last seller event returned by a page.
type PageCursor struct {
	SellerID     domain.ID        `json:"sellerId"`
	AuditEventID domain.ID        `json:"auditEventId"`
	OccurredAt   domain.Timestamp `json:"occurredAt"`
}

// Snapshot returns an isolated complete event representation.
func (event Event) Snapshot() EventSnapshot {
	return EventSnapshot{
		AuditEventID:  event.auditEventID,
		SellerID:      event.sellerID,
		ActorType:     event.actorType,
		ActorID:       event.actorID,
		Action:        event.action,
		TargetType:    event.targetType,
		TargetID:      event.targetID,
		Outcome:       event.outcome,
		RequestID:     event.requestID,
		ChangedFields: event.ChangedFields(),
		OccurredAt:    event.occurredAt,
	}
}

// RestoreEvent validates one persisted immutable audit event.
func RestoreEvent(snapshot EventSnapshot) (Event, error) {
	return NewEvent(EventParams(snapshot))
}

// EncodeCursor creates an opaque seller-bound page cursor.
func EncodeCursor(event Event) (string, error) {
	encoded, err := json.Marshal(PageCursor{
		SellerID:     event.SellerID(),
		AuditEventID: event.AuditEventID(),
		OccurredAt:   event.OccurredAt(),
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

// DecodeCursor validates an opaque cursor and its seller binding.
func DecodeCursor(raw string, sellerID domain.ID) (PageCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return PageCursor{}, domain.NewValidationError("cursor", "format", "must be a valid audit cursor")
	}
	var cursor PageCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return PageCursor{}, domain.NewValidationError("cursor", "format", "must be a valid audit cursor")
	}
	if cursor.SellerID != sellerID {
		return PageCursor{}, domain.NewValidationError("cursor", "scope", "does not belong to this seller")
	}
	if cursor.AuditEventID.Prefix() != domain.AuditEventIDPrefix ||
		cursor.OccurredAt.Time().IsZero() {
		return PageCursor{}, domain.NewValidationError("cursor", "format", "must identify an audit event")
	}
	return cursor, nil
}

// eventView removes persistence concerns from the public response.
func eventView(event Event) EventView {
	snapshot := event.Snapshot()
	return EventView(snapshot)
}
