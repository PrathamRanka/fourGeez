package transactions

import (
	"errors"
	"net/http"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// HTTPController exposes transaction and evidence read operations.
type HTTPController struct {
	service *Service
}

// NewHTTPController creates the transaction read controller.
func NewHTTPController(service *Service) *HTTPController {
	return &HTTPController{service: service}
}

// RegisterRoutes registers transaction detail and seller-list endpoints.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(
		"GET /v1/transactions/{transactionId}",
		api.RequireAgentOrSeller(http.HandlerFunc(controller.get)),
	)
	mux.Handle(
		"GET /v1/transactions/{transactionId}/receipt",
		api.RequireAgentOrSeller(http.HandlerFunc(controller.receipt)),
	)
	mux.Handle(
		"GET /v1/sellers/{sellerId}/transactions",
		api.RequireSeller(http.HandlerFunc(controller.listSeller)),
	)
}

// get returns a transaction with its evidence verification summary.
func (controller *HTTPController) get(
	response http.ResponseWriter,
	request *http.Request,
) {
	response.Header().Set("Cache-Control", "no-store")
	transactionID, err := domain.ParseID(
		request.PathValue("transactionId"),
		domain.TransactionIDPrefix,
	)
	if err != nil {
		writeTransactionError(response, request, persistence.ErrNotFound)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	var detail DetailResponse
	if principal.Kind == api.PrincipalSeller {
		detail, err = controller.service.GetForSeller(
			request.Context(),
			transactionID,
			principal.Subject,
		)
	} else {
		detail, err = controller.service.Get(request.Context(), transactionID)
	}
	if err != nil {
		writeTransactionError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, detail)
}

// listSeller returns one authorized cursor page of seller transactions.
func (controller *HTTPController) listSeller(
	response http.ResponseWriter,
	request *http.Request,
) {
	response.Header().Set("Cache-Control", "no-store")
	sellerID, err := domain.ParseID(
		request.PathValue("sellerId"),
		domain.SellerIDPrefix,
	)
	if err != nil {
		writeTransactionError(response, request, persistence.ErrNotFound)
		return
	}
	query, err := ParseSellerTransactionQuery(sellerID, request.URL.Query(), false)
	if err != nil {
		writeTransactionError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	page, err := controller.service.ListSellerFiltered(
		request.Context(),
		principal.Subject,
		query,
	)
	if err != nil {
		writeTransactionError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, page)
}

// writeTransactionError maps read failures to stable API errors.
func writeTransactionError(
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
	case errors.Is(err, persistence.ErrNotFound),
		errors.Is(err, ErrSellerAccess),
		errors.Is(err, ErrReceiptAccess):
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	case errors.Is(err, ErrReceiptUnavailable),
		errors.Is(err, ErrReceiptEvidenceInvalid):
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
