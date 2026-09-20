package notifications

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// HTTPController exposes seller webhook subscription operations.
type HTTPController struct {
	service          *Service
	idempotencyStore domain.IdempotencyStore
}

// NewHTTPController creates the webhook subscription controller.
func NewHTTPController(
	service *Service,
	idempotencyStore domain.IdempotencyStore,
) *HTTPController {
	return &HTTPController{service: service, idempotencyStore: idempotencyStore}
}

// RegisterRoutes registers seller-authenticated webhook configuration routes.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(
		"POST /v1/sellers/{sellerId}/webhook-subscriptions",
		api.RequireSeller(http.HandlerFunc(controller.create)),
	)
	mux.Handle(
		"GET /v1/sellers/{sellerId}/webhook-subscriptions",
		api.RequireSeller(http.HandlerFunc(controller.list)),
	)
	mux.Handle(
		"POST /v1/sellers/{sellerId}/webhook-subscriptions/{subscriptionId}/disable",
		api.RequireSeller(http.HandlerFunc(controller.disable)),
	)
}

// disable idempotently stops new deliveries to one subscription.
func (controller *HTTPController) disable(response http.ResponseWriter, request *http.Request) {
	sellerID, err := domain.ParseID(request.PathValue("sellerId"), domain.SellerIDPrefix)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	subscriptionID, err := domain.ParseID(request.PathValue("subscriptionId"), domain.WebhookSubscriptionIDPrefix)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, api.MaximumJSONBodyBytes)
	requestBody, err := io.ReadAll(request.Body)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	var input DisableSubscriptionRequest
	if err := api.DecodeJSONBytes(requestBody, &input); err != nil {
		writeNotificationError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	scope := principal.Subject + ":disableWebhookSubscription:" + sellerID.String() + ":" + subscriptionID.String()
	decision, err := api.CheckIdempotency(request.Context(), controller.idempotencyStore, scope, request.Header.Get("Idempotency-Key"), requestBody)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	if decision.Replay {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(decision.Status)
		_, _ = response.Write(decision.Body)
		return
	}
	disabled, err := controller.service.Disable(request.Context(), principal.Subject, sellerID, subscriptionID, input)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	encoded, err := json.Marshal(disabled)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	encoded = append(encoded, '\n')
	if err := api.SaveIdempotency(request.Context(), controller.idempotencyStore, scope, decision, http.StatusOK, encoded, time.Now().UTC()); err != nil {
		writeNotificationError(response, request, err)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(encoded)
}

// create validates a subscription and returns its signing secret once.
func (controller *HTTPController) create(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := domain.ParseID(request.PathValue("sellerId"), domain.SellerIDPrefix)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, api.MaximumJSONBodyBytes)
	requestBody, err := io.ReadAll(request.Body)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	var input CreateSubscriptionRequest
	if err := api.DecodeJSONBytes(requestBody, &input); err != nil {
		writeNotificationError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	scope := principal.Subject + ":createWebhookSubscription:" + sellerID.String()
	decision, err := api.CheckIdempotency(
		request.Context(),
		controller.idempotencyStore,
		scope,
		request.Header.Get("Idempotency-Key"),
		requestBody,
	)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	if decision.Replay {
		api.WriteError(response, request, http.StatusConflict, api.ErrorCodeConflict, "webhook signing secret cannot be replayed", nil)
		return
	}
	created, err := controller.service.Create(request.Context(), principal.Subject, sellerID, input)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	encoded, err := json.Marshal(created)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	encoded = append(encoded, '\n')
	redacted, err := json.Marshal(created.SubscriptionView)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	if err := api.SaveIdempotency(request.Context(), controller.idempotencyStore, scope, decision, http.StatusCreated, append(redacted, '\n'), time.Now().UTC()); err != nil {
		writeNotificationError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusCreated)
	_, _ = response.Write(encoded)
}

// list returns redacted subscriptions for the authenticated seller.
func (controller *HTTPController) list(response http.ResponseWriter, request *http.Request) {
	sellerID, err := domain.ParseID(request.PathValue("sellerId"), domain.SellerIDPrefix)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	items, err := controller.service.List(request.Context(), principal.Subject, sellerID)
	if err != nil {
		writeNotificationError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, map[string]any{"items": items})
}

// writeNotificationError maps subscription failures to stable API responses.
func writeNotificationError(response http.ResponseWriter, request *http.Request, err error) {
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
	case errors.Is(err, api.ErrIdempotencyConflict), errors.Is(err, persistence.ErrAlreadyExists), errors.Is(err, persistence.ErrConditionFailed):
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	case errors.Is(err, ErrSubscriptionNotFound), errors.Is(err, persistence.ErrNotFound):
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
