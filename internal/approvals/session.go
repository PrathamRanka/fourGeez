package approvals

import (
	"errors"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

const (
	requiredApprovalsForHackathon = 2
	maximumApproverLabelLength    = 80
)

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

// NewSession validates the binding and returns raw invitation grants separately
// from the session so only token hashes need to be persisted.
func NewSession(params SessionParams, tokenGenerator TokenGenerator) (Session, []InvitationGrant, error) {
	validationErrors := validateSessionParams(params, tokenGenerator)
	if len(validationErrors) > 0 {
		return Session{}, nil, validationErrors
	}

	session := Session{
		sessionID:         params.SessionID,
		intentID:          params.IntentID,
		intentHash:        params.IntentHash,
		requiredApprovals: requiredApprovalsForHackathon,
		invitations:       make([]Invitation, 0, requiredApprovalsForHackathon),
		invitationByHash:  make(map[string]int, requiredApprovalsForHackathon),
		status:            SessionStatusPending,
		createdAt:         params.CreatedAt,
		updatedAt:         params.CreatedAt,
		expiresAt:         params.ExpiresAt,
		version:           1,
	}
	grants := make([]InvitationGrant, 0, requiredApprovalsForHackathon)

	for _, label := range params.ApproverLabels {
		token, err := tokenGenerator.NewToken()
		if err != nil {
			return Session{}, nil, err
		}
		tokenHash := hashToken(token)
		if _, exists := session.invitationByHash[tokenHash]; exists {
			return Session{}, nil, errors.New("token generator produced a duplicate invitation")
		}

		session.invitationByHash[tokenHash] = len(session.invitations)
		session.invitations = append(session.invitations, Invitation{Label: strings.TrimSpace(label), TokenHash: tokenHash})
		grants = append(grants, InvitationGrant{Label: strings.TrimSpace(label), Token: token, ExpiresAt: params.ExpiresAt})
	}
	return session, grants, nil
}

// Decide records one invitation's decision and resolves the session when a veto
// arrives or all required approvals are present.
func (session *Session) Decide(rawInvitationToken string, decision Decision, decidedAt domain.Timestamp, signer *ApprovalTokenSigner) (DecisionResult, error) {
	if session.status != SessionStatusPending {
		return DecisionResult{}, ErrSessionResolved
	}
	if session.Expire(decidedAt) {
		return DecisionResult{}, ErrSessionExpired
	}
	if decision != DecisionApprove && decision != DecisionVeto {
		return DecisionResult{}, domain.NewValidationError("decision", "supported", "must be approve or veto")
	}
	if signer == nil {
		return DecisionResult{}, domain.NewValidationError("approvalTokenSigner", "required", "is required")
	}

	invitationIndex, exists := session.invitationByHash[hashToken(rawInvitationToken)]
	if !exists {
		return DecisionResult{}, ErrInvitationInvalid
	}
	invitation := &session.invitations[invitationIndex]
	if invitation.Decision != nil {
		return DecisionResult{}, ErrInvitationUsed
	}

	decisionCopy := decision
	decidedAtCopy := decidedAt
	invitation.Decision = &decisionCopy
	invitation.DecidedAt = &decidedAtCopy
	session.updatedAt = decidedAt
	session.version++

	if decision == DecisionVeto {
		session.status = SessionStatusVetoed
		return DecisionResult{Status: session.status}, nil
	}
	if session.approvalCount() < session.requiredApprovals {
		return DecisionResult{Status: session.status}, nil
	}

	token, tokenHash, err := signer.Issue(session.sessionID, session.intentID, session.intentHash, session.expiresAt)
	if err != nil {
		return DecisionResult{}, err
	}
	session.status = SessionStatusApproved
	session.approvalTokenHash = tokenHash
	return DecisionResult{Status: session.status, ApprovalToken: token}, nil
}

// Expire transitions a pending session when now reaches or passes its deadline.
// It returns true only when this call performs the transition.
func (session *Session) Expire(now domain.Timestamp) bool {
	if session.status != SessionStatusPending || now.Time().Before(session.expiresAt.Time()) {
		return false
	}
	session.status = SessionStatusExpired
	session.updatedAt = now
	session.version++
	return true
}

// Status returns the current persisted state.
func (session Session) Status() SessionStatus { return session.status }

// SessionID returns the approval-session identifier.
func (session Session) SessionID() domain.ID { return session.sessionID }

// IntentID returns the bound purchase-intent identifier.
func (session Session) IntentID() domain.ID { return session.intentID }

// IntentHash returns the exact intent digest approved by participants.
func (session Session) IntentHash() intents.SHA256Digest { return session.intentHash }

// RequiredApprovals returns the number of approvals required to resolve approved.
func (session Session) RequiredApprovals() int { return session.requiredApprovals }

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
func (session Session) ApprovalTokenHash() string { return session.approvalTokenHash }

// CreatedAt returns the session creation time.
func (session Session) CreatedAt() domain.Timestamp { return session.createdAt }

// UpdatedAt returns the latest state-change time.
func (session Session) UpdatedAt() domain.Timestamp { return session.updatedAt }

// ExpiresAt returns the approval deadline.
func (session Session) ExpiresAt() domain.Timestamp { return session.expiresAt }

// Version returns the optimistic-concurrency version.
func (session Session) Version() uint64 { return session.version }

func (session Session) approvalCount() int {
	count := 0
	for _, invitation := range session.invitations {
		if invitation.Decision != nil && *invitation.Decision == DecisionApprove {
			count++
		}
	}
	return count
}

func validateSessionParams(params SessionParams, tokenGenerator TokenGenerator) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.SessionID.String(), domain.ApprovalIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("sessionId", "format", "must be an approval-session identifier"))
	}
	if _, err := domain.ParseID(params.IntentID.String(), domain.IntentIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("intentId", "format", "must be a purchase-intent identifier"))
	}
	if _, err := intents.ParseSHA256Digest(params.IntentHash.String()); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("intentHash", "format", "must be a SHA-256 digest"))
	}
	if len(params.ApproverLabels) != requiredApprovalsForHackathon {
		validationErrors = append(validationErrors, domain.NewValidationError("approvers", "count", "must contain exactly two approvers"))
	} else {
		first := strings.TrimSpace(params.ApproverLabels[0])
		second := strings.TrimSpace(params.ApproverLabels[1])
		if first == "" || second == "" || len(first) > maximumApproverLabelLength || len(second) > maximumApproverLabelLength {
			validationErrors = append(validationErrors, domain.NewValidationError("approvers", "length", "each approver label must contain 1-80 characters"))
		} else if strings.EqualFold(first, second) {
			validationErrors = append(validationErrors, domain.NewValidationError("approvers", "unique", "approver labels must be unique"))
		}
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	if params.ExpiresAt.Time().IsZero() || !params.ExpiresAt.Time().After(params.CreatedAt.Time()) {
		validationErrors = append(validationErrors, domain.NewValidationError("expiresAt", "chronology", "must occur after creation"))
	}
	if tokenGenerator == nil {
		validationErrors = append(validationErrors, domain.NewValidationError("tokenGenerator", "required", "is required"))
	}
	return validationErrors
}
