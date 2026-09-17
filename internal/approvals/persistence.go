package approvals

import (
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

// Snapshot contains the complete persisted representation of an approval session.
type Snapshot struct {
	SessionID         domain.ID            `json:"sessionId"`
	IntentID          domain.ID            `json:"intentId"`
	IntentHash        intents.SHA256Digest `json:"intentHash"`
	RequiredApprovals int                  `json:"requiredApprovals"`
	Invitations       []Invitation         `json:"invitations"`
	Status            SessionStatus        `json:"status"`
	ApprovalTokenHash string               `json:"approvalTokenHash,omitempty"`
	CreatedAt         domain.Timestamp     `json:"createdAt"`
	UpdatedAt         domain.Timestamp     `json:"updatedAt"`
	ExpiresAt         domain.Timestamp     `json:"expiresAt"`
	Version           uint64               `json:"version"`
}

// Snapshot returns an isolated approval-session persistence representation.
func (session Session) Snapshot() Snapshot {
	return Snapshot{
		SessionID:         session.sessionID,
		IntentID:          session.intentID,
		IntentHash:        session.intentHash,
		RequiredApprovals: session.requiredApprovals,
		Invitations:       session.Invitations(),
		Status:            session.status,
		ApprovalTokenHash: session.approvalTokenHash,
		CreatedAt:         session.createdAt,
		UpdatedAt:         session.updatedAt,
		ExpiresAt:         session.expiresAt,
		Version:           session.version,
	}
}

// Restore validates and recreates an approval session from storage.
func Restore(snapshot Snapshot) (Session, error) {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(snapshot.SessionID.String(), domain.ApprovalIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("sessionId", "format", "stored session ID is invalid"))
	}
	if _, err := domain.ParseID(snapshot.IntentID.String(), domain.IntentIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("intentId", "format", "stored intent ID is invalid"))
	}
	if _, err := intents.ParseSHA256Digest(snapshot.IntentHash.String()); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("intentHash", "format", "stored intent hash is invalid"))
	}
	if snapshot.RequiredApprovals != requiredApprovalsForHackathon || len(snapshot.Invitations) != requiredApprovalsForHackathon {
		validationErrors = append(validationErrors, domain.NewValidationError("invitations", "count", "stored session must contain exactly two invitations"))
	}
	if snapshot.Version == 0 || snapshot.CreatedAt.Time().IsZero() || snapshot.UpdatedAt.Time().IsZero() || snapshot.ExpiresAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("version", "persistence", "stored session metadata is invalid"))
	}
	if len(validationErrors) > 0 {
		return Session{}, validationErrors
	}

	session := Session{
		sessionID:         snapshot.SessionID,
		intentID:          snapshot.IntentID,
		intentHash:        snapshot.IntentHash,
		requiredApprovals: snapshot.RequiredApprovals,
		invitations:       snapshot.Invitations,
		invitationByHash:  make(map[string]int, len(snapshot.Invitations)),
		status:            snapshot.Status,
		approvalTokenHash: snapshot.ApprovalTokenHash,
		createdAt:         snapshot.CreatedAt,
		updatedAt:         snapshot.UpdatedAt,
		expiresAt:         snapshot.ExpiresAt,
		version:           snapshot.Version,
	}
	for index, invitation := range session.invitations {
		session.invitationByHash[invitation.TokenHash] = index
	}

	return session.Clone(), nil
}
