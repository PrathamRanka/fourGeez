package billing

import (
	"errors"
	"net/http"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
)

// UsageHTTPController exposes seller invoice usage exports.
type UsageHTTPController struct {
	service *UsageService
}

// NewUsageHTTPController creates the usage export controller.
func NewUsageHTTPController(service *UsageService) *UsageHTTPController {
	return &UsageHTTPController{service: service}
}

// RegisterRoutes registers the seller invoice-export endpoint.
func (controller *UsageHTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(
		"GET /v1/sellers/{sellerId}/invoice-export",
		api.RequireSeller(http.HandlerFunc(controller.exportInvoice)),
	)
}

// exportInvoice returns one bounded non-monetary usage statement.
func (controller *UsageHTTPController) exportInvoice(response http.ResponseWriter, request *http.Request) {
	sellerID, err := domain.ParseID(
		request.PathValue("sellerId"),
		domain.SellerIDPrefix,
	)
	if err != nil {
		writeUsageError(response, request, err)
		return
	}
	from, err := time.Parse(time.RFC3339, request.URL.Query().Get("from"))
	if err != nil {
		writeUsageError(response, request, ErrUsageWindowInvalid)
		return
	}
	to, err := time.Parse(time.RFC3339, request.URL.Query().Get("to"))
	if err != nil {
		writeUsageError(response, request, ErrUsageWindowInvalid)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	export, err := controller.service.ExportInvoice(
		request.Context(),
		principal.Subject,
		sellerID,
		domain.NewTimestamp(from),
		domain.NewTimestamp(to),
	)
	if err != nil {
		writeUsageError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, export)
}

// writeUsageError maps usage export failures to stable API responses.
func writeUsageError(response http.ResponseWriter, request *http.Request, err error) {
	var validationErrors domain.ValidationErrors
	var validationError domain.ValidationError
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	switch {
	case errors.As(err, &validationErrors),
		errors.As(err, &validationError),
		errors.Is(err, ErrUsageWindowInvalid):
		status = http.StatusBadRequest
		code = api.ErrorCodeBadRequest
	case errors.Is(err, ErrUsageExportTooLarge):
		status = http.StatusUnprocessableEntity
		code = api.ErrorCodeUnprocessable
	case errors.Is(err, ErrSellerPlanNotFound):
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
