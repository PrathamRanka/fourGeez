package memory

import (
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestAuditEventRepositoryPaginatesAndBindsCursorsToSeller verifies local parity.
func TestAuditEventRepositoryPaginatesAndBindsCursorsToSeller(t *testing.T) {
	t.Parallel()

	repository := NewAuditEventRepository()
	sellerID := mustMemoryAuditID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.SellerIDPrefix,
	)
	otherSellerID := mustMemoryAuditID(
		t,
		"sel_01K5D09YJ0C0M7RJM4FWQ0K9H9",
		domain.SellerIDPrefix,
	)
	older := memoryAuditEvent(
		t,
		sellerID,
		"aud_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	)
	newer := memoryAuditEvent(
		t,
		sellerID,
		"aud_01K5D09YJ0C0M7RJM4FWQ0K9HA",
		time.Date(2026, time.September, 18, 11, 0, 0, 0, time.UTC),
	)
	other := memoryAuditEvent(
		t,
		otherSellerID,
		"aud_01K5D09YJ0C0M7RJM4FWQ0K9HB",
		time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC),
	)
	for _, event := range []audit.Event{older, newer, other} {
		if err := repository.Create(t.Context(), event); err != nil {
			t.Fatal(err)
		}
	}

	firstPage, nextCursor, err := repository.ListBySeller(
		t.Context(),
		sellerID,
		1,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage) != 1 || firstPage[0].AuditEventID() != newer.AuditEventID() || nextCursor == nil {
		t.Fatalf("first page = %#v, cursor = %#v", firstPage, nextCursor)
	}
	secondPage, finalCursor, err := repository.ListBySeller(
		t.Context(),
		sellerID,
		1,
		*nextCursor,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage) != 1 || secondPage[0].AuditEventID() != older.AuditEventID() || finalCursor != nil {
		t.Fatalf("second page = %#v, cursor = %#v", secondPage, finalCursor)
	}
	if _, _, err := repository.ListBySeller(
		t.Context(),
		otherSellerID,
		1,
		*nextCursor,
	); err == nil {
		t.Fatal("cross-seller cursor was accepted")
	}
}

// memoryAuditEvent creates one immutable audit fixture.
func memoryAuditEvent(
	t *testing.T,
	sellerID domain.ID,
	auditEventID string,
	occurredAt time.Time,
) audit.Event {
	t.Helper()

	event, err := audit.NewEvent(audit.EventParams{
		AuditEventID: mustMemoryAuditID(t, auditEventID, domain.AuditEventIDPrefix),
		SellerID:     sellerID,
		ActorType:    audit.ActorTypeSystem,
		ActorID:      "agentpay",
		Action:       audit.ActionSellerSuspended,
		TargetType:   audit.TargetTypeSeller,
		TargetID:     sellerID.String(),
		Outcome:      audit.OutcomeSucceeded,
		RequestID:    "request-123",
		ChangedFields: []string{
			"status",
		},
		OccurredAt: domain.NewTimestamp(occurredAt),
	})
	if err != nil {
		t.Fatal(err)
	}
	return event
}

// mustMemoryAuditID parses one canonical memory repository identifier.
func mustMemoryAuditID(
	t *testing.T,
	raw string,
	prefix domain.IDPrefix,
) domain.ID {
	t.Helper()

	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}
