package approvals

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

const maximumApproverLabelLength = 80

// ErrApprovalNotRequired reports an invalid session request for a ready intent.
var ErrApprovalNotRequired = errors.New("purchase intent does not require approval")

// Service coordinates approval rules with persistence and signing boundaries.
type Service struct {
	repository       Repository
	intentRepository IntentRepository
	idGenerator      domain.IDGenerator
	tokenGenerator   TokenGenerator
	tokenSigner      *ApprovalTokenSigner
	clock            domain.Clock
	publicBaseURL    string
	eventPublisher   EventPublisher
}

// SetEventPublisher attaches the optional realtime approval event boundary.
func (service *Service) SetEventPublisher(publisher EventPublisher) {
	service.eventPublisher = publisher
}

// NewService creates the approval application service.
func NewService(
	repository Repository,
	intentRepository IntentRepository,
	idGenerator domain.IDGenerator,
	tokenGenerator TokenGenerator,
	tokenSigner *ApprovalTokenSigner,
	clock domain.Clock,
	publicBaseURL string,
) *Service {
	return &Service{
		repository:       repository,
		intentRepository: intentRepository,
		idGenerator:      idGenerator,
		tokenGenerator:   tokenGenerator,
		tokenSigner:      tokenSigner,
		clock:            clock,
		publicBaseURL:    strings.TrimSuffix(publicBaseURL, "/"),
	}
}

// Create starts a two-person approval session for an approval-bound intent.
func (service *Service) Create(
	ctx context.Context,
	intentID domain.ID,
	request CreateSessionRequest,
) (SessionResponse, error) {
	purchaseIntent, err := service.intentRepository.Get(ctx, intentID)
	if err != nil {
		return SessionResponse{}, err
	}
	if !purchaseIntent.RequiresApproval() {
		return SessionResponse{}, ErrApprovalNotRequired
	}

	sessionID, err := service.idGenerator.New(domain.ApprovalIDPrefix)
	if err != nil {
		return SessionResponse{}, err
	}
	labels := make([]string, len(request.Approvers))
	for index, approver := range request.Approvers {
		labels[index] = approver.Label
	}
	session, grants, err := NewSession(SessionParams{
		SessionID:      sessionID,
		IntentID:       purchaseIntent.IntentID(),
		IntentHash:     purchaseIntent.IntentHash(),
		ApproverLabels: labels,
		CreatedAt:      domain.NewTimestamp(service.clock.Now()),
		ExpiresAt:      purchaseIntent.ExpiresAt(),
	}, service.tokenGenerator)
	if err != nil {
		return SessionResponse{}, err
	}
	if err := service.repository.Create(ctx, session); err != nil {
		return SessionResponse{}, err
	}
	return service.response(session, grants, ""), nil
}

// Get returns a redacted current session after applying expiration.
func (service *Service) Get(ctx context.Context, sessionID domain.ID) (SessionResponse, error) {
	session, err := service.repository.Get(ctx, sessionID)
	if err != nil {
		return SessionResponse{}, err
	}
	expectedVersion := session.Version()
	if session.Expire(domain.NewTimestamp(service.clock.Now())) {
		if err := service.repository.Update(ctx, session, expectedVersion); err != nil {
			return SessionResponse{}, err
		}
		response := service.response(session, nil, "")
		service.publishEvent(ctx, EventTypeSessionResolved, response)
		return response, nil
	}
	return service.response(session, nil, ""), nil
}

// Decide records one invitation decision and returns a token only on resolution.
func (service *Service) Decide(
	ctx context.Context,
	sessionID domain.ID,
	rawInvitationToken string,
	request DecideRequest,
) (SessionResponse, error) {
	session, err := service.repository.Get(ctx, sessionID)
	if err != nil {
		return SessionResponse{}, err
	}
	expectedVersion := session.Version()
	result, err := session.Decide(
		rawInvitationToken,
		request.Decision,
		domain.NewTimestamp(service.clock.Now()),
		service.tokenSigner,
	)
	if err != nil {
		return SessionResponse{}, err
	}
	if err := service.repository.Update(ctx, session, expectedVersion); err != nil {
		return SessionResponse{}, err
	}
	response := service.response(session, nil, result.ApprovalToken)
	service.publishEvent(ctx, EventTypeApprovalDecided, response)
	if response.Status != SessionStatusPending {
		service.publishEvent(ctx, EventTypeSessionResolved, response)
	}
	return response, nil
}

// publishEvent sends best-effort updates because REST snapshots remain authoritative.
func (service *Service) publishEvent(
	ctx context.Context,
	eventType EventType,
	response SessionResponse,
) {
	if service.eventPublisher == nil {
		return
	}
	service.eventPublisher.PublishApprovalEvent(ctx, eventType, response)
}

// AuthorizeInvitation verifies that a token belongs to the requested session.
func (service *Service) AuthorizeInvitation(
	ctx context.Context,
	sessionID domain.ID,
	rawInvitationToken string,
) error {
	session, err := service.repository.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.Expire(domain.NewTimestamp(service.clock.Now())) {
		return ErrSessionExpired
	}
	if _, exists := session.invitationByHash[hashToken(rawInvitationToken)]; !exists {
		return ErrInvitationInvalid
	}
	return nil
}

// response converts a session into its redacted API representation.
func (service *Service) response(
	session Session,
	grants []InvitationGrant,
	approvalToken string,
) SessionResponse {
	decisions := make([]DecisionView, 0, len(session.invitations))
	for _, invitation := range session.Invitations() {
		if invitation.Decision == nil || invitation.DecidedAt == nil {
			continue
		}
		decisions = append(decisions, DecisionView{
			Label:     invitation.Label,
			Decision:  *invitation.Decision,
			DecidedAt: *invitation.DecidedAt,
		})
	}
	invitationLinks := make([]InvitationLink, 0, len(grants))
	for _, grant := range grants {
		invitationURL := service.publicBaseURL + "/approve/" + session.sessionID.String() + "?token=" + url.QueryEscape(grant.Token)
		invitationLinks = append(invitationLinks, InvitationLink{Label: grant.Label, URL: invitationURL})
	}
	return SessionResponse{
		SessionID:         session.sessionID,
		IntentID:          session.intentID,
		IntentHash:        session.intentHash,
		RequiredApprovals: session.requiredApprovals,
		Decisions:         decisions,
		Status:            session.status,
		ExpiresAt:         session.expiresAt,
		CreatedAt:         session.createdAt,
		UpdatedAt:         session.updatedAt,
		Version:           session.version,
		Invitations:       invitationLinks,
		ApprovalToken:     approvalToken,
	}
}

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
		return DecisionResult{}, domain.NewValidationError(
			"decision",
			"supported",
			"must be approve or veto",
		)
	}
	if signer == nil {
		return DecisionResult{}, domain.NewValidationError(
			"approvalTokenSigner",
			"required",
			"is required",
		)
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
	if session.status != SessionStatusPending ||
		now.Time().Before(session.expiresAt.Time()) {
		return false
	}

	session.status = SessionStatusExpired
	session.updatedAt = now
	session.version++
	return true
}
