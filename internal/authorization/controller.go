package authorization

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
)

const ProjectKeyHeader = "X-AgentPay-Project-Key"

type HTTPController struct {
	service *AccessTokenService
}

func NewHTTPController(service *AccessTokenService) *HTTPController {
	return &HTTPController{service: service}
}

func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/jwks.json", controller.jwks)
	mux.HandleFunc("POST /v1/integration-access-tokens", controller.exchange)
}

func (controller *HTTPController) exchange(response http.ResponseWriter, request *http.Request) {
	projectKey := strings.TrimSpace(request.Header.Get(ProjectKeyHeader))
	if projectKey == "" {
		writeAuthorizationError(response, request, integrations.ErrCredentialInvalid)
		return
	}
	var input AccessTokenRequest
	if err := api.DecodeJSON(response, request, &input); err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}
	issued, err := controller.service.Exchange(request.Context(), projectKey, input)
	if err != nil {
		writeAuthorizationError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Pragma", "no-cache")
	_ = api.WriteJSON(response, http.StatusOK, issued)
}

func (controller *HTTPController) jwks(response http.ResponseWriter, request *http.Request) {
	keys, err := controller.service.JWKS(request.Context())
	if err != nil {
		writeAuthorizationError(response, request, err)
		return
	}
	encoded, err := json.Marshal(keys)
	if err != nil {
		writeAuthorizationError(response, request, ErrAuthorizationUnavailable)
		return
	}
	response.Header().Set("Cache-Control", "public, max-age=60")
	response.Header().Set("Content-Type", "application/jwk-set+json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(append(encoded, '\n'))
}

func writeAuthorizationError(response http.ResponseWriter, request *http.Request, err error) {
	status := http.StatusServiceUnavailable
	code := api.ErrorCodeDependencyUnavailable
	message := "authorization state is unavailable"
	var validationError domain.ValidationError
	var validationErrors domain.ValidationErrors
	switch {
	case errors.As(err, &validationError), errors.As(err, &validationErrors):
		status, code, message = http.StatusBadRequest, api.ErrorCodeBadRequest, err.Error()
	case errors.Is(err, integrations.ErrCredentialInvalid), errors.Is(err, integrations.ErrCredentialExpired):
		status, code, message = http.StatusUnauthorized, api.ErrorCodeInvalidCredential, "project key is invalid"
	case errors.Is(err, integrations.ErrCredentialRevoked), errors.Is(err, ErrAccessTokenRevoked):
		status, code, message = http.StatusUnauthorized, api.ErrorCodeTokenRevoked, "credential is revoked"
	case errors.Is(err, ErrInvalidAccessToken):
		status, code, message = http.StatusUnauthorized, api.ErrorCodeInvalidCredential, "access token is invalid"
	case errors.Is(err, ErrAccessTokenExpired):
		status, code, message = http.StatusUnauthorized, api.ErrorCodeTokenExpired, "access token has expired"
	case errors.Is(err, integrations.ErrSubscriptionInactive), errors.Is(err, ErrSubscriptionInactive):
		status, code, message = http.StatusForbidden, api.ErrorCodeSubscriptionInactive, "seller subscription is inactive"
	case errors.Is(err, integrations.ErrScopeDenied), errors.Is(err, ErrInsufficientScope):
		status, code, message = http.StatusForbidden, api.ErrorCodeInsufficientScope, "requested scope is not permitted"
	case errors.Is(err, domain.ErrRateLimitExceeded):
		status, code, message = http.StatusTooManyRequests, api.ErrorCodeRateLimited, "rate limit exceeded"
		response.Header().Set("Retry-After", "1")
	}
	response.Header().Set("Cache-Control", "no-store")
	api.WriteError(response, request, status, code, message, nil)
}
