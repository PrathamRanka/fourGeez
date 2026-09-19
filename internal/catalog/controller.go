package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// HTTPController exposes catalog use cases through the documented routes.
type HTTPController struct {
	service          *Service
	idempotencyStore domain.IdempotencyStore
}

// NewHTTPController creates the catalog HTTP controller.
func NewHTTPController(
	service *Service,
	idempotencyStore domain.IdempotencyStore,
) *HTTPController {
	return &HTTPController{
		service:          service,
		idempotencyStore: idempotencyStore,
	}
}

// RegisterRoutes registers seller and paid-route mutation endpoints.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	controller.RegisterControlRoutes(mux)
	mux.HandleFunc("GET /store/{slug}/manifest.json", controller.getStorefrontManifest)
	mux.HandleFunc("GET /store/{slug}/llms.txt", controller.getStorefrontLLMSText)
}

// RegisterControlRoutes omits legacy unsigned discovery for production wiring.
func (controller *HTTPController) RegisterControlRoutes(mux *http.ServeMux) {
	mux.Handle("POST /v1/sellers", api.RequireSeller(http.HandlerFunc(controller.createSeller)))
	mux.Handle("GET /v1/sellers/{sellerId}/routes", api.RequireSeller(http.HandlerFunc(controller.listRoutes)))
	mux.Handle("POST /v1/sellers/{sellerId}/routes", api.RequireSeller(http.HandlerFunc(controller.createRoute)))
	mux.Handle("GET /v1/sellers/{sellerId}/routes/{routeId}", api.RequireSeller(http.HandlerFunc(controller.getRoute)))
	mux.Handle("PATCH /v1/sellers/{sellerId}/routes/{routeId}", api.RequireSeller(http.HandlerFunc(controller.updateRoutePrice)))
	mux.Handle("GET /v1/sellers/{sellerId}/routes/{routeId}/validation", api.RequireSeller(http.HandlerFunc(controller.validateRoute)))
	mux.Handle("POST /v1/sellers/{sellerId}/routes/{routeId}/publish", api.RequireSeller(http.HandlerFunc(controller.publishRoute)))
	mux.Handle("POST /v1/sellers/{sellerId}/routes/{routeId}/pause", api.RequireSeller(http.HandlerFunc(controller.pauseRoute)))
	mux.Handle("POST /v1/sellers/{sellerId}/routes/{routeId}/archive", api.RequireSeller(http.HandlerFunc(controller.archiveRoute)))
	mux.Handle("POST /v1/sellers/{sellerId}/routes/{routeId}/emergency-disable", api.RequireSeller(http.HandlerFunc(controller.emergencyDisableRoute)))
}

// listRoutes returns every paid route owned by the authenticated seller.
func (controller *HTTPController) listRoutes(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, err := domain.ParseID(
		request.PathValue("sellerId"),
		domain.SellerIDPrefix,
	)
	if err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid seller ID", nil)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	routes, err := controller.service.ListSellerRoutes(
		request.Context(),
		principal.Subject,
		sellerID,
	)
	if err != nil {
		controller.writeServiceError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, routes)
}

