package billing

import (
	"errors"
	"net/http"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// HTTPController exposes the plan catalog and seller assignment reads.
type HTTPController struct {
	service *Service
}

// NewHTTPController creates the seller billing controller.
func NewHTTPController(service *Service) *HTTPController {
	return &HTTPController{service: service}
}

// RegisterRoutes registers plan catalog and seller assignment routes.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/plans", controller.listPlans)
	mux.Handle(
		"GET /v1/sellers/{sellerId}/plan",
		api.RequireSeller(http.HandlerFunc(controller.getSellerPlan)),
	)
}

// listPlans returns the immutable V1 plan catalog.
func (controller *HTTPController) listPlans(
	response http.ResponseWriter,
	_ *http.Request,
) {
	_ = api.WriteJSON(
		response,
		http.StatusOK,
		map[string]any{"items": V1PlanCatalog()},
	)
}

// getSellerPlan returns the authenticated seller's current assignment.
func (controller *HTTPController) getSellerPlan(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := domain.ParseID(
		request.PathValue("sellerId"),
		domain.SellerIDPrefix,
	)
	if err != nil {
		writeBillingError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	assignment, err := controller.service.GetSellerPlan(
		request.Context(),
		principal.Subject,
		sellerID,
	)
	if err != nil {
		writeBillingError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, assignment)
}

// writeBillingError maps billing failures to stable API responses.
func writeBillingError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	var validationErrors domain.ValidationErrors
	var validationError domain.ValidationError
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	switch {
	case errors.As(err, &validationErrors), errors.As(err, &validationError):
		status = http.StatusBadRequest
		code = api.ErrorCodeBadRequest
	case errors.Is(err, ErrSellerPlanConflict),
		errors.Is(err, persistence.ErrConditionFailed):
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	case errors.Is(err, ErrPlanNotFound),
		errors.Is(err, ErrSellerPlanNotFound),
		errors.Is(err, persistence.ErrNotFound):
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
