package sellerworkspace

import (
	"errors"
	"net/http"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

type HTTPController struct {
	service   *Service
	principal AuthenticatedPrincipal
}

func NewHTTPController(service *Service, principal AuthenticatedPrincipal) *HTTPController {
	return &HTTPController{service: service, principal: principal}
}

func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /v1/me/onboarding", api.RequireSeller(http.HandlerFunc(controller.getOnboarding)))
	mux.Handle("GET /v1/me/dashboard", api.RequireSeller(http.HandlerFunc(controller.getDashboard)))
	mux.Handle("GET /v1/me/dashboard/products", api.RequireSeller(http.HandlerFunc(controller.getProducts)))
	mux.Handle("GET /v1/me/dashboard/transactions", api.RequireSeller(http.HandlerFunc(controller.getTransactions)))
	mux.Handle("GET /v1/me/dashboard/evidence", api.RequireSeller(http.HandlerFunc(controller.getEvidence)))
	mux.Handle("GET /v1/me/dashboard/webhooks", api.RequireSeller(http.HandlerFunc(controller.getWebhooks)))
	mux.Handle("GET /v1/me/dashboard/billing", api.RequireSeller(http.HandlerFunc(controller.getBilling)))
	mux.Handle("GET /v1/me/dashboard/credentials", api.RequireSeller(http.HandlerFunc(controller.getCredentials)))
	mux.Handle("GET /v1/me/settings", api.RequireSeller(http.HandlerFunc(controller.getSettings)))
	mux.Handle("PATCH /v1/me/settings", api.RequireSeller(http.HandlerFunc(controller.updateSettings)))
	mux.Handle("POST /v1/me/billing/portal-sessions", api.RequireSeller(http.HandlerFunc(controller.createPortalSession)))
}

func (controller *HTTPController) getOnboarding(response http.ResponseWriter, request *http.Request) {
	controller.writeCurrent(response, request, func(principal Principal) (any, error) {
		return controller.service.Onboarding(request.Context(), principal)
	})
}

func (controller *HTTPController) getDashboard(response http.ResponseWriter, request *http.Request) {
	controller.writeCurrent(response, request, func(principal Principal) (any, error) {
		return controller.service.Dashboard(request.Context(), principal)
	})
}

func (controller *HTTPController) getProducts(response http.ResponseWriter, request *http.Request) {
	controller.writeCurrent(response, request, func(principal Principal) (any, error) {
		return controller.service.Products(request.Context(), principal)
	})
}

func (controller *HTTPController) getTransactions(response http.ResponseWriter, request *http.Request) {
	controller.writeCurrent(response, request, func(principal Principal) (any, error) {
		return controller.service.Transactions(request.Context(), principal)
	})
}

func (controller *HTTPController) getEvidence(response http.ResponseWriter, request *http.Request) {
	controller.writeCurrent(response, request, func(principal Principal) (any, error) {
		return controller.service.Evidence(request.Context(), principal)
	})
}

func (controller *HTTPController) getWebhooks(response http.ResponseWriter, request *http.Request) {
	controller.writeCurrent(response, request, func(principal Principal) (any, error) {
		return controller.service.Webhooks(request.Context(), principal)
	})
}

func (controller *HTTPController) getBilling(response http.ResponseWriter, request *http.Request) {
	controller.writeCurrent(response, request, func(principal Principal) (any, error) {
		return controller.service.Billing(request.Context(), principal)
	})
}

func (controller *HTTPController) getCredentials(response http.ResponseWriter, request *http.Request) {
	controller.writeCurrent(response, request, func(principal Principal) (any, error) {
		return controller.service.Credentials(request.Context(), principal)
	})
}

func (controller *HTTPController) getSettings(response http.ResponseWriter, request *http.Request) {
	controller.writeCurrent(response, request, func(principal Principal) (any, error) {
		return controller.service.Settings(request.Context(), principal)
	})
}

func (controller *HTTPController) updateSettings(response http.ResponseWriter, request *http.Request) {
	principal, err := controller.principal.CurrentPrincipal(request.Context())
	if err != nil {
		writeWorkspaceError(response, request, err)
		return
	}
	var update UpdateSettingsRequest
	if err := api.DecodeJSON(response, request, &update); err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, err.Error(), nil)
		return
	}
	settings, err := controller.service.UpdateSettings(request.Context(), principal, update)
	if err != nil {
		writeWorkspaceError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(response, http.StatusOK, settings)
}

type portalSessionRequest struct {
	ReturnURL string `json:"returnUrl"`
}

func (controller *HTTPController) createPortalSession(response http.ResponseWriter, request *http.Request) {
	principal, err := controller.principal.CurrentPrincipal(request.Context())
	if err != nil {
		writeWorkspaceError(response, request, err)
		return
	}
	var body portalSessionRequest
	if err := api.DecodeJSON(response, request, &body); err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, err.Error(), nil)
		return
	}
	session, err := controller.service.CreateBillingPortalSession(request.Context(), principal, body.ReturnURL)
	if err != nil {
		writeWorkspaceError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(response, http.StatusCreated, session)
}

func (controller *HTTPController) writeCurrent(response http.ResponseWriter, request *http.Request, load func(Principal) (any, error)) {
	principal, err := controller.principal.CurrentPrincipal(request.Context())
	if err != nil {
		writeWorkspaceError(response, request, err)
		return
	}
	value, err := load(principal)
	if err != nil {
		writeWorkspaceError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(response, http.StatusOK, value)
}

func writeWorkspaceError(response http.ResponseWriter, request *http.Request, err error) {
	var validationErrors domain.ValidationErrors
	var validationError domain.ValidationError
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	message := "workspace operation failed"
	switch {
	case errors.Is(err, ErrAuthenticationRequired):
		status, code, message = http.StatusUnauthorized, api.ErrorCodeUnauthorized, err.Error()
	case errors.Is(err, ErrSellerIdentityMissing), errors.Is(err, persistence.ErrNotFound):
		status, code, message = http.StatusNotFound, api.ErrorCodeNotFound, "seller workspace was not found"
	case errors.Is(err, persistence.ErrConditionFailed):
		status, code, message = http.StatusConflict, api.ErrorCodeConflict, "workspace state changed; reload and retry"
	case errors.As(err, &validationErrors), errors.As(err, &validationError):
		status, code, message = http.StatusBadRequest, api.ErrorCodeBadRequest, err.Error()
	}
	api.WriteError(response, request, status, code, message, nil)
}