// getRoute returns one seller-owned paid route.
func (controller *HTTPController) getRoute(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, routeID, ok := controller.parseRoutePathIDs(response, request)
	if !ok {
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	route, err := controller.service.GetSellerRoute(
		request.Context(),
		principal.Subject,
		sellerID,
		routeID,
	)
	if err != nil {
		controller.writeServiceError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, route)
}

// validateRoute returns current deterministic publication checks.
func (controller *HTTPController) validateRoute(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, routeID, ok := controller.parseRoutePathIDs(response, request)
	if !ok {
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	validation, err := controller.service.ValidateSellerRoute(
		request.Context(),
		principal.Subject,
		sellerID,
		routeID,
	)
	if err != nil {
		controller.writeServiceError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, validation)
}

// publishRoute validates and publishes or resumes a seller route.
func (controller *HTTPController) publishRoute(
	response http.ResponseWriter,
	request *http.Request,
) {
	sellerID, routeID, ok := controller.parseRoutePathIDs(response, request)
	if !ok {
		return
	}
	var input PublishRouteRequest
	controller.executeMutation(response, request, "publishPaidRoute:"+routeID.String(), &input, func(principal api.Principal) (any, error) {
		return controller.service.PublishSellerRoute(request.Context(), principal.Subject, sellerID, routeID, input)
	}, http.StatusOK)
}

// pauseRoute stops a published seller route until it is resumed.
func (controller *HTTPController) pauseRoute(
	response http.ResponseWriter,
	request *http.Request,
) {
	controller.executeRouteVersionMutation(
		response,
		request,
		"pausePaidRoute",
		controller.service.PauseSellerRoute,
	)
}

// archiveRoute permanently retires a non-published seller route.
func (controller *HTTPController) archiveRoute(
	response http.ResponseWriter,
	request *http.Request,
) {
	controller.executeRouteVersionMutation(
		response,
		request,
		"archivePaidRoute",
		controller.service.ArchiveSellerRoute,
	)
}

// emergencyDisableRoute immediately stops a published seller route.
func (controller *HTTPController) emergencyDisableRoute(
	response http.ResponseWriter,
	request *http.Request,
) {
	controller.executeRouteVersionMutation(
		response,
		request,
		"emergencyDisablePaidRoute",
		controller.service.EmergencyDisableSellerRoute,
	)
}

// executeRouteVersionMutation applies strict JSON and idempotency to one route transition.
func (controller *HTTPController) executeRouteVersionMutation(
	response http.ResponseWriter,
	request *http.Request,
	scope string,
	execute func(
		context.Context,
		string,
		domain.ID,
		domain.ID,
		RouteVersionRequest,
	) (PaidRoute, error),
) {
	sellerID, routeID, ok := controller.parseRoutePathIDs(response, request)
	if !ok {
		return
	}
	var input RouteVersionRequest
	controller.executeMutation(
		response,
		request,
		scope+":"+routeID.String(),
		&input,
		func(principal api.Principal) (any, error) {
			return execute(
				request.Context(),
				principal.Subject,
				sellerID,
				routeID,
				input,
			)
		},
		http.StatusOK,
	)
}

// parseRoutePathIDs validates the seller and route identifiers in one route URL.
func (controller *HTTPController) parseRoutePathIDs(
	response http.ResponseWriter,
	request *http.Request,
) (domain.ID, domain.ID, bool) {
	sellerID, err := domain.ParseID(
		request.PathValue("sellerId"),
		domain.SellerIDPrefix,
	)
	if err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid seller ID", nil)
		return "", "", false
	}
	routeID, err := domain.ParseID(
		request.PathValue("routeId"),
		domain.RouteIDPrefix,
	)
	if err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid route ID", nil)
		return "", "", false
	}
	return sellerID, routeID, true
}

// getStorefrontManifest returns public machine-readable seller discovery.
func (controller *HTTPController) getStorefrontManifest(
	response http.ResponseWriter,
	request *http.Request,
) {
	manifest, err := controller.service.GetStorefrontManifest(
		request.Context(),
		request.PathValue("slug"),
	)
	if err != nil {
		controller.writeServiceError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, manifest)
}

// getStorefrontLLMSText returns public plain-text agent discovery.
func (controller *HTTPController) getStorefrontLLMSText(
	response http.ResponseWriter,
	request *http.Request,
) {
	document, err := controller.service.GetStorefrontLLMSText(
		request.Context(),
		request.PathValue("slug"),
	)
	if err != nil {
		controller.writeServiceError(response, request, err)
		return
	}
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(document))
}

// createSeller validates and executes seller onboarding.
func (controller *HTTPController) createSeller(response http.ResponseWriter, request *http.Request) {
	var input CreateSellerRequest
	controller.executeMutation(response, request, "createSeller", &input, func(principal api.Principal) (any, error) {
		return controller.service.CreateSeller(request.Context(), principal.Subject, input)
	}, http.StatusCreated)
}

