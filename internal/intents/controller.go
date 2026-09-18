package intents

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// HTTPController exposes purchase-intent use cases through HTTP.
type HTTPController struct {
	service           *Service
	idempotencyStore  domain.IdempotencyStore
	browserAuthorizer *browserpurchase.RequestAuthorizer
}

// SetBrowserPurchaseAuthorizer enables the browser-cookie buyer channel.
func (controller *HTTPController) SetBrowserPurchaseAuthorizer(authorizer *browserpurchase.RequestAuthorizer) {
	controller.browserAuthorizer = authorizer
}

// NewHTTPController creates the purchase-intent HTTP controller.
func NewHTTPController(service *Service, idempotencyStore domain.IdempotencyStore) *HTTPController {
	return &HTTPController{service: service, idempotencyStore: idempotencyStore}
}

// RegisterRoutes registers purchase-intent endpoints.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	createHandler := http.Handler(api.RequireAgent(http.HandlerFunc(controller.create)))
	if controller.browserAuthorizer != nil {
		createHandler = browserpurchase.RequireAgentOrBrowser(
			controller.browserAuthorizer,
			func(*http.Request) (browserpurchase.AuthorizationRequirement, error) {
				return browserpurchase.AuthorizationRequirement{Authority: browserpurchase.AuthorityCommerce, Mutation: true}, nil
			},
			http.HandlerFunc(controller.create),
		)
	}
	mux.Handle("POST /v1/intents", createHandler)
	mux.Handle("GET /v1/intents/{intentId}", api.RequireAgentOrSeller(http.HandlerFunc(controller.get)))
}

// create validates and persists an immutable purchase intent.
func (controller *HTTPController) create(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, api.MaximumJSONBodyBytes)
	requestBody, err := io.ReadAll(request.Body)
	if err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}
	var input CreateIntentRequest
	if err := api.DecodeJSONBytes(requestBody, &input); err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}
	if authorization, ok := browserpurchase.AuthorizationFromContext(request.Context()); ok {
		session := authorization.PurchaseSession
		if session.RouteID != input.RouteID || session.RequestBodyHash != input.RequestBodyHash.String() || session.MaximumAmount.Compare(input.MaximumAmount) != 0 {
			api.WriteError(response, request, http.StatusConflict, api.ErrorCodeConflict, "browser purchase binding does not match the intent request", nil)
			return
		}
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	scope := principal.Subject + ":createPurchaseIntent"
	decision, err := api.CheckIdempotency(request.Context(), controller.idempotencyStore, scope, request.Header.Get("Idempotency-Key"), requestBody)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, api.ErrIdempotencyConflict) {
			status = http.StatusConflict
		}
		api.WriteError(response, request, status, api.ErrorCodeConflict, err.Error(), nil)
		return
	}
	if decision.Replay {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(decision.Status)
		_, _ = response.Write(decision.Body)
		return
	}

	purchaseIntent, err := controller.service.Create(request.Context(), principal.Subject, input)
	if err != nil {
		writeIntentError(response, request, err)
		return
	}
	encoded, err := json.Marshal(purchaseIntent.Response())
	if err != nil {
		api.WriteError(response, request, http.StatusInternalServerError, api.ErrorCodeInternal, "response encoding failed", nil)
		return
	}
	encoded = append(encoded, '\n')
	if err := api.SaveIdempotency(request.Context(), controller.idempotencyStore, scope, decision, http.StatusCreated, encoded, time.Now().UTC()); err != nil {
		api.WriteError(response, request, http.StatusInternalServerError, api.ErrorCodeInternal, "idempotency persistence failed", nil)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusCreated)
	_, _ = response.Write(encoded)
}

// get returns an immutable purchase intent.
func (controller *HTTPController) get(response http.ResponseWriter, request *http.Request) {
	intentID, err := domain.ParseID(request.PathValue("intentId"), domain.IntentIDPrefix)
	if err != nil {
		api.WriteError(response, request, http.StatusNotFound, api.ErrorCodeNotFound, "purchase intent not found", nil)
		return
	}
	purchaseIntent, err := controller.service.Get(request.Context(), intentID)
	if err != nil {
		writeIntentError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, purchaseIntent.Response())
}

// writeIntentError maps intent service errors to stable HTTP responses.
func writeIntentError(response http.ResponseWriter, request *http.Request, err error) {
	var validationErrors domain.ValidationErrors
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	if errors.As(err, &validationErrors) {
		status = http.StatusBadRequest
		code = api.ErrorCodeBadRequest
	} else if errors.Is(err, persistence.ErrNotFound) {
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	} else if errors.Is(err, persistence.ErrAlreadyExists) {
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	} else if errors.Is(err, domain.ErrCommerceUnavailable) {
		status = http.StatusGone
		code = api.ErrorCodeGone
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}

// NewPurchaseIntent validates, hashes, and freezes a purchase proposal.
func NewPurchaseIntent(params PurchaseIntentParams) (PurchaseIntent, error) {
	return createPurchaseIntent(params)
}

// ParseSHA256Digest validates a digest received at a system boundary.
func ParseSHA256Digest(raw string) (SHA256Digest, error) {
	return parseSHA256Digest(raw)
}

// HashRequestBody hashes JSON canonically and other media types byte-for-byte.
func HashRequestBody(requestBody []byte, mediaType string) (SHA256Digest, error) {
	return hashRequestBody(requestBody, mediaType)
}
