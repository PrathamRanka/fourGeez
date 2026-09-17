package approvals

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const approvalNoStoreValue = "no-store"

// HTTPController exposes approval-session use cases through HTTP.
type HTTPController struct {
	service          *Service
	idempotencyStore domain.IdempotencyStore
}

// NewHTTPController creates the approval-session HTTP controller.
func NewHTTPController(
	service *Service,
	idempotencyStore domain.IdempotencyStore,
) *HTTPController {
	return &HTTPController{
		service:          service,
		idempotencyStore: idempotencyStore,
	}
}

// RegisterRoutes registers the documented approval-session endpoints.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(
		"POST /v1/intents/{intentId}/approval-sessions",
		api.RequireAgent(http.HandlerFunc(controller.create)),
	)
	mux.HandleFunc(
		"GET /v1/approval-sessions/{sessionId}",
		controller.get,
	)
	mux.HandleFunc(
		"POST /v1/approval-sessions/{sessionId}/decisions",
		controller.decide,
	)
}

// create starts a two-person session and returns raw invitations once.
func (controller *HTTPController) create(
	response http.ResponseWriter,
	request *http.Request,
) {
	response.Header().Set("Cache-Control", approvalNoStoreValue)
	intentID, err := domain.ParseID(
		request.PathValue("intentId"),
		domain.IntentIDPrefix,
	)
	if err != nil {
		api.WriteError(
			response,
			request,
			http.StatusNotFound,
			api.ErrorCodeNotFound,
			"purchase intent not found",
			nil,
		)
		return
	}

	requestBody, err := readApprovalRequestBody(response, request)
	if err != nil {
		api.WriteError(
			response,
			request,
			http.StatusBadRequest,
			api.ErrorCodeBadRequest,
			"invalid request body",
			nil,
		)
		return
	}
	var input CreateSessionRequest
	if err := api.DecodeJSONBytes(requestBody, &input); err != nil {
		api.WriteError(
			response,
			request,
			http.StatusBadRequest,
			api.ErrorCodeBadRequest,
			"invalid request body",
			nil,
		)
		return
	}

	principal, _ := api.PrincipalFromContext(request.Context())
	scope := principal.Subject + ":createApprovalSession:" + intentID.String()
	decision, err := api.CheckIdempotency(
		request.Context(),
		controller.idempotencyStore,
		scope,
		request.Header.Get("Idempotency-Key"),
		requestBody,
	)
	if err != nil {
		writeApprovalIdempotencyError(response, request, err)
		return
	}
	if decision.Replay {
		writeApprovalReplay(response, decision)
		return
	}

	session, err := controller.service.Create(
		request.Context(),
		intentID,
		input,
	)
	if err != nil {
		writeApprovalError(response, request, err)
		return
	}
	controller.writeMutation(
		response,
		request,
		scope,
		decision,
		http.StatusCreated,
		session,
	)
}

// get returns a redacted snapshot to an agent or a session invitee.
func (controller *HTTPController) get(
	response http.ResponseWriter,
	request *http.Request,
) {
	response.Header().Set("Cache-Control", approvalNoStoreValue)
	sessionID, err := parseApprovalSessionID(request)
	if err != nil {
		writeApprovalError(response, request, err)
		return
	}

	_, agentAuthenticated := api.AuthenticateAgentRequest(
		request.Context(),
		request.Header.Get(api.AgentKeyHeader),
	)
	if !agentAuthenticated {
		err := controller.service.AuthorizeInvitation(
			request.Context(),
			sessionID,
			request.URL.Query().Get("token"),
		)
		if err != nil {
			writeApprovalError(response, request, err)
			return
		}
	}

	session, err := controller.service.Get(request.Context(), sessionID)
	if err != nil {
		writeApprovalError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, session)
}