// createRoute validates and creates a seller-owned paid route.
func (controller *HTTPController) createRoute(response http.ResponseWriter, request *http.Request) {
	sellerID, err := domain.ParseID(request.PathValue("sellerId"), domain.SellerIDPrefix)
	if err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid seller ID", nil)
		return
	}

	var input CreateRouteRequest
	controller.executeMutation(response, request, "createPaidRoute", &input, func(principal api.Principal) (any, error) {
		return controller.service.CreateRoute(request.Context(), principal.Subject, sellerID, input)
	}, http.StatusCreated)
}

// updateRoutePrice validates and changes pricing for future intents.
func (controller *HTTPController) updateRoutePrice(response http.ResponseWriter, request *http.Request) {
	sellerID, err := domain.ParseID(request.PathValue("sellerId"), domain.SellerIDPrefix)
	if err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid seller ID", nil)
		return
	}
	routeID, err := domain.ParseID(request.PathValue("routeId"), domain.RouteIDPrefix)
	if err != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid route ID", nil)
		return
	}

	var input UpdateRoutePriceRequest
	controller.executeMutation(response, request, "updatePaidRoutePrice", &input, func(principal api.Principal) (any, error) {
		return controller.service.UpdateRoutePrice(request.Context(), principal.Subject, sellerID, routeID, input)
	}, http.StatusOK)
}

// executeMutation applies strict JSON and idempotency to a catalog mutation.
func (controller *HTTPController) executeMutation(
	response http.ResponseWriter,
	request *http.Request,
	scope string,
	input any,
	execute func(api.Principal) (any, error),
	successStatus int,
) {
	request.Body = http.MaxBytesReader(response, request.Body, api.MaximumJSONBodyBytes)
	requestBody, err := io.ReadAll(request.Body)
	if err != nil || api.DecodeJSONBytes(requestBody, input) != nil {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}

	principal, _ := api.PrincipalFromContext(request.Context())
	idempotencyScope := principal.Subject + ":" + scope
	decision, err := api.CheckIdempotency(
		request.Context(),
		controller.idempotencyStore,
		idempotencyScope,
		request.Header.Get("Idempotency-Key"),
		requestBody,
	)
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

	result, err := execute(principal)
	if err != nil {
		controller.writeServiceError(response, request, err)
		return
	}
	encodedResult, err := json.Marshal(result)
	if err != nil {
		api.WriteError(response, request, http.StatusInternalServerError, api.ErrorCodeInternal, "response encoding failed", nil)
		return
	}
	encodedResult = append(encodedResult, '\n')
	if err := api.SaveIdempotency(request.Context(), controller.idempotencyStore, idempotencyScope, decision, successStatus, encodedResult, time.Now().UTC()); err != nil {
		api.WriteError(response, request, http.StatusInternalServerError, api.ErrorCodeInternal, "idempotency persistence failed", nil)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(successStatus)
	_, _ = response.Write(encodedResult)
}

// writeServiceError maps domain and persistence errors to HTTP responses.
func (controller *HTTPController) writeServiceError(response http.ResponseWriter, request *http.Request, err error) {
	var validationErrors domain.ValidationErrors
	status := http.StatusInternalServerError
	code := api.ErrorCodeInternal
	if errors.As(err, &validationErrors) {
		status = http.StatusBadRequest
		code = api.ErrorCodeBadRequest
	} else if errors.Is(err, persistence.ErrNotFound) {
		status = http.StatusNotFound
		code = api.ErrorCodeNotFound
	} else if errors.Is(err, persistence.ErrAlreadyExists) || errors.Is(err, persistence.ErrConditionFailed) {
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	} else if errors.Is(err, ErrRouteContractStale) {
		status = http.StatusConflict
		code = api.ErrorCodeRouteContractStale
	} else if errors.Is(err, ErrRouteLifecycleTransition) || errors.Is(err, ErrRoutePublished) {
		status = http.StatusConflict
		code = api.ErrorCodeConflict
	} else if errors.Is(err, ErrRouteValidation) {
		status = http.StatusUnprocessableEntity
		code = api.ErrorCodeUnprocessable
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
