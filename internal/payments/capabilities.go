package payments

import (
	"net/http"

	"github.com/fourgeez/agentpay/internal/api"
)

// CapabilityHTTPController publishes the payment paths enabled by this runtime.
type CapabilityHTTPController struct {
	provider CapabilityProvider
}

// NewCapabilityHTTPController creates the public payment-capability endpoint.
func NewCapabilityHTTPController(provider CapabilityProvider) *CapabilityHTTPController {
	return &CapabilityHTTPController{provider: provider}
}

// RegisterRoutes registers the public capability catalog.
func (controller *CapabilityHTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/payment-capabilities", controller.get)
}

func (controller *CapabilityHTTPController) get(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Cache-Control", "public, max-age=60")
	_ = api.WriteJSON(response, http.StatusOK, controller.provider.PaymentCapabilities())
}