// decide records one invitee decision and returns an approval token once.
func (controller *HTTPController) decide(
	response http.ResponseWriter,
	request *http.Request,
) {
	response.Header().Set("Cache-Control", approvalNoStoreValue)
	sessionID, err := parseApprovalSessionID(request)
	if err != nil {
		writeApprovalError(response, request, err)
		return
	}
	rawInvitationToken := request.URL.Query().Get("token")
	if rawInvitationToken == "" {
		writeApprovalError(response, request, ErrInvitationInvalid)
		return
	}

	requestBody, err := readApprovalRequestBody(response, request)
	if err != nil {
		api.WriteError(
			response,
			request,
			http.StatusBadRequest,
			api.ErrorCodeBadRequest,
			"invalid request body",
			nil,
		)
		return
	}
	var input DecideRequest
	if err := api.DecodeJSONBytes(requestBody, &input); err != nil {
		api.WriteError(
			response,
			request,
			http.StatusBadRequest,
			api.ErrorCodeBadRequest,
			"invalid request body",
			nil,
		)
		return
	}

	scope := "approvalDecision:" + sessionID.String() + ":" + hashToken(rawInvitationToken)
	idempotencyDecision, err := api.CheckIdempotency(
		request.Context(),
		controller.idempotencyStore,
		scope,
		request.Header.Get("Idempotency-Key"),
		requestBody,
	)
	if err != nil {
		writeApprovalIdempotencyError(response, request, err)
		return
	}
	if idempotencyDecision.Replay {
		writeApprovalReplay(response, idempotencyDecision)
		return
	}

	session, err := controller.service.Decide(
		request.Context(),
		sessionID,
		rawInvitationToken,
		input,
	)
	if err != nil {
		writeApprovalError(response, request, err)
		return
	}
	controller.writeMutation(
		response,
		request,
		scope,
		idempotencyDecision,
		http.StatusOK,
		session,
	)
}

// writeMutation stores and writes one completed idempotent mutation response.
func (controller *HTTPController) writeMutation(
	response http.ResponseWriter,
	request *http.Request,
	scope string,
	decision api.IdempotencyDecision,
	status int,
	value SessionResponse,
) {
	encoded, err := json.Marshal(value)
	if err != nil {
		api.WriteError(
			response,
			request,
			http.StatusInternalServerError,
			api.ErrorCodeInternal,
			"response encoding failed",
			nil,
		)
		return
	}
	encoded = append(encoded, '\n')
	if err := api.SaveIdempotency(
		request.Context(),
		controller.idempotencyStore,
		scope,
		decision,
		status,
		encoded,
		time.Now().UTC(),
	); err != nil {
		api.WriteError(
			response,
			request,
			http.StatusInternalServerError,
			api.ErrorCodeInternal,
			"idempotency persistence failed",
			nil,
		)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_, _ = response.Write(encoded)
}

// readApprovalRequestBody captures a bounded body for strict decoding and replay.
func readApprovalRequestBody(
	response http.ResponseWriter,
	request *http.Request,
) ([]byte, error) {
	request.Body = http.MaxBytesReader(
		response,
		request.Body,
		api.MaximumJSONBodyBytes,
	)
	return io.ReadAll(request.Body)
}

// parseApprovalSessionID validates a path session identifier without disclosure.
func parseApprovalSessionID(request *http.Request) (domain.ID, error) {
	sessionID, err := domain.ParseID(
		request.PathValue("sessionId"),
		domain.ApprovalIDPrefix,
	)
	if err != nil {
		return "", persistence.ErrNotFound
	}
	return sessionID, nil
}

// writeApprovalReplay writes a previously stored mutation response.
func writeApprovalReplay(
	response http.ResponseWriter,
	decision api.IdempotencyDecision,
) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(decision.Status)
	_, _ = response.Write(decision.Body)
}

// writeApprovalIdempotencyError maps mutation-key failures consistently.
func writeApprovalIdempotencyError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	status := http.StatusBadRequest
	code := api.ErrorCodeBadRequest
	if errors.Is(err, api.ErrIdempotencyConflict) {
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}

// writeApprovalError maps approval and persistence errors to stable responses.
func writeApprovalError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	var validationErrors domain.ValidationErrors
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	switch {
	case errors.As(err, &validationErrors), errors.Is(err, ErrApprovalNotRequired):
		status = http.StatusUnprocessableEntity
		code = api.ErrorCodeUnprocessable
	case errors.Is(err, ErrInvitationInvalid):
		status = http.StatusUnauthorized
		code = api.ErrorCodeUnauthorized
	case errors.Is(err, ErrSessionExpired):
		status = http.StatusGone
		code = api.ErrorCodeGone
	case errors.Is(err, ErrInvitationUsed),
		errors.Is(err, ErrSessionResolved),
		errors.Is(err, persistence.ErrAlreadyExists),
		errors.Is(err, persistence.ErrConditionFailed):
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	case errors.Is(err, persistence.ErrNotFound):
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
