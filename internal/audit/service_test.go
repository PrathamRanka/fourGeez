package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// TestNewEventEnforcesVocabularyAndChangedFieldAllowlists verifies safe audit metadata.
func TestNewEventEnforcesVocabularyAndChangedFieldAllowlists(t *testing.T) {
	t.Parallel()

	validParameters := EventParams{
		AuditEventID: mustAuditID(t, "aud_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.AuditEventIDPrefix),
		SellerID:     mustAuditID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.SellerIDPrefix),
		ActorType:    ActorTypeSellerUser,
		ActorID:      "seller-subject",
		Action:       ActionRoutePriceChanged,
		TargetType:   TargetTypePaidRoute,
		TargetID:     "rte_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		Outcome:      OutcomeSucceeded,
		RequestID:    "request-123",
		ChangedFields: []string{
			"amount",
		},
		OccurredAt: domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)),
	}

	tests := []struct {
		name   string
		mutate func(*EventParams)
		field  string
	}{
		{
			name: "action target mismatch",
			mutate: func(parameters *EventParams) {
				parameters.TargetType = TargetTypeSeller
				parameters.TargetID = parameters.SellerID.String()
			},
			field: "targetType",
		},
		{
			name: "secret field",
			mutate: func(parameters *EventParams) {
				parameters.ChangedFields = []string{"tokenHash"}
			},
			field: "changedFields",
		},
		{
			name: "duplicate field",
			mutate: func(parameters *EventParams) {
				parameters.ChangedFields = []string{"amount", "amount"}
			},
			field: "changedFields",
		},
	}

	if _, err := NewEvent(validParameters); err != nil {
		t.Fatalf("NewEvent(valid) error = %v", err)
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			parameters := validParameters
			parameters.ChangedFields = append([]string(nil), validParameters.ChangedFields...)
			testCase.mutate(&parameters)
			_, err := NewEvent(parameters)
			assertAuditValidationField(t, err, testCase.field)
		})
	}
}

func TestNewEventAcceptsSubscriptionAndCredentialSecurityActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		action        Action
		targetType    TargetType
		targetID      string
		changedFields []string
	}{
		{action: ActionCredentialRotated, targetType: TargetTypeIntegrationCredential, targetID: "key_01K5D09YJ0C0M7RJM4FWQ0K9H8", changedFields: []string{"revokedAt", "replacedByCredentialId"}},
		{action: ActionCredentialExchangeSucceeded, targetType: TargetTypeIntegrationCredential, targetID: "key_01K5D09YJ0C0M7RJM4FWQ0K9H8", changedFields: []string{"lastUsedAt"}},
		{action: ActionCredentialExchangeDenied, targetType: TargetTypeIntegrationCredential, targetID: "key_01K5D09YJ0C0M7RJM4FWQ0K9H8", changedFields: []string{"authorization"}},
		{action: ActionEntitlementChanged, targetType: TargetTypeSeller, targetID: "sel_01K5D09YJ0C0M7RJM4FWQ0K9H8", changedFields: []string{"status", "accessEndsAt", "entitlementEpoch", "sourceRevision", "credentialRotationRequired"}},
		{action: ActionIntegrationVerificationDone, targetType: TargetTypePaidRoute, targetID: "rte_01K5D09YJ0C0M7RJM4FWQ0K9H8", changedFields: []string{"valid", "routeVersion", "checks"}},
		{action: ActionManualRefundRecorded, targetType: TargetTypeDispute, targetID: "dsp_01K5D09YJ0C0M7RJM4FWQ0K9H8", changedFields: []string{"amount", "asset", "network", "reference"}},
	}
	for _, test := range tests {
		_, err := NewEvent(EventParams{
			AuditEventID: mustAuditID(t, "aud_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.AuditEventIDPrefix),
			SellerID:     mustAuditID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.SellerIDPrefix),
			ActorType:    ActorTypeSystem, ActorID: "authorization", Action: test.action,
			TargetType: test.targetType, TargetID: test.targetID, Outcome: OutcomeSucceeded,
			RequestID: "request-789", ChangedFields: test.changedFields,
			OccurredAt: domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)),
		})
		if err != nil {
			t.Fatalf("NewEvent(%s) error = %v", test.action, err)
		}
	}
}

// TestServiceRecordsAndListsAuthorizedHistory verifies append and tenant checks.
func TestServiceRecordsAndListsAuthorizedHistory(t *testing.T) {
	t.Parallel()

	repository := &auditTestRepository{}
	service := NewService(
		repository,
		auditTestAuthorizer{},
		auditTestIDGenerator{},
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 11, 0, 0, 0, time.UTC)},
	)
	sellerID := mustAuditID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.SellerIDPrefix)

	err := service.Record(t.Context(), RecordRequest{
		SellerID:   sellerID,
		ActorType:  ActorTypeIntegrationCredential,
		ActorID:    "key_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		Action:     ActionRoutePublished,
		TargetType: TargetTypePaidRoute,
		TargetID:   "rte_01K5D09YJ0C0M7RJM4FWQ0K9H8",
		Outcome:    OutcomeSucceeded,
		RequestID:  "request-456",
		ChangedFields: []string{
			"enabled",
		},
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	page, err := service.List(t.Context(), "seller-subject", sellerID, 25, "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Action != ActionRoutePublished {
		t.Fatalf("List() = %#v", page)
	}

	_, err = service.List(t.Context(), "another-subject", sellerID, 25, "")
	if !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("List(unauthorized) error = %v", err)
	}
}

// mustAuditID parses one canonical test identifier.
func mustAuditID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()

	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}

// assertAuditValidationField verifies one expected validation field.
func assertAuditValidationField(t *testing.T, err error, field string) {
	t.Helper()

	var validationErrors domain.ValidationErrors
	if !errors.As(err, &validationErrors) {
		t.Fatalf("error = %v, want validation errors", err)
	}
	for _, validationError := range validationErrors {
		if validationError.Field == field {
			return
		}
	}
	t.Fatalf("validation errors = %v, want field %q", validationErrors, field)
}

type auditTestRepository struct {
	events []Event
}

// Create appends one test audit event.
func (repository *auditTestRepository) Create(_ context.Context, event Event) error {
	repository.events = append(repository.events, event)
	return nil
}

// ListBySeller returns the events captured by the test repository.
func (repository *auditTestRepository) ListBySeller(
	_ context.Context,
	_ domain.ID,
	_ int,
	_ string,
) ([]Event, *string, error) {
	return append([]Event(nil), repository.events...), nil, nil
}

type auditTestAuthorizer struct{}

// AuthorizeSeller permits the expected subject and hides all other callers.
func (auditTestAuthorizer) AuthorizeSeller(
	_ context.Context,
	ownerSubject string,
	_ domain.ID,
) error {
	if ownerSubject != "seller-subject" {
		return persistence.ErrNotFound
	}
	return nil
}

type auditTestIDGenerator struct{}

// New returns one deterministic audit identifier.
func (auditTestIDGenerator) New(prefix domain.IDPrefix) (domain.ID, error) {
	if prefix != domain.AuditEventIDPrefix {
		return "", errors.New("unexpected prefix")
	}
	return domain.ID("aud_01K5D09YJ0C0M7RJM4FWQ0K9H8"), nil
}
