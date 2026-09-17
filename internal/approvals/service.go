package approvals

import (
	"errors"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

const maximumApproverLabelLength = 80

// NewSession validates the binding and creates secure invitation grants.
func NewSession(
	params SessionParams,
	tokenGenerator TokenGenerator,
) (Session, []InvitationGrant, error) {
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
		session.invitations = append(session.invitations, Invitation{
			Label:     strings.TrimSpace(label),
			TokenHash: tokenHash,
		})
		grants = append(grants, InvitationGrant{
			Label:     strings.TrimSpace(label),
			Token:     token,
			ExpiresAt: params.ExpiresAt,
		})
	}
	return session, grants, nil
}

// validateSessionParams validates session identity, approvers, and lifetime.
func validateSessionParams(
	params SessionParams,
	tokenGenerator TokenGenerator,
) domain.ValidationErrors {
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
		firstLabel := strings.TrimSpace(params.ApproverLabels[0])
		secondLabel := strings.TrimSpace(params.ApproverLabels[1])
		if firstLabel == "" || secondLabel == "" ||
			len(firstLabel) > maximumApproverLabelLength ||
			len(secondLabel) > maximumApproverLabelLength {
			validationErrors = append(validationErrors, domain.NewValidationError("approvers", "length", "each approver label must contain 1-80 characters"))
		} else if strings.EqualFold(firstLabel, secondLabel) {
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

// approvalCount counts immutable approve decisions.
func (session Session) approvalCount() int {
	count := 0
	for _, invitation := range session.invitations {
		if invitation.Decision != nil && *invitation.Decision == DecisionApprove {
			count++
		}
	}
	return count
}
