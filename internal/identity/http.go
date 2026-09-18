package identity

import (
	"context"
	"errors"
	"net/http"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// AgentAuthenticator is the existing independent buyer-agent credential boundary.
type AgentAuthenticator interface {
	AuthenticateAgent(context.Context, string) (api.Principal, bool)
}

// HTTPAuthenticator exposes seller identity through the shared API principal contract.
type HTTPAuthenticator struct {
	service *Service
	agents  AgentAuthenticator
}

// NewHTTPAuthenticator combines seller identity with the independent agent authenticator.
func NewHTTPAuthenticator(service *Service, agents AgentAuthenticator) *HTTPAuthenticator {
	return &HTTPAuthenticator{service: service, agents: agents}
}

// AuthenticateSeller validates a seller bearer and carries revocation context forward.
func (authenticator *HTTPAuthenticator) AuthenticateSeller(ctx context.Context, token string) (api.Principal, bool) {
	principal, err := authenticator.service.Authenticate(ctx, token)
	if err != nil {
		return api.Principal{}, false
	}
	return api.Principal{
		Kind: api.PrincipalSeller, Subject: principal.Subject, TokenID: principal.TokenID,
		SessionID: principal.SessionID, ExpiresAt: principal.ExpiresAt,
	}, true
}

// AuthenticateAgent delegates to the separately configured agent credential boundary.
func (authenticator *HTTPAuthenticator) AuthenticateAgent(ctx context.Context, key string) (api.Principal, bool) {
	if authenticator == nil || authenticator.agents == nil {
		return api.Principal{}, false
	}
	return authenticator.agents.AuthenticateAgent(ctx, key)
}

// CurrentSellerFinder resolves the one seller owned by an authenticated subject.
type CurrentSellerFinder interface {
	GetCurrentSeller(context.Context, string) (catalog.SellerResponse, error)
}

// HTTPController exposes seller-session operations without trusting caller tenant IDs.
type HTTPController struct {
	sessions *Service
	sellers  CurrentSellerFinder
}

// NewHTTPController creates the seller identity HTTP controller.
func NewHTTPController(sessions *Service, sellers CurrentSellerFinder) *HTTPController {
	return &HTTPController{sessions: sessions, sellers: sellers}
}

// RegisterRoutes registers the current-seller and session-revocation endpoints.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /v1/me/seller", api.RequireSeller(http.HandlerFunc(controller.getCurrentSeller)))
	mux.Handle("DELETE /v1/me/session", api.RequireSeller(http.HandlerFunc(controller.revokeCurrentSession)))
}

func (controller *HTTPController) getCurrentSeller(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	principal, _ := api.PrincipalFromContext(request.Context())
	seller, err := controller.sellers.GetCurrentSeller(request.Context(), principal.Subject)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			api.WriteError(response, request, http.StatusNotFound, api.ErrorCodeNotFound, "seller was not found", nil)
			return
		}
		api.WriteError(response, request, http.StatusServiceUnavailable, api.ErrorCodeDependencyUnavailable, "seller lookup is unavailable", nil)
		return
	}
	_ = api.WriteJSON(response, http.StatusOK, seller)
}

func (controller *HTTPController) revokeCurrentSession(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	principal, _ := api.PrincipalFromContext(request.Context())
	err := controller.sessions.Revoke(request.Context(), Principal{
		Subject: principal.Subject, TokenID: principal.TokenID,
		SessionID: principal.SessionID, ExpiresAt: principal.ExpiresAt,
	})
	if err != nil {
		api.WriteError(response, request, http.StatusServiceUnavailable, api.ErrorCodeDependencyUnavailable, "session revocation is unavailable", nil)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}
