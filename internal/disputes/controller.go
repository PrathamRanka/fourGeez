package disputes

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
	"github.com/fourgeez/agentpay/internal/transactions"
)

// HTTPController exposes dispute creation and retrieval operations.
type HTTPController struct {
	service           *Service
	idempotencyStore  domain.IdempotencyStore
	browserAuthorizer *browserpurchase.RequestAuthorizer
}

// SetBrowserPurchaseAuthorizer enables cookie-bound dispute access.
func (controller *HTTPController) SetBrowserPurchaseAuthorizer(authorizer *browserpurchase.RequestAuthorizer) {
	controller.browserAuthorizer = authorizer
}

// NewHTTPController creates the dispute HTTP controller.
func NewHTTPController(
	service *Service,
	idempotencyStore domain.IdempotencyStore,
) *HTTPController {
	return &HTTPController{
		service:          service,
		idempotencyStore: idempotencyStore,
	}
}

// RegisterRoutes registers the documented dispute endpoints.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	createHandler := http.Handler(api.RequireAgentOrSeller(http.HandlerFunc(controller.create)))
	getHandler := http.Handler(api.RequireAgentOrSeller(http.HandlerFunc(controller.get)))
	if controller.browserAuthorizer != nil {
		createHandler = browserpurchase.RequireAgentSellerOrBrowser(
			controller.browserAuthorizer,
			func(*http.Request) (browserpurchase.AuthorizationRequirement, error) {
				return browserpurchase.AuthorizationRequirement{Authority: browserpurchase.AuthorityRemediation, Mutation: true}, nil
			},
			http.HandlerFunc(controller.create),
		)
		getHandler = browserpurchase.RequireAgentSellerOrBrowser(
			controller.browserAuthorizer,
			func(*http.Request) (browserpurchase.AuthorizationRequirement, error) {
				return browserpurchase.AuthorizationRequirement{Authority: browserpurchase.AuthorityRemediation}, nil
			},
			http.HandlerFunc(controller.get),
		)
	}
	mux.Handle(
		"POST /v1/disputes",
		createHandler,
	)
	mux.Handle(
		"GET /v1/disputes/{disputeId}",
		getHandler,
	)
}

// create opens and classifies one idempotent dispute request.
func (controller *HTTPController) create(
	response http.ResponseWriter,
	request *http.Request,
) {
	response.Header().Set("Cache-Control", "no-store")
	request.Body = http.MaxBytesReader(
		response,
		request.Body,
		api.MaximumJSONBodyBytes,
	)
	requestBody, err := io.ReadAll(request.Body)
	if err != nil {
		api.WriteError(
			response,
			request,
			http.StatusBadRequest,
			api.ErrorCodeBadRequest,
			"invalid request body",
			nil,
		)
		return
	}
	var input CreateRequest
	if err := api.DecodeJSONBytes(requestBody, &input); err != nil {
		api.WriteError(
			response,
			request,
			http.StatusBadRequest,
			api.ErrorCodeBadRequest,
			"invalid request body",
			nil,
		)
		return
	}

	principal, _ := api.PrincipalFromContext(request.Context())
	if principal.Kind == api.PrincipalBrowser {
		authorization, ok := browserpurchase.AuthorizationFromContext(request.Context())
		if !ok || authorization.PurchaseSession.TransactionID == nil ||
			*authorization.PurchaseSession.TransactionID != input.TransactionID {
			writeDisputeError(response, request, persistence.ErrNotFound)
			return
		}
	}
	scope := principal.Subject + ":createDispute"
	decision, err := api.CheckIdempotency(
		request.Context(),
		controller.idempotencyStore,
		scope,
		request.Header.Get("Idempotency-Key"),
		requestBody,
	)
	if err != nil {
		writeDisputeIdempotencyError(response, request, err)
		return
	}
	if decision.Replay {
		writeDisputeReplay(response, decision)
		return
	}

	sellerSubject := ""
	if principal.Kind == api.PrincipalSeller {
		sellerSubject = principal.Subject
	}
	dispute, err := controller.service.Create(
		request.Context(),
		input,
		sellerSubject,
	)
	if err != nil {
		writeDisputeError(response, request, err)
		return
	}
	controller.writeMutation(
		response,
		request,
		scope,
		decision,
		dispute,
	)
}

// get returns one dispute after applying seller tenant authorization.
func (controller *HTTPController) get(
	response http.ResponseWriter,
	request *http.Request,
) {
	response.Header().Set("Cache-Control", "no-store")
	disputeID, err := domain.ParseID(
		request.PathValue("disputeId"),
		domain.DisputeIDPrefix,
	)
	if err != nil {
		writeDisputeError(response, request, persistence.ErrNotFound)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	var dispute Dispute
	if principal.Kind == api.PrincipalSeller {
		dispute, err = controller.service.GetForSeller(
			request.Context(),
			disputeID,
			principal.Subject,
		)
	} else {
		dispute, err = controller.service.Get(request.Context(), disputeID)
	}
	if err != nil {
		writeDisputeError(response, request, err)
		return
	}
	if principal.Kind == api.PrincipalBrowser {
		authorization, ok := browserpurchase.AuthorizationFromContext(request.Context())
		if !ok || authorization.PurchaseSession.TransactionID == nil ||
			*authorization.PurchaseSession.TransactionID != dispute.TransactionID {
			writeDisputeError(response, request, persistence.ErrNotFound)
			return
		}
	}
	_ = api.WriteJSON(response, http.StatusOK, dispute)
}

// writeMutation stores and returns a successful idempotent dispute response.
func (controller *HTTPController) writeMutation(
	response http.ResponseWriter,
	request *http.Request,
	scope string,
	decision api.IdempotencyDecision,
	dispute Dispute,
) {
	encoded, err := json.Marshal(dispute)
	if err != nil {
		api.WriteError(
			response,
			request,
			http.StatusInternalServerError,
			api.ErrorCodeInternal,
			"response encoding failed",
			nil,
		)
		return
	}
	encoded = append(encoded, '\n')
	if err := api.SaveIdempotency(
		request.Context(),
		controller.idempotencyStore,
		scope,
		decision,
		http.StatusCreated,
		encoded,
		time.Now().UTC(),
	); err != nil {
		api.WriteError(
			response,
			request,
			http.StatusInternalServerError,
			api.ErrorCodeInternal,
			"idempotency persistence failed",
			nil,
		)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusCreated)
	_, _ = response.Write(encoded)
}

// writeDisputeReplay writes a stored dispute mutation response.
func writeDisputeReplay(
	response http.ResponseWriter,
	decision api.IdempotencyDecision,
) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(decision.Status)
	_, _ = response.Write(decision.Body)
}

// writeDisputeIdempotencyError maps mutation-key failures.
func writeDisputeIdempotencyError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	status := http.StatusBadRequest
	code := api.ErrorCodeBadRequest
	if errors.Is(err, api.ErrIdempotencyConflict) {
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}

// writeDisputeError maps domain and persistence failures to stable responses.
func writeDisputeError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	var validationErrors domain.ValidationErrors
	var validationError domain.ValidationError
	var invalidTransition transactions.InvalidTransitionError
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	switch {
	case errors.As(err, &validationErrors), errors.As(err, &validationError):
		status = http.StatusBadRequest
		code = api.ErrorCodeBadRequest
	case errors.Is(err, persistence.ErrNotFound),
		errors.Is(err, ErrDisputeAccess):
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	case errors.As(err, &invalidTransition),
		errors.Is(err, persistence.ErrAlreadyExists),
		errors.Is(err, persistence.ErrConditionFailed):
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
