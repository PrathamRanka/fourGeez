package approvals

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

func TestNewSessionCreatesHashedSingleUseInvitations(t *testing.T) {
	t.Parallel()

	params := validSessionParams(t)
	tokenGenerator := &sequenceTokenGenerator{tokens: []string{"invite-alice-secret", "invite-bob-secret"}}
	session, grants, err := NewSession(params, tokenGenerator)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	if session.Status() != SessionStatusPending || session.RequiredApprovals() != 2 {
		t.Fatalf("session status/count = %q/%d", session.Status(), session.RequiredApprovals())
	}
	if len(grants) != 2 || grants[0].Token != "invite-alice-secret" || grants[1].Token != "invite-bob-secret" {
		t.Fatalf("grants = %#v", grants)
	}

	invitations := session.Invitations()
	if len(invitations) != 2 {
		t.Fatalf("stored invitations = %d, want 2", len(invitations))
	}
	for _, invitation := range invitations {
		if invitation.TokenHash == "" || invitation.TokenHash == grants[0].Token || invitation.TokenHash == grants[1].Token {
			t.Fatalf("raw invitation token appears stored: %#v", invitation)
		}
	}
}

func TestSessionApprovesAfterBothUniqueApprovers(t *testing.T) {
	t.Parallel()

	params := validSessionParams(t)
	tokenGenerator := &sequenceTokenGenerator{tokens: []string{"invite-alice-secret", "invite-bob-secret"}}
	session, _, err := NewSession(params, tokenGenerator)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	signer := mustApprovalSigner(t)

	firstResult, err := session.Decide("invite-alice-secret", DecisionApprove, params.CreatedAt.Add(time.Minute), signer)
	if err != nil {
		t.Fatalf("first Decide() error = %v", err)
	}
	if firstResult.Status != SessionStatusPending || firstResult.ApprovalToken != "" {
		t.Fatalf("first result = %#v", firstResult)
	}

	secondResult, err := session.Decide("invite-bob-secret", DecisionApprove, params.CreatedAt.Add(2*time.Minute), signer)
	if err != nil {
		t.Fatalf("second Decide() error = %v", err)
	}
	if secondResult.Status != SessionStatusApproved || secondResult.ApprovalToken == "" {
		t.Fatalf("second result = %#v", secondResult)
	}
	if session.ApprovalTokenHash() == "" || session.ApprovalTokenHash() == secondResult.ApprovalToken {
		t.Fatal("session did not store only the approval-token hash")
	}

	claims, err := signer.Verify(secondResult.ApprovalToken, params.SessionID, params.IntentID, params.IntentHash, params.CreatedAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.IntentHash != params.IntentHash || claims.SessionID != params.SessionID {
		t.Fatalf("claims = %#v", claims)
	}
}

func TestSessionVetoResolvesImmediately(t *testing.T) {
	t.Parallel()

	params := validSessionParams(t)
	session, _, err := NewSession(params, &sequenceTokenGenerator{tokens: []string{"invite-alice-secret", "invite-bob-secret"}})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	result, err := session.Decide("invite-alice-secret", DecisionVeto, params.CreatedAt.Add(time.Minute), mustApprovalSigner(t))
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if result.Status != SessionStatusVetoed || result.ApprovalToken != "" {
		t.Fatalf("result = %#v", result)
	}
	if _, err := session.Decide("invite-bob-secret", DecisionApprove, params.CreatedAt.Add(2*time.Minute), mustApprovalSigner(t)); !errors.Is(err, ErrSessionResolved) {
		t.Fatalf("second Decide() error = %v, want ErrSessionResolved", err)
	}
}

func TestSessionRejectsUsedOrUnknownInvitation(t *testing.T) {
	t.Parallel()

	params := validSessionParams(t)
	session, _, err := NewSession(params, &sequenceTokenGenerator{tokens: []string{"invite-alice-secret", "invite-bob-secret"}})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	signer := mustApprovalSigner(t)
	decisionTime := params.CreatedAt.Add(time.Minute)

	if _, err := session.Decide("unknown-token", DecisionApprove, decisionTime, signer); !errors.Is(err, ErrInvitationInvalid) {
		t.Fatalf("unknown token error = %v", err)
	}
	if _, err := session.Decide("invite-alice-secret", DecisionApprove, decisionTime, signer); err != nil {
		t.Fatalf("first Decide() error = %v", err)
	}
	if _, err := session.Decide("invite-alice-secret", DecisionApprove, decisionTime, signer); !errors.Is(err, ErrInvitationUsed) {
		t.Fatalf("reused token error = %v, want ErrInvitationUsed", err)
	}
}

func TestSessionExpiresBeforeDecision(t *testing.T) {
	t.Parallel()

	params := validSessionParams(t)
	session, _, err := NewSession(params, &sequenceTokenGenerator{tokens: []string{"invite-alice-secret", "invite-bob-secret"}})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	_, err = session.Decide("invite-alice-secret", DecisionApprove, params.ExpiresAt, mustApprovalSigner(t))
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("Decide() error = %v, want ErrSessionExpired", err)
	}
	if session.Status() != SessionStatusExpired {
		t.Fatalf("Status() = %q, want %q", session.Status(), SessionStatusExpired)
	}
}

