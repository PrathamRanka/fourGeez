package settlement

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

// HTTPController exposes seller-scoped payment-destination operations.
type HTTPController struct {
	service          *Service
	idempotencyStore domain.IdempotencyStore
}

// NewHTTPController creates the payment-destination HTTP controller.
func NewHTTPController(
	service *Service,
	idempotencyStore domain.IdempotencyStore,
) *HTTPController {
	return &HTTPController{
		service:          service,
		idempotencyStore: idempotencyStore,
	}
}

// RegisterRoutes registers destination lifecycle and ownership-proof routes.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(
		"POST /v1/sellers/{sellerId}/payment-destinations",
		api.RequireSeller(http.HandlerFunc(controller.create)),
	)
	mux.Handle(
		"GET /v1/sellers/{sellerId}/payment-destinations",
		api.RequireSeller(http.HandlerFunc(controller.list)),
	)
	mux.Handle(
		"GET /v1/sellers/{sellerId}/payment-destinations/{destinationId}",
		api.RequireSeller(http.HandlerFunc(controller.get)),
	)
	mux.Handle(
		"POST /v1/sellers/{sellerId}/payment-destinations/{destinationId}/ownership-challenges",
		api.RequireSeller(http.HandlerFunc(controller.issueOwnershipChallenge)),
	)
	mux.Handle(
		"POST /v1/sellers/{sellerId}/payment-destinations/{destinationId}/verify",
		api.RequireSeller(http.HandlerFunc(controller.verifyOwnership)),
	)
}

// create validates and stores a pending seller payment destination.
func (controller *HTTPController) create(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := parseSellerID(request)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, api.MaximumJSONBodyBytes)
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
	var input CreatePaymentDestinationRequest
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
	idempotencyScope := principal.Subject + ":createPaymentDestination:" + sellerID.String()
	decision, err := api.CheckIdempotency(
		request.Context(),
		controller.idempotencyStore,
		idempotencyScope,
		request.Header.Get("Idempotency-Key"),
		requestBody,
	)
	if err != nil {
		writeIdempotencyError(response, request, err)
		return
	}
	if decision.Replay {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(decision.Status)
		_, _ = response.Write(decision.Body)
		return
	}
	destination, err := controller.service.Create(
		request.Context(),
		principal.Subject,
		sellerID,
		input,
	)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	responseBody, err := json.Marshal(destination)
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
	responseBody = append(responseBody, '\n')
	if err := api.SaveIdempotency(
		request.Context(),
		controller.idempotencyStore,
		idempotencyScope,
		decision,
		http.StatusCreated,
		responseBody,
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
	_, _ = response.Write(responseBody)
}

// list returns every payment destination owned by the authenticated seller.
func (controller *HTTPController) list(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := parseSellerID(request)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	destinations, err := controller.service.List(
		request.Context(),
		principal.Subject,
		sellerID,
	)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	_ = api.WriteJSON(
		response,
		http.StatusOK,
		PaymentDestinationListResponse{Items: destinations},
	)
}

// get returns one seller-owned payment destination.
func (controller *HTTPController) get(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := parseSellerID(request)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	destinationID, err := domain.ParseID(
		request.PathValue("destinationId"),
		domain.PaymentDestinationIDPrefix,
	)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	destination, err := controller.service.Get(
		request.Context(),
		principal.Subject,
		sellerID,
		destinationID,
	)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, destination)
}

// issueOwnershipChallenge returns one short-lived challenge with no cacheable secret copy.
func (controller *HTTPController) issueOwnershipChallenge(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, destinationID, err := parseDestinationPath(request)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	challenge, err := controller.service.IssueOwnershipChallenge(
		request.Context(),
		principal.Subject,
		sellerID,
		destinationID,
	)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(response, http.StatusCreated, challenge)
}

// verifyOwnership validates one proof and returns the activated destination.
func (controller *HTTPController) verifyOwnership(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, destinationID, err := parseDestinationPath(request)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, api.MaximumJSONBodyBytes)
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
	var input VerifyOwnershipRequest
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
	idempotencyScope := principal.Subject + ":verifyPaymentDestination:" +
		destinationID.String()
	decision, err := api.CheckIdempotency(
		request.Context(),
		controller.idempotencyStore,
		idempotencyScope,
		request.Header.Get("Idempotency-Key"),
		requestBody,
	)
	if err != nil {
		writeIdempotencyError(response, request, err)
		return
	}
	if decision.Replay {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(decision.Status)
		_, _ = response.Write(decision.Body)
		return
	}
	destination, err := controller.service.VerifyOwnership(
		request.Context(),
		principal.Subject,
		sellerID,
		destinationID,
		input,
	)
	if err != nil {
		writeSettlementError(response, request, err)
		return
	}
	responseBody, err := json.Marshal(destination)
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
	responseBody = append(responseBody, '\n')
	if err := api.SaveIdempotency(
		request.Context(),
		controller.idempotencyStore,
		idempotencyScope,
		decision,
		http.StatusOK,
		responseBody,
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
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(responseBody)
}

// parseSellerID validates the seller identifier path parameter.
func parseSellerID(request *http.Request) (domain.ID, error) {
	return domain.ParseID(request.PathValue("sellerId"), domain.SellerIDPrefix)
}

// parseDestinationPath validates the seller and destination path identifiers.
func parseDestinationPath(request *http.Request) (domain.ID, domain.ID, error) {
	sellerID, err := parseSellerID(request)
	if err != nil {
		return "", "", err
	}
	destinationID, err := domain.ParseID(
		request.PathValue("destinationId"),
		domain.PaymentDestinationIDPrefix,
	)
	if err != nil {
		return "", "", err
	}
	return sellerID, destinationID, nil
}

// writeIdempotencyError maps replay-protection failures to stable responses.
func writeIdempotencyError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	status := http.StatusBadRequest
	if errors.Is(err, api.ErrIdempotencyConflict) {
		status = http.StatusConflict
	}
	api.WriteError(response, request, status, api.ErrorCodeConflict, err.Error(), nil)
}

// writeSettlementError maps domain and persistence failures to HTTP responses.
func writeSettlementError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	var validationErrors domain.ValidationErrors
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	if errors.As(err, &validationErrors) {
		status = http.StatusBadRequest
		code = api.ErrorCodeBadRequest
	} else if errors.Is(err, ErrOwnershipChallengeExpired) {
		status = http.StatusGone
		code = api.ErrorCodeGone
	} else if errors.Is(err, ErrOwnershipChallengeMismatch) ||
		errors.Is(err, ErrOwnershipProofInvalid) ||
		errors.Is(err, ErrOwnershipNetworkUnsupported) {
		status = http.StatusUnprocessableEntity
		code = api.ErrorCodeUnprocessable
	} else if errors.Is(err, ErrOwnershipStateInvalid) ||
		errors.Is(err, ErrRotationConfirmationRequired) ||
		errors.Is(err, persistence.ErrConditionFailed) {
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	} else if errors.Is(err, persistence.ErrNotFound) {
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	} else if errors.Is(err, persistence.ErrAlreadyExists) {
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
