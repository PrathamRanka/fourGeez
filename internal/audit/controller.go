package audit

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// HTTPController exposes seller-visible immutable audit history.
type HTTPController struct {
	service *Service
}

// NewHTTPController creates the audit history controller.
func NewHTTPController(service *Service) *HTTPController {
	return &HTTPController{service: service}
}

// RegisterRoutes registers the seller audit history endpoint.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(
		"GET /v1/sellers/{sellerId}/audit-events",
		api.RequireSeller(http.HandlerFunc(controller.list)),
	)
}

// list returns one authorized and bounded newest-first audit page.
func (controller *HTTPController) list(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := domain.ParseID(
		request.PathValue("sellerId"),
		domain.SellerIDPrefix,
	)
	if err != nil {
		writeAuditError(response, request, err)
		return
	}
	limit, err := parseAuditLimit(request.URL.Query().Get("limit"))
	if err != nil {
		writeAuditError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	page, err := controller.service.List(
		request.Context(),
		principal.Subject,
		sellerID,
		limit,
		request.URL.Query().Get("cursor"),
	)
	if err != nil {
		writeAuditError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(response, http.StatusOK, page)
}

// parseAuditLimit parses an optional bounded page size.
func parseAuditLimit(raw string) (int, error) {
	if raw == "" {
		return DefaultPageLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > MaximumPageLimit {
		return 0, domain.NewValidationError(
			"limit",
			"range",
			"must be between 1 and 100",
		)
	}
	return limit, nil
}

// writeAuditError maps audit failures to stable API responses.
func writeAuditError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	var validationError domain.ValidationError
	var validationErrors domain.ValidationErrors
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	if errors.As(err, &validationError) || errors.As(err, &validationErrors) {
		status = http.StatusBadRequest
		code = api.ErrorCodeBadRequest
	} else if errors.Is(err, persistence.ErrNotFound) {
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