func TestSessionExpireUpdatesPendingSessionOnly(t *testing.T) {
	t.Parallel()

	params := validSessionParams(t)
	session, _, err := NewSession(params, &sequenceTokenGenerator{tokens: []string{"invite-alice-secret", "invite-bob-secret"}})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	if session.Expire(params.ExpiresAt.Add(-time.Second)) {
		t.Fatal("Expire() expired the session before its deadline")
	}
	if !session.Expire(params.ExpiresAt) {
		t.Fatal("Expire() did not expire the session at its deadline")
	}
	if session.Status() != SessionStatusExpired {
		t.Fatalf("Status() = %q, want %q", session.Status(), SessionStatusExpired)
	}
	if session.Expire(params.ExpiresAt.Add(time.Second)) {
		t.Fatal("Expire() reported a second state transition")
	}
}

func TestInvitationSnapshotsCannotMutateSession(t *testing.T) {
	t.Parallel()

	params := validSessionParams(t)
	session, _, err := NewSession(params, &sequenceTokenGenerator{tokens: []string{"invite-alice-secret", "invite-bob-secret"}})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := session.Decide("invite-alice-secret", DecisionApprove, params.CreatedAt.Add(time.Minute), mustApprovalSigner(t)); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	snapshot := session.Invitations()
	changedDecision := DecisionVeto
	*snapshot[0].Decision = changedDecision
	snapshot[0].Label = "Changed"

	fresh := session.Invitations()
	if fresh[0].Label != "Alice" || fresh[0].Decision == nil || *fresh[0].Decision != DecisionApprove {
		t.Fatalf("external snapshot mutated session: %#v", fresh[0])
	}
}

func TestNewSessionValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*SessionParams)
		wantField string
	}{
		{name: "session ID prefix", mutate: func(params *SessionParams) {
			params.SessionID = mustApprovalID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
		}, wantField: "sessionId"},
		{name: "intent ID prefix", mutate: func(params *SessionParams) {
			params.IntentID = mustApprovalID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
		}, wantField: "intentId"},
		{name: "two approvers required", mutate: func(params *SessionParams) { params.ApproverLabels = []string{"Alice"} }, wantField: "approvers"},
		{name: "unique approvers", mutate: func(params *SessionParams) { params.ApproverLabels = []string{"Alice", "Alice"} }, wantField: "approvers"},
		{name: "approver label required", mutate: func(params *SessionParams) { params.ApproverLabels = []string{"Alice", ""} }, wantField: "approvers"},
		{name: "expiration after creation", mutate: func(params *SessionParams) { params.ExpiresAt = params.CreatedAt }, wantField: "expiresAt"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			params := validSessionParams(t)
			test.mutate(&params)
			_, _, err := NewSession(params, &sequenceTokenGenerator{tokens: []string{"invite-alice-secret", "invite-bob-secret"}})
			assertApprovalValidationField(t, err, test.wantField)
		})
	}
}

