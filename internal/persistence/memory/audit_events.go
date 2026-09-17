package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// AuditEventRepository stores append-only seller audit events for local use.
type AuditEventRepository struct {
	mutex  sync.RWMutex
	events map[domain.ID]audit.EventSnapshot
}

// NewAuditEventRepository creates an empty audit repository.
func NewAuditEventRepository() *AuditEventRepository {
	return &AuditEventRepository{
		events: make(map[domain.ID]audit.EventSnapshot),
	}
}

// Create appends one immutable event when its identifier is unused.
func (repository *AuditEventRepository) Create(
	_ context.Context,
	event audit.Event,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.events[event.AuditEventID()]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.events[event.AuditEventID()] = event.Snapshot()
	return nil
}

// ListBySeller returns one seller-bound newest-first page.
func (repository *AuditEventRepository) ListBySeller(
	_ context.Context,
	sellerID domain.ID,
	limit int,
	cursor string,
) ([]audit.Event, *string, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	events := make([]audit.Event, 0)
	for _, snapshot := range repository.events {
		if snapshot.SellerID != sellerID {
			continue
		}
		event, err := audit.RestoreEvent(snapshot)
		if err != nil {
			return nil, nil, err
		}
		events = append(events, event)
	}
	sort.Slice(events, func(leftIndex int, rightIndex int) bool {
		left := events[leftIndex]
		right := events[rightIndex]
		if left.OccurredAt().String() == right.OccurredAt().String() {
			return left.AuditEventID().String() > right.AuditEventID().String()
		}
		return left.OccurredAt().Time().After(right.OccurredAt().Time())
	})
	start, err := auditPageStart(events, sellerID, cursor)
	if err != nil {
		return nil, nil, err
	}
	end := start + limit
	if end > len(events) {
		end = len(events)
	}
	page := append([]audit.Event(nil), events[start:end]...)
	var nextCursor *string
	if end < len(events) && len(page) > 0 {
		encoded, err := audit.EncodeCursor(page[len(page)-1])
		if err != nil {
			return nil, nil, err
		}
		nextCursor = &encoded
	}
	return page, nextCursor, nil
}

// auditPageStart resolves one cursor only inside its seller page.
func auditPageStart(
	events []audit.Event,
	sellerID domain.ID,
	rawCursor string,
) (int, error) {
	if rawCursor == "" {
		return 0, nil
	}
	cursor, err := audit.DecodeCursor(rawCursor, sellerID)
	if err != nil {
		return 0, err
	}
	for index, event := range events {
		if event.AuditEventID() == cursor.AuditEventID &&
			event.OccurredAt().String() == cursor.OccurredAt.String() {
			return index + 1, nil
		}
	}
	return 0, domain.NewValidationError(
		"cursor",
		"scope",
		"does not identify an event in this seller history",
	)
}
