package approvals

import "github.com/fourgeez/agentpay/internal/domain"

// Decide records one invitation decision and resolves the session when possible.
func (session *Session) Decide(
	rawInvitationToken string,
	decision Decision,
	decidedAt domain.Timestamp,
	signer *ApprovalTokenSigner,
) (DecisionResult, error) {
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

	token, tokenHash, err := signer.Issue(
		session.sessionID,
		session.intentID,
		session.intentHash,
		session.expiresAt,
	)
	if err != nil {
		return DecisionResult{}, err
	}
	session.status = SessionStatusApproved
	session.approvalTokenHash = tokenHash
	return DecisionResult{
		Status:        session.status,
		ApprovalToken: token,
	}, nil
}

// Expire transitions a pending session when its deadline is reached.
func (session *Session) Expire(now domain.Timestamp) bool {
	if session.status != SessionStatusPending || now.Time().Before(session.expiresAt.Time()) {
		return false
	}

	session.status = SessionStatusExpired
	session.updatedAt = now
	session.version++
	return true
}
