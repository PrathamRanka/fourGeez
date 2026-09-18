package notifications

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// DeliveryHTTPController exposes seller webhook delivery operations.
type DeliveryHTTPController struct {
	service          *DeliveryService
	idempotencyStore domain.IdempotencyStore
}

// NewDeliveryHTTPController creates the webhook delivery controller.
func NewDeliveryHTTPController(
	service *DeliveryService,
	idempotencyStore domain.IdempotencyStore,
) *DeliveryHTTPController {
	return &DeliveryHTTPController{
		service:          service,
		idempotencyStore: idempotencyStore,
	}
}

// RegisterRoutes registers seller-authenticated delivery history routes.
func (controller *DeliveryHTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(
		"GET /v1/sellers/{sellerId}/webhook-deliveries",
		api.RequireSeller(http.HandlerFunc(controller.list)),
	)
	mux.Handle(
		"POST /v1/sellers/{sellerId}/webhook-deliveries/{deliveryId}/redeliver",
		api.RequireSeller(http.HandlerFunc(controller.redeliver)),
	)
}

// list returns one bounded seller-visible delivery history page.
func (controller *DeliveryHTTPController) list(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := domain.ParseID(
		request.PathValue("sellerId"),
		domain.SellerIDPrefix,
	)
	if err != nil {
		writeDeliveryError(response, request, err)
		return
	}
	limit, err := parseDeliveryLimit(request.URL.Query().Get("limit"))
	if err != nil {
		writeDeliveryError(response, request, err)
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
		writeDeliveryError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, page)
}

// redeliver idempotently schedules one dead-letter delivery for immediate retry.
func (controller *DeliveryHTTPController) redeliver(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, deliveryID, err := parseDeliveryPath(request)
	if err != nil {
		writeDeliveryError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	scope := principal.Subject + ":redeliverWebhook:" + deliveryID.String()
	decision, err := api.CheckIdempotency(
		request.Context(),
		controller.idempotencyStore,
		scope,
		request.Header.Get("Idempotency-Key"),
		[]byte(deliveryID.String()),
	)
	if err != nil {
		writeDeliveryError(response, request, err)
		return
	}
	if decision.Replay {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(decision.Status)
		_, _ = response.Write(decision.Body)
		return
	}
	view, err := controller.service.Redeliver(
		request.Context(),
		principal.Subject,
		sellerID,
		deliveryID,
	)
	if err != nil {
		writeDeliveryError(response, request, err)
		return
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		writeDeliveryError(response, request, err)
		return
	}
	encoded = append(encoded, '\n')
	if err := api.SaveIdempotency(
		request.Context(),
		controller.idempotencyStore,
		scope,
		decision,
		http.StatusOK,
		encoded,
		time.Now().UTC(),
	); err != nil {
		writeDeliveryError(response, request, err)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(encoded)
}

// parseDeliveryPath validates seller and delivery identifiers.
func parseDeliveryPath(request *http.Request) (domain.ID, domain.ID, error) {
	sellerID, err := domain.ParseID(
		request.PathValue("sellerId"),
		domain.SellerIDPrefix,
	)
	if err != nil {
		return "", "", err
	}
	deliveryID, err := domain.ParseID(
		request.PathValue("deliveryId"),
		domain.WebhookDeliveryIDPrefix,
	)
	return sellerID, deliveryID, err
}

// parseDeliveryLimit validates an optional bounded page size.
func parseDeliveryLimit(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, domain.NewValidationError("limit", "format", "must be an integer")
	}
	return limit, nil
}

// writeDeliveryError maps delivery failures to stable API responses.
func writeDeliveryError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	var validationErrors domain.ValidationErrors
	var validationError domain.ValidationError
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	switch {
	case errors.Is(err, domain.ErrPermissionDenied):
		status = http.StatusForbidden
		code = api.ErrorCodePermissionDenied
	case errors.Is(err, domain.ErrRateLimitExceeded):
		status = http.StatusTooManyRequests
		code = api.ErrorCodeRateLimited
	case errors.As(err, &validationErrors), errors.As(err, &validationError):
		status = http.StatusBadRequest
		code = api.ErrorCodeBadRequest
	case errors.Is(err, api.ErrIdempotencyConflict),
		errors.Is(err, ErrDeliveryConflict),
		errors.Is(err, persistence.ErrConditionFailed),
		errors.Is(err, persistence.ErrAlreadyExists):
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	case errors.Is(err, ErrDeliveryNotFound),
		errors.Is(err, ErrSubscriptionNotFound),
		errors.Is(err, persistence.ErrNotFound):
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
