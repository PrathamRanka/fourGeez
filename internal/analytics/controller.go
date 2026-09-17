package analytics

import (
	"errors"
	"net/http"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// HTTPController exposes bounded seller dashboard summaries.
type HTTPController struct {
	service *DashboardService
}

// NewHTTPController creates the analytics HTTP controller.
func NewHTTPController(service *DashboardService) *HTTPController {
	return &HTTPController{service: service}
}

// RegisterRoutes registers the seller dashboard summary endpoint.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(
		"GET /v1/sellers/{sellerId}/dashboard-summary",
		api.RequireSeller(http.HandlerFunc(controller.summary)),
	)
}

// summary returns one bounded, asset-separated seller analytics window.
func (controller *HTTPController) summary(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := domain.ParseID(
		request.PathValue("sellerId"),
		domain.SellerIDPrefix,
	)
	if err != nil {
		writeAnalyticsError(response, request, err)
		return
	}
	query, err := transactions.ParseSellerTransactionQuery(
		sellerID,
		request.URL.Query(),
		true,
	)
	if err != nil {
		writeAnalyticsError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	summary, err := controller.service.Summary(
		request.Context(),
		principal.Subject,
		query,
	)
	if err != nil {
		writeAnalyticsError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(response, http.StatusOK, summary)
}

// writeAnalyticsError maps query and tenancy failures to stable API responses.
func writeAnalyticsError(
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
	case errors.Is(err, ErrSummaryLimit):
		status = http.StatusUnprocessableEntity
		code = api.ErrorCodeUnprocessable
	case errors.Is(err, transactions.ErrSellerAccess),
		errors.Is(err, persistence.ErrNotFound):
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
