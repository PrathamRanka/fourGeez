package authorization

import (
	"errors"
	"net/http"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

type ConfirmationGrantHTTPController struct {
	service *ConfirmationGrantService
}

func NewConfirmationGrantHTTPController(service *ConfirmationGrantService) *ConfirmationGrantHTTPController {
	return &ConfirmationGrantHTTPController{service: service}
}

func (controller *ConfirmationGrantHTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(
		"POST /v1/sellers/{sellerId}/mcp-confirmation-grants",
		api.RequireSeller(http.HandlerFunc(controller.create)),
	)
}

func (controller *ConfirmationGrantHTTPController) create(response http.ResponseWriter, request *http.Request) {
	sellerID, err := domain.ParseID(request.PathValue("sellerId"), domain.SellerIDPrefix)
	if err != nil {
		writeConfirmationError(response, request, err)
		return
	}
	var input CreateConfirmationGrantRequest
	if err := api.DecodeJSON(response, request, &input); err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	created, err := controller.service.Create(request.Context(), principal.Subject, sellerID, input)
	if err != nil {
		writeConfirmationError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Pragma", "no-cache")
	_ = api.WriteJSON(response, http.StatusCreated, created)
}

func writeConfirmationError(response http.ResponseWriter, request *http.Request, err error) {
	status := http.StatusServiceUnavailable
	code := api.ErrorCodeDependencyUnavailable
	message := "confirmation authorization state is unavailable"
	var validationError domain.ValidationError
	var validationErrors domain.ValidationErrors
	switch {
	case errors.As(err, &validationError), errors.As(err, &validationErrors):
		status, code, message = http.StatusBadRequest, api.ErrorCodeBadRequest, err.Error()
	case errors.Is(err, persistence.ErrNotFound):
		status, code, message = http.StatusNotFound, api.ErrorCodeNotFound, "credential was not found"
	case errors.Is(err, ErrSubscriptionInactive):
		status, code, message = http.StatusForbidden, api.ErrorCodeSubscriptionInactive, "seller subscription is inactive"
	case errors.Is(err, ErrConfirmationDenied):
		status, code, message = http.StatusForbidden, api.ErrorCodePermissionDenied, "confirmation grant is not permitted"
	case errors.Is(err, persistence.ErrAlreadyExists), errors.Is(err, persistence.ErrConditionFailed):
		status, code, message = http.StatusConflict, api.ErrorCodeConflict, "confirmation grant state conflict"
	}
	response.Header().Set("Cache-Control", "no-store")
	api.WriteError(response, request, status, code, message, nil)
}
