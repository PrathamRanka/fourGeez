package integrations

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

// HTTPController exposes seller credential lifecycle operations.
type HTTPController struct {
	service          *Service
	idempotencyStore domain.IdempotencyStore
}

// NewHTTPController creates the integration credential HTTP controller.
func NewHTTPController(
	service *Service,
	idempotencyStore domain.IdempotencyStore,
) *HTTPController {
	return &HTTPController{
		service:          service,
		idempotencyStore: idempotencyStore,
	}
}

// RegisterRoutes registers seller-authenticated credential endpoints.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(
		"POST /v1/sellers/{sellerId}/integration-credentials",
		api.RequireSeller(http.HandlerFunc(controller.create)),
	)
	mux.Handle(
		"GET /v1/sellers/{sellerId}/integration-credentials",
		api.RequireSeller(http.HandlerFunc(controller.list)),
	)
	mux.Handle(
		"POST /v1/sellers/{sellerId}/integration-credentials/{credentialId}/revoke",
		api.RequireSeller(http.HandlerFunc(controller.revoke)),
	)
	mux.Handle(
		"POST /v1/sellers/{sellerId}/integration-credentials/{credentialId}/rotate",
		api.RequireSeller(http.HandlerFunc(controller.rotate)),
	)
}

func (controller *HTTPController) rotate(response http.ResponseWriter, request *http.Request) {
	sellerID, err := parseSellerID(request)
	if err != nil {
		controller.writeError(response, request, err)
		return
	}
	credentialID, err := domain.ParseID(request.PathValue("credentialId"), domain.CredentialIDPrefix)
	if err != nil {
		controller.writeError(response, request, err)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, api.MaximumJSONBodyBytes)
	requestBody, err := io.ReadAll(request.Body)
	if err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}
	var input RotateCredentialRequest
	if err := api.DecodeJSONBytes(requestBody, &input); err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	rotation, err := controller.service.Rotate(
		request.Context(),
		principal.Subject,
		sellerID,
		credentialID,
		input,
		RotationIdempotency{Key: request.Header.Get("Idempotency-Key"), RequestBody: requestBody},
	)
	if err != nil {
		controller.writeError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(response, http.StatusCreated, rotation)
}

// create issues a credential and returns raw token material once.
func (controller *HTTPController) create(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := parseSellerID(request)
	if err != nil {
		controller.writeError(response, request, err)
		return
	}
	var input CreateCredentialRequest
	controller.executeMutation(
		response,
		request,
		"createIntegrationCredential:"+sellerID.String(),
		&input,
		func(principal api.Principal) (any, error) {
			return controller.service.Create(
				request.Context(),
				principal.Subject,
				sellerID,
				input,
			)
		},
		http.StatusCreated,
		true,
	)
}

