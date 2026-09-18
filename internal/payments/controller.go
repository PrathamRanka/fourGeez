package payments

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const (
	paymentRequiredHeader  = "PAYMENT-REQUIRED"
	paymentSignatureHeader = "PAYMENT-SIGNATURE"
	paymentResponseHeader  = "PAYMENT-RESPONSE"
	intentIDHeader         = "X-AgentPay-Intent-Id"
	transactionIDHeader    = "X-AgentPay-Transaction-Id"
)

// HTTPController exposes the authenticated paid-route operations.
type HTTPController struct {
	service           *CheckoutService
	browserAuthorizer *browserpurchase.RequestAuthorizer
}

// SetBrowserPurchaseAuthorizer enables browser-cookie checkout requests.
func (controller *HTTPController) SetBrowserPurchaseAuthorizer(authorizer *browserpurchase.RequestAuthorizer) {
	controller.browserAuthorizer = authorizer
}

// NewHTTPController creates the paid-route HTTP controller.
func NewHTTPController(service *CheckoutService) *HTTPController {
	return &HTTPController{service: service}
}

// RegisterRoutes registers GET and POST paid-resource operations.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	executeHandler := http.Handler(api.RequireAgent(http.HandlerFunc(controller.execute)))
	if controller.browserAuthorizer != nil {
		executeHandler = browserpurchase.RequireAgentOrBrowser(
			controller.browserAuthorizer,
			func(*http.Request) (browserpurchase.AuthorizationRequirement, error) {
				return browserpurchase.AuthorizationRequirement{Authority: browserpurchase.AuthorityCommerce, Mutation: true}, nil
			},
			http.HandlerFunc(controller.execute),
		)
	}
	mux.Handle(
		"GET /pay/{slug}/{proxyPath...}",
		executeHandler,
	)
	mux.Handle(
		"POST /pay/{slug}/{proxyPath...}",
		executeHandler,
	)
}

// execute validates transport input and writes a challenge or seller response.
func (controller *HTTPController) execute(
	response http.ResponseWriter,
	request *http.Request,
) {
	intentID, err := domain.ParseID(
		request.Header.Get(intentIDHeader),
		domain.IntentIDPrefix,
	)
	if err != nil {
		api.WriteError(
			response,
			request,
			http.StatusBadRequest,
			api.ErrorCodeBadRequest,
			"invalid purchase intent ID",
			nil,
		)
		return
	}
	request.Body = http.MaxBytesReader(
		response,
		request.Body,
		api.MaximumJSONBodyBytes,
	)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		api.WriteError(
			response,
			request,
			http.StatusBadRequest,
			api.ErrorCodeBadRequest,
			"request body is too large",
			nil,
		)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	result, err := controller.service.Execute(
		request.Context(),
		CheckoutRequest{
			PaidRouteRequest: PaidRouteRequest{
				Slug:      request.PathValue("slug"),
				Method:    catalog.RouteMethod(request.Method),
				ProxyPath: "/" + strings.TrimLeft(request.PathValue("proxyPath"), "/"),
				IntentID:  intentID,
				BuyerID:   principal.Subject,
			},
			PaymentProof: request.Header.Get(paymentSignatureHeader),
			Body:         body,
			ContentType:  request.Header.Get("Content-Type"),
		},
	)
	if err != nil {
		controller.writeError(response, request, result, err)
		return
	}
	if result.Challenge != nil {
		controller.writeChallenge(response, request, *result.Challenge)
		return
	}
	response.Header().Set(transactionIDHeader, result.TransactionID.String())
	if result.SettlementHeader != "" {
		response.Header().Set(paymentResponseHeader, result.SettlementHeader)
	}
	response.Header().Set("Content-Type", result.Response.ContentType)
	response.WriteHeader(result.Response.StatusCode)
	_, _ = response.Write(result.Response.Body)
}

// writeChallenge writes the documented x402 v2 response.
func (controller *HTTPController) writeChallenge(
	response http.ResponseWriter,
	request *http.Request,
	challenge Challenge,
) {
	response.Header().Set(paymentRequiredHeader, challenge.Header)
	response.Header().Set("Cache-Control", "no-store")
	api.WriteError(
		response,
		request,
		http.StatusPaymentRequired,
		"payment_required",
		"payment is required",
		nil,
	)
}

// writeError maps paid-route failures to stable transport errors.
func (controller *HTTPController) writeError(
	response http.ResponseWriter,
	request *http.Request,
	result CheckoutResult,
	err error,
) {
	if result.Challenge != nil && errors.Is(err, ErrPaymentRejected) {
		controller.writeChallenge(response, request, *result.Challenge)
		return
	}
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	message := err.Error()
	switch {
	case errors.Is(err, ErrIntentExpired):
		status = http.StatusGone
		code = api.ErrorCodeGone
	case errors.Is(err, domain.ErrCommerceUnavailable):
		status = http.StatusGone
		code = api.ErrorCodeGone
		message = "seller commerce is unavailable"
	case errors.Is(err, ErrPaidRouteMismatch),
		errors.Is(err, persistence.ErrNotFound):
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	case errors.Is(err, ErrPaymentReplay):
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	case errors.Is(err, ErrSubscriptionInactive):
		status = http.StatusForbidden
		code = "subscription_inactive"
		message = "seller subscription is inactive"
	case errors.Is(err, ErrCommerceAuthorizationUnavailable):
		status = http.StatusServiceUnavailable
		code = api.ErrorCodeDependencyUnavailable
		message = "commerce authorization is unavailable"
	case IsRetryable(err):
		status = http.StatusServiceUnavailable
		code = "payment_unavailable"
	}
	api.WriteError(response, request, status, code, message, nil)
}