func TestApprovalTokenRejectsModifiedOrExpiredIntent(t *testing.T) {
	t.Parallel()

	params := validSessionParams(t)
	signer := mustApprovalSigner(t)
	token, hash, err := signer.Issue(params.SessionID, params.IntentID, params.IntentHash, params.ExpiresAt)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if token == "" || hash == "" || token == hash {
		t.Fatalf("token/hash = %q/%q", token, hash)
	}

	modifiedHash, err := intents.ParseSHA256Digest(strings.Repeat("b", 64))
	if err != nil {
		t.Fatalf("ParseSHA256Digest() error = %v", err)
	}
	if _, err := signer.Verify(token, params.SessionID, params.IntentID, modifiedHash, params.CreatedAt); !errors.Is(err, ErrApprovalTokenInvalid) {
		t.Fatalf("modified-intent Verify() error = %v", err)
	}
	if _, err := signer.Verify(token, params.SessionID, params.IntentID, params.IntentHash, params.ExpiresAt); !errors.Is(err, ErrApprovalTokenExpired) {
		t.Fatalf("expired Verify() error = %v", err)
	}

	tampered := token[:len(token)-1] + "A"
	if _, err := signer.Verify(tampered, params.SessionID, params.IntentID, params.IntentHash, params.CreatedAt); !errors.Is(err, ErrApprovalTokenInvalid) {
		t.Fatalf("tampered Verify() error = %v", err)
	}
}

func TestApprovalTokenSignerRequiresStrongSecret(t *testing.T) {
	t.Parallel()

	if _, err := NewApprovalTokenSigner([]byte("short")); err == nil {
		t.Fatal("NewApprovalTokenSigner() accepted a short secret")
	}
}

func validSessionParams(t *testing.T) SessionParams {
	t.Helper()
	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	intentHash, err := intents.ParseSHA256Digest(strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("ParseSHA256Digest() error = %v", err)
	}
	return SessionParams{
		SessionID:      mustApprovalID(t, "aps_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.ApprovalIDPrefix),
		IntentID:       mustApprovalID(t, "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.IntentIDPrefix),
		IntentHash:     intentHash,
		ApproverLabels: []string{"Alice", "Bob"},
		CreatedAt:      createdAt,
		ExpiresAt:      createdAt.Add(10 * time.Minute),
	}
}

func mustApprovalSigner(t *testing.T) *ApprovalTokenSigner {
	t.Helper()
	signer, err := NewApprovalTokenSigner([]byte(strings.Repeat("s", 32)))
	if err != nil {
		t.Fatalf("NewApprovalTokenSigner() error = %v", err)
	}
	return signer
}

func mustApprovalID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatalf("ParseID(%q) error = %v", raw, err)
	}
	return identifier
}

func assertApprovalValidationField(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error for %q", field)
	}

	var validationErrors domain.ValidationErrors
	if errors.As(err, &validationErrors) {
		if !validationErrors.HasField(field) {
			t.Fatalf("validation errors = %#v, want field %q", validationErrors, field)
		}
		return
	}

	var validationError domain.ValidationError
	if errors.As(err, &validationError) && validationError.Field == field {
		return
	}
	t.Fatalf("error = %#v, want validation error for %q", err, field)
}

type sequenceTokenGenerator struct {
	tokens []string
	index  int
}

func (generator *sequenceTokenGenerator) NewToken() (string, error) {
	token := generator.tokens[generator.index]
	generator.index++
	return token, nil
}