// list returns only redacted metadata for credentials owned by the seller.
func (controller *HTTPController) list(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := parseSellerID(request)
	if err != nil {
		controller.writeError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	credentials, err := controller.service.List(
		request.Context(),
		principal.Subject,
		sellerID,
	)
	if err != nil {
		controller.writeError(response, request, err)
		return
	}
	_ = api.WriteJSON(
		response,
		http.StatusOK,
		map[string]any{"items": credentials},
	)
}

// revoke disables one credential while retaining its audit metadata.
func (controller *HTTPController) revoke(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := parseSellerID(request)
	if err != nil {
		controller.writeError(response, request, err)
		return
	}
	credentialID, err := domain.ParseID(
		request.PathValue("credentialId"),
		domain.CredentialIDPrefix,
	)
	if err != nil {
		controller.writeError(response, request, err)
		return
	}
	var input RevokeCredentialRequest
	controller.executeMutation(
		response,
		request,
		"revokeIntegrationCredential:"+credentialID.String(),
		&input,
		func(principal api.Principal) (any, error) {
			return controller.service.Revoke(
				request.Context(),
				principal.Subject,
				sellerID,
				credentialID,
				input.ExpectedVersion,
			)
		},
		http.StatusOK,
		false,
	)
}

// executeMutation applies strict decoding and idempotency to credential writes.
func (controller *HTTPController) executeMutation(
	response http.ResponseWriter,
	request *http.Request,
	scope string,
	input any,
	execute func(api.Principal) (any, error),
	successStatus int,
	containsOneTimeToken bool,
) {
	request.Body = http.MaxBytesReader(
		response,
		request.Body,
		api.MaximumJSONBodyBytes,
	)
	requestBody, err := io.ReadAll(request.Body)
	if err != nil || api.DecodeJSONBytes(requestBody, input) != nil {
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
	idempotencyScope := principal.Subject + ":" + scope
	decision, err := api.CheckIdempotency(
		request.Context(),
		controller.idempotencyStore,
		idempotencyScope,
		request.Header.Get("Idempotency-Key"),
		requestBody,
	)
	if err != nil {
		status := http.StatusBadRequest
		code := api.ErrorCodeBadRequest
		if errors.Is(err, api.ErrIdempotencyConflict) {
			status = http.StatusConflict
			code = api.ErrorCodeConflict
		}
		api.WriteError(response, request, status, code, err.Error(), nil)
		return
	}
	if decision.Replay {
		if containsOneTimeToken {
			api.WriteError(
				response,
				request,
				http.StatusConflict,
				api.ErrorCodeConflict,
				"credential token cannot be replayed",
				nil,
			)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(decision.Status)
		_, _ = response.Write(decision.Body)
		return
	}

	result, err := execute(principal)
	if err != nil {
		controller.writeError(response, request, err)
		return
	}
	encodedResult, err := json.Marshal(result)
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
	encodedResult = append(encodedResult, '\n')
	storedResult, err := credentialIdempotencyResult(
		result,
		encodedResult,
		containsOneTimeToken,
	)
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
	if err := api.SaveIdempotency(
		request.Context(),
		controller.idempotencyStore,
		idempotencyScope,
		decision,
		successStatus,
		storedResult,
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
	if containsOneTimeToken {
		response.Header().Set("Cache-Control", "no-store")
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(successStatus)
	_, _ = response.Write(encodedResult)
}

// credentialIdempotencyResult prevents one-time tokens entering replay storage.
func credentialIdempotencyResult(
	result any,
	encodedResult []byte,
	containsOneTimeToken bool,
) ([]byte, error) {
	if !containsOneTimeToken {
		return encodedResult, nil
	}
	var redacted any
	switch credentialResult := result.(type) {
	case CredentialCreated:
		redacted = credentialResult.CredentialView
	case CredentialRotation:
		redacted = struct {
			Predecessor           CredentialView   `json:"predecessor"`
			Successor             CredentialView   `json:"successor"`
			SecretReplayExpiresAt domain.Timestamp `json:"secretReplayExpiresAt"`
		}{credentialResult.Predecessor, credentialResult.Successor, credentialResult.SecretReplayExpiresAt}
	default:
		return nil, errors.New("one-time credential response has an unexpected type")
	}
	redactedResult, err := json.Marshal(redacted)
	if err != nil {
		return nil, err
	}
	return append(redactedResult, '\n'), nil
}

// parseSellerID validates the seller path identifier.
func parseSellerID(request *http.Request) (domain.ID, error) {
	return domain.ParseID(
		request.PathValue("sellerId"),
		domain.SellerIDPrefix,
	)
}

// writeError maps credential failures to stable API responses.
func (controller *HTTPController) writeError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	var validationError domain.ValidationError
	var validationErrors domain.ValidationErrors
	var recoveryError RotationRecoveryError
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	var details map[string]any
	if errors.As(err, &validationError) ||
		errors.As(err, &validationErrors) {
		status = http.StatusBadRequest
		code = api.ErrorCodeBadRequest
	} else if errors.Is(err, persistence.ErrNotFound) {
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	} else if errors.Is(err, persistence.ErrAlreadyExists) ||
		errors.Is(err, persistence.ErrConditionFailed) ||
		errors.Is(err, ErrCredentialRevoked) ||
		errors.Is(err, ErrRotationIdempotencyConflict) {
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	} else if errors.As(err, &recoveryError) {
		status = http.StatusConflict
		code = api.ErrorCodeConflict
		details = map[string]any{
			"successorCredentialId": recoveryError.SuccessorCredentialID.String(),
			"recoveryAction":        "rotate_successor",
		}
	}
	api.WriteError(response, request, status, code, err.Error(), details)
}
