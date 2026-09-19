package storefront

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

type HTTPController struct{ service *Service }

func NewHTTPController(service *Service) *HTTPController { return &HTTPController{service: service} }

func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/agentpay", controller.getPlatformManifest)
	mux.HandleFunc("GET /v1/discovery/products", controller.listPublicProducts)
	mux.HandleFunc("GET /store/{slug}/manifest.json", controller.getManifest)
	mux.HandleFunc("GET /store/{slug}/llms.txt", controller.getLLMSText)
	mux.HandleFunc("GET /v1/storefronts/{sellerSlug}/products/{productSlug}", controller.getProduct)
}

func (controller *HTTPController) getPlatformManifest(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Cache-Control", "public, max-age=300")
	_ = api.WriteJSON(response, http.StatusOK, controller.service.GetPlatformManifest())
}

func (controller *HTTPController) listPublicProducts(response http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	allowed := map[string]bool{"q": true, "asset": true, "network": true, "limit": true, "cursor": true}
	for key, values := range query {
		if !allowed[key] || len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid directory query", nil)
			return
		}
	}
	limit := defaultDirectoryLimit
	if rawLimit := query.Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid directory limit", nil)
			return
		}
		limit = parsed
	}
	page, err := controller.service.ListPublicProducts(request.Context(), PublicDirectoryRequest{
		Query: query.Get("q"), Asset: query.Get("asset"), Network: query.Get("network"), Limit: limit, Cursor: query.Get("cursor"),
	})
	if err != nil {
		writePublicError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "public, max-age=30")
	_ = api.WriteJSON(response, http.StatusOK, page)
}

func (controller *HTTPController) getManifest(response http.ResponseWriter, request *http.Request) {
	result, err := controller.service.GetManifest(request.Context(), request.PathValue("slug"))
	response.Header().Set("Cache-Control", "public, max-age=60")
	if errors.Is(err, ErrSellerInactive) && result.Tombstone != nil {
		_ = api.WriteJSON(response, http.StatusGone, result.Tombstone)
		return
	}
	if err != nil {
		writePublicError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, result)
}

func (controller *HTTPController) getProduct(response http.ResponseWriter, request *http.Request) {
	document, err := controller.service.GetProduct(request.Context(), request.PathValue("sellerSlug"), request.PathValue("productSlug"))
	response.Header().Set("Cache-Control", "public, max-age=60")
	if err != nil {
		writePublicError(response, request, err)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, document)
}

func (controller *HTTPController) getLLMSText(response http.ResponseWriter, request *http.Request) {
	document, err := controller.service.GetLLMSText(request.Context(), request.PathValue("slug"))
	if err != nil {
		writePublicError(response, request, err)
		return
	}
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("Cache-Control", "public, max-age=60")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(document))
}

func writePublicError(response http.ResponseWriter, request *http.Request, err error) {
	var validationErrors domain.ValidationErrors
	var validationError domain.ValidationError
	switch {
	case errors.As(err, &validationErrors), errors.As(err, &validationError):
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid directory query", nil)
	case errors.Is(err, persistence.ErrNotFound):
		api.WriteError(response, request, http.StatusNotFound, api.ErrorCodeNotFound, "storefront or product not found", nil)
	case errors.Is(err, ErrSellerInactive), errors.Is(err, ErrCommerceUnavailable):
		api.WriteError(response, request, http.StatusGone, api.ErrorCodeGone, "seller is inactive", nil)
	default:
		api.WriteError(response, request, http.StatusServiceUnavailable, api.ErrorCodeDependencyUnavailable, "storefront dependency unavailable", nil)
	}
}
