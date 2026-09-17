package approvals

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

const requiredApprovalsForHackathon = 2

var (
	ErrInvitationInvalid = errors.New("approval invitation is invalid")
	ErrInvitationUsed    = errors.New("approval invitation has already been used")
	ErrSessionExpired    = errors.New("approval session has expired")
	ErrSessionResolved   = errors.New("approval session is already resolved")
)

// SessionStatus is the current approval-session state.
type SessionStatus string

const (
	SessionStatusPending  SessionStatus = "pending"
	SessionStatusApproved SessionStatus = "approved"
	SessionStatusVetoed   SessionStatus = "vetoed"
	SessionStatusExpired  SessionStatus = "expired"
)

// Decision is an approver's immutable response.
type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionVeto    Decision = "veto"
)

// SessionParams contains values bound to an approval session.
type SessionParams struct {
	SessionID      domain.ID
	IntentID       domain.ID
	IntentHash     intents.SHA256Digest
	ApproverLabels []string
	CreatedAt      domain.Timestamp
	ExpiresAt      domain.Timestamp
}

// InvitationGrant contains the raw token returned only at session creation.
type InvitationGrant struct {
	Label     string
	Token     string
	ExpiresAt domain.Timestamp
}

// Invitation is the safe persisted representation of an approver invitation.
type Invitation struct {
	Label     string
	TokenHash string
	Decision  *Decision
	DecidedAt *domain.Timestamp
}

// DecisionResult is returned after an invitation is consumed.
type DecisionResult struct {
	Status        SessionStatus
	ApprovalToken string
}

// ApproverRequest identifies one approval participant label.
type ApproverRequest struct {
	Label string `json:"label"`
}

// CreateSessionRequest is the approval-session creation request.
type CreateSessionRequest struct {
	Approvers []ApproverRequest `json:"approvers"`
}

// DecideRequest is the approval decision request.
type DecideRequest struct {
	Decision Decision `json:"decision"`
}

// DecisionView is the public representation of a recorded decision.
type DecisionView struct {
	Label     string           `json:"label"`
	Decision  Decision         `json:"decision"`
	DecidedAt domain.Timestamp `json:"decidedAt"`
}

// InvitationLink is a one-time approval invitation returned at creation.
type InvitationLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// SessionResponse is the redacted approval-session API representation.
type SessionResponse struct {
	SessionID         domain.ID            `json:"sessionId"`
	IntentID          domain.ID            `json:"intentId"`
	IntentHash        intents.SHA256Digest `json:"intentHash"`
	RequiredApprovals int                  `json:"requiredApprovals"`
	Decisions         []DecisionView       `json:"decisions"`
	Status            SessionStatus        `json:"status"`
	ExpiresAt         domain.Timestamp     `json:"expiresAt"`
	CreatedAt         domain.Timestamp     `json:"createdAt"`
	UpdatedAt         domain.Timestamp     `json:"updatedAt"`
	Version           uint64               `json:"version"`
	Invitations       []InvitationLink     `json:"invitations,omitempty"`
	ApprovalToken     string               `json:"approvalToken,omitempty"`
}

// EventType identifies an approval event published to connected clients.
type EventType string

const (
	EventTypeSessionSnapshot EventType = "session.snapshot"
	EventTypeApproverJoined  EventType = "approver.joined"
	EventTypeApprovalDecided EventType = "approval.decided"
	EventTypeSessionResolved EventType = "session.resolved"
)

// EventPublisher distributes redacted approval-session changes.
type EventPublisher interface {
	PublishApprovalEvent(
		ctx context.Context,
		eventType EventType,
		session SessionResponse,
	)
}

// Repository is the persistence boundary consumed by approval use cases.
type Repository interface {
	Create(ctx context.Context, session Session) error
	Get(ctx context.Context, sessionID domain.ID) (Session, error)
	Update(ctx context.Context, session Session, expectedVersion uint64) error
}

// IntentRepository loads purchase intents for approval binding.
type IntentRepository interface {
	Get(ctx context.Context, intentID domain.ID) (intents.PurchaseIntent, error)
}

// Session binds human decisions to one immutable purchase-intent hash.
type Session struct {
	sessionID         domain.ID
	intentID          domain.ID
	intentHash        intents.SHA256Digest
	requiredApprovals int
	invitations       []Invitation
	invitationByHash  map[string]int
	status            SessionStatus
	approvalTokenHash string
	createdAt         domain.Timestamp
	updatedAt         domain.Timestamp
	expiresAt         domain.Timestamp
	version           uint64
}

// Status returns the current persisted state.
func (session Session) Status() SessionStatus {
	return session.status
}

// SessionID returns the approval-session identifier.
func (session Session) SessionID() domain.ID {
	return session.sessionID
}

// IntentID returns the bound purchase-intent identifier.
func (session Session) IntentID() domain.ID {
	return session.intentID
}

// IntentHash returns the exact intent digest approved by participants.
func (session Session) IntentHash() intents.SHA256Digest {
	return session.intentHash
}

// RequiredApprovals returns the number of approvals required to resolve approved.
func (session Session) RequiredApprovals() int {
	return session.requiredApprovals
}

// Invitations returns a copy that contains token hashes but never raw tokens.
func (session Session) Invitations() []Invitation {
	snapshots := make([]Invitation, len(session.invitations))
	for index, invitation := range session.invitations {
		snapshots[index] = invitation
		if invitation.Decision != nil {
			decisionCopy := *invitation.Decision
			snapshots[index].Decision = &decisionCopy
		}
		if invitation.DecidedAt != nil {
			decidedAtCopy := *invitation.DecidedAt
			snapshots[index].DecidedAt = &decidedAtCopy
		}
	}
	return snapshots
}

// ApprovalTokenHash returns the persisted token digest after approval.
func (session Session) ApprovalTokenHash() string {
	return session.approvalTokenHash
}

// CreatedAt returns the session creation time.
func (session Session) CreatedAt() domain.Timestamp {
	return session.createdAt
}

// UpdatedAt returns the latest state-change time.
func (session Session) UpdatedAt() domain.Timestamp {
	return session.updatedAt
}

// ExpiresAt returns the approval deadline.
func (session Session) ExpiresAt() domain.Timestamp {
	return session.expiresAt
}

// Version returns the optimistic-concurrency version.
func (session Session) Version() uint64 {
	return session.version
}

// Clone returns an independent in-memory persistence snapshot.
func (session Session) Clone() Session {
	cloned := session
	cloned.invitations = session.Invitations()
	cloned.invitationByHash = make(map[string]int, len(session.invitationByHash))
	for tokenHash, index := range session.invitationByHash {
		cloned.invitationByHash[tokenHash] = index
	}
	return cloned
}
