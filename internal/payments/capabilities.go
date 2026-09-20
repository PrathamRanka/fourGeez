package payments

import (
	"net/http"
	"strings"

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
	mux.HandleFunc("POST /v1/payment-capabilities/compatibility", controller.detectExternalBuyerCompatibility)
}

func (controller *CapabilityHTTPController) get(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Cache-Control", "public, max-age=60")
	_ = api.WriteJSON(response, http.StatusOK, controller.provider.PaymentCapabilities())
}

func (controller *CapabilityHTTPController) detectExternalBuyerCompatibility(response http.ResponseWriter, request *http.Request) {
	var input ExternalBuyerCompatibilityRequest
	if err := api.DecodeJSON(response, request, &input); err != nil ||
		input.SchemaVersion != ExternalBuyerCapabilitiesSchemaVersion ||
		len(input.Capabilities) < 1 || len(input.Capabilities) > 8 {
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid external buyer capability probe", nil)
		return
	}
	for _, capability := range input.Capabilities {
		if capability.Protocol != PaymentRailX402 ||
			capability.X402Version < 1 || capability.X402Version > 16 ||
			strings.TrimSpace(capability.Scheme) == "" || len(capability.Scheme) > 32 ||
			strings.TrimSpace(capability.Network) == "" || len(capability.Network) > 80 ||
			strings.TrimSpace(capability.Asset) == "" || len(capability.Asset) > 160 {
			api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid external buyer capability probe", nil)
			return
		}
	}

	response.Header().Set("Cache-Control", "no-store")
	_ = api.WriteJSON(
		response,
		http.StatusOK,
		DetectExternalBuyerCompatibility(controller.provider.PaymentCapabilities(), input),
	)
}

// DetectExternalBuyerCompatibility selects the first runtime capability also
// implemented by the external buyer. It never accepts seller credentials.
func DetectExternalBuyerCompatibility(
	catalog PaymentCapabilityCatalog,
	request ExternalBuyerCompatibilityRequest,
) ExternalBuyerCompatibilityResponse {
	result := ExternalBuyerCompatibilityResponse{
		SchemaVersion:      ExternalBuyerCompatibilitySchemaVersion,
		RuntimeEnvironment: catalog.Environment,
		ReasonCode:         ExternalBuyerNoCompatibleCapabilityReason,
		Authentication: BuyerAgentAuthentication{
			CredentialType:           BuyerAgentCredentialType,
			HeaderName:               BuyerAgentCredentialHeader,
			SellerProjectKeyAccepted: false,
		},
		Headers: ExternalBuyerHTTPHeaders{
			IntentID:         intentIDHeader,
			PaymentRequired:  paymentRequiredHeader,
			PaymentSignature: paymentSignatureHeader,
			PaymentResponse:  paymentResponseHeader,
			TransactionID:    transactionIDHeader,
		},
	}
	for capabilityIndex := range catalog.Capabilities {
		runtimeCapability := &catalog.Capabilities[capabilityIndex]
		if runtimeCapability.Rail != PaymentRailX402 || !containsPaymentChannel(runtimeCapability.Channels, PaymentChannelAgent) {
			continue
		}
		for _, buyerCapability := range request.Capabilities {
			if buyerCapability.Protocol == runtimeCapability.Rail &&
				buyerCapability.X402Version == X402ProtocolVersion &&
				buyerCapability.Scheme == runtimeCapability.Scheme &&
				buyerCapability.Network == runtimeCapability.Network &&
				buyerCapability.Asset == runtimeCapability.Asset.Identifier {
				selected := *runtimeCapability
				result.Compatible = true
				result.ReasonCode = ExternalBuyerCompatibleReason
				result.SelectedCapability = &selected
				return result
			}
		}
	}
	return result
}

func containsPaymentChannel(channels []string, expected string) bool {
	for _, channel := range channels {
		if channel == expected {
			return true
		}
	}
	return false
}
