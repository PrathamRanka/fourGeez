package storefront

import (
	"errors"
	"net/http"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/persistence"
)

type HTTPController struct{ service *Service }

func NewHTTPController(service *Service) *HTTPController { return &HTTPController{service: service} }

func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /store/{slug}/manifest.json", controller.getManifest)
	mux.HandleFunc("GET /store/{slug}/llms.txt", controller.getLLMSText)
	mux.HandleFunc("GET /v1/storefronts/{sellerSlug}/products/{productSlug}", controller.getProduct)
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
	switch {
	case errors.Is(err, persistence.ErrNotFound):
		api.WriteError(response, request, http.StatusNotFound, api.ErrorCodeNotFound, "storefront or product not found", nil)
	case errors.Is(err, ErrSellerInactive), errors.Is(err, ErrCommerceUnavailable):
		api.WriteError(response, request, http.StatusGone, api.ErrorCodeGone, "seller is inactive", nil)
	default:
		api.WriteError(response, request, http.StatusServiceUnavailable, api.ErrorCodeDependencyUnavailable, "storefront dependency unavailable", nil)
	}
}
