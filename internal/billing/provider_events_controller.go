package billing

import (
	"errors"
	"io"
	"net/http"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
)

const stripeSignatureHeader = "Stripe-Signature"

type StripeProviderEventHTTPController struct{ service *StripeProviderEventService }

func NewStripeProviderEventHTTPController(service *StripeProviderEventService) *StripeProviderEventHTTPController {
	return &StripeProviderEventHTTPController{service: service}
}

func (controller *StripeProviderEventHTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/billing/providers/stripe/webhooks", controller.ingest)
}

func (controller *StripeProviderEventHTTPController) ingest(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, api.MaximumJSONBodyBytes)
	rawBody, err := io.ReadAll(request.Body)
	if err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}
	_, err = controller.service.Ingest(request.Context(), rawBody, request.Header.Get(stripeSignatureHeader))
	if err != nil {
		controller.writeError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (controller *StripeProviderEventHTTPController) writeError(response http.ResponseWriter, request *http.Request, err error) {
	var validationError domain.ValidationError
	var validationErrors domain.ValidationErrors
	status := http.StatusServiceUnavailable
	code := api.ErrorCodeDependencyUnavailable
	switch {
	case errors.Is(err, ErrStripeSignatureInvalid), errors.Is(err, ErrProviderEventEnvironmentMismatch):
		status = http.StatusUnauthorized
		code = api.ErrorCodeUnauthorized
	case errors.Is(err, ErrProviderEventPayloadConflict):
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	case errors.As(err, &validationError), errors.As(err, &validationErrors):
		status = http.StatusBadRequest
		code = api.ErrorCodeBadRequest
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
