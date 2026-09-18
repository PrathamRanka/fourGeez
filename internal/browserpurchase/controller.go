package browserpurchase

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const (
	ProductionPurchaseCookieName = "__Host-agentpay_purchase"
	ProductionCSRFCookieName     = "__Host-agentpay_purchase_csrf"
	LocalPurchaseCookieName      = "agentpay_purchase"
	LocalCSRFCookieName          = "agentpay_purchase_csrf"
	CSRFHeaderName               = "X-AgentPay-CSRF"
)

type CookiePolicy struct {
	AllowedOrigin string
	Secure        bool
}

func (policy CookiePolicy) purchaseCookieName() string {
	if policy.Secure {
		return ProductionPurchaseCookieName
	}
	return LocalPurchaseCookieName
}

func (policy CookiePolicy) csrfCookieName() string {
	if policy.Secure {
		return ProductionCSRFCookieName
	}
	return LocalCSRFCookieName
}

type HTTPController struct {
	service      *Service
	cookiePolicy CookiePolicy
}

func NewHTTPController(service *Service, cookiePolicy CookiePolicy) *HTTPController {
	return &HTTPController{service: service, cookiePolicy: cookiePolicy}
}

func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/storefronts/{sellerSlug}/products/{productSlug}/purchase-sessions", controller.create)
	mux.HandleFunc("POST /v1/browser-purchases/{purchaseSessionId}/recovery-challenges", controller.createRecoveryChallenge)
	mux.HandleFunc("POST /v1/browser-purchases/{purchaseSessionId}/recover", controller.recover)
}

func (controller *HTTPController) create(response http.ResponseWriter, request *http.Request) {
	var input CreateBrowserPurchaseSessionRequest
	if err := api.DecodeJSON(response, request, &input); err != nil {
		setNoStore(response)
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}
	created, err := controller.service.Create(request.Context(), request.PathValue("sellerSlug"), request.PathValue("productSlug"), request.Header.Get("Idempotency-Key"), input)
	if err != nil {
		writeError(response, request, err)
		return
	}
	if !created.Replay {
		controller.setCookies(response, created.BrowserGrant, created.CSRFToken, created.Session.AccessExpiresAt.Time())
	}
	setNoStore(response)
	_ = api.WriteJSON(response, http.StatusCreated, created.Response())
}

func (controller *HTTPController) createRecoveryChallenge(response http.ResponseWriter, request *http.Request) {
	purchaseSessionID, err := ParsePurchaseSessionID(request.PathValue("purchaseSessionId"))
	if err != nil {
		writeError(response, request, persistence.ErrNotFound)
		return
	}
	var input BrowserPurchaseRecoveryChallengeRequest
	if err := api.DecodeJSON(response, request, &input); err != nil {
		setNoStore(response)
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}
	challenge, err := controller.service.CreateRecoveryChallenge(request.Context(), purchaseSessionID, input)
	if err != nil {
		writeError(response, request, err)
		return
	}
	setNoStore(response)
	_ = api.WriteJSON(response, http.StatusCreated, challenge)
}

func (controller *HTTPController) recover(response http.ResponseWriter, request *http.Request) {
	purchaseSessionID, err := ParsePurchaseSessionID(request.PathValue("purchaseSessionId"))
	if err != nil {
		writeError(response, request, persistence.ErrNotFound)
		return
	}
	var input RecoverBrowserPurchaseRequest
	if err := api.DecodeJSON(response, request, &input); err != nil {
		setNoStore(response)
		api.WriteError(response, request, http.StatusBadRequest, api.ErrorCodeBadRequest, "invalid request body", nil)
		return
	}
	recovered, err := controller.service.Recover(request.Context(), purchaseSessionID, input)
	if err != nil {
		writeError(response, request, err)
		return
	}
	controller.setCookies(response, recovered.BrowserGrant, recovered.CSRFToken, recovered.Session.AccessExpiresAt.Time())
	setNoStore(response)
	response.WriteHeader(http.StatusNoContent)
}

func (controller *HTTPController) setCookies(response http.ResponseWriter, browserGrant, csrfToken string, expiresAt time.Time) {
	http.SetCookie(response, &http.Cookie{Name: controller.cookiePolicy.purchaseCookieName(), Value: browserGrant, Path: "/", Expires: expiresAt, Secure: controller.cookiePolicy.Secure, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.SetCookie(response, &http.Cookie{Name: controller.cookiePolicy.csrfCookieName(), Value: csrfToken, Path: "/", Expires: expiresAt, Secure: controller.cookiePolicy.Secure, HttpOnly: false, SameSite: http.SameSiteStrictMode})
}

type RequestAuthorizer struct {
	service      *Service
	cookiePolicy CookiePolicy
}

func NewRequestAuthorizer(service *Service, cookiePolicy CookiePolicy) *RequestAuthorizer {
	return &RequestAuthorizer{service: service, cookiePolicy: cookiePolicy}
}

func (authorizer *RequestAuthorizer) Authorize(request *http.Request, requirement AuthorizationRequirement) (Authorization, error) {
	purchaseCookie, err := request.Cookie(authorizer.cookiePolicy.purchaseCookieName())
	if err != nil {
		return Authorization{}, ErrInvalidGrant
	}
	authorization, err := authorizer.service.Authorize(request.Context(), purchaseCookie.Value, requirement)
	if err != nil {
		return Authorization{}, err
	}
	if !requirement.Mutation {
		return authorization, nil
	}
	fetchSite := request.Header.Get("Sec-Fetch-Site")
	if request.Header.Get("Origin") != authorizer.cookiePolicy.AllowedOrigin || fetchSite != "same-origin" && fetchSite != "same-site" {
		return Authorization{}, ErrOriginDenied
	}
	csrfCookie, err := request.Cookie(authorizer.cookiePolicy.csrfCookieName())
	if err != nil {
		return Authorization{}, ErrCSRFInvalid
	}
	headerToken := request.Header.Get(CSRFHeaderName)
	if len(headerToken) != len(csrfCookie.Value) || len(headerToken) < 32 || subtle.ConstantTimeCompare([]byte(headerToken), []byte(csrfCookie.Value)) != 1 {
		return Authorization{}, ErrCSRFInvalid
	}
	if err := authorizer.service.ValidateCSRF(authorization.PurchaseSession, headerToken); err != nil {
		return Authorization{}, err
	}
	return authorization, nil
}

type authorizationContextKey struct{}

func AuthorizationFromContext(ctx context.Context) (Authorization, bool) {
	authorization, ok := ctx.Value(authorizationContextKey{}).(Authorization)
	return authorization, ok
}

// RequireAgentOrBrowser accepts the independent buyer-agent key or a browser purchase grant.
func RequireAgentOrBrowser(authorizer *RequestAuthorizer, requirement func(*http.Request) (AuthorizationRequirement, error), next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if principal, valid := api.AuthenticateAgentRequest(request.Context(), request.Header.Get(api.AgentKeyHeader)); valid {
			next.ServeHTTP(response, api.WithPrincipal(request, principal))
			return
		}
		if authorizer == nil {
			api.WriteError(response, request, http.StatusUnauthorized, api.ErrorCodeUnauthorized, "buyer authentication is required", nil)
			return
		}
		authorizer.Middleware(requirement, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			authorization, _ := AuthorizationFromContext(request.Context())
			principal := api.Principal{
				Kind: api.PrincipalBrowser, Subject: authorization.BuyerID,
				SessionID: authorization.PurchaseSession.PurchaseSessionID.String(),
				ExpiresAt: authorization.PurchaseSession.AccessExpiresAt.Time(),
			}
			next.ServeHTTP(response, api.WithPrincipal(request, principal))
		})).ServeHTTP(response, request)
	})
}

// RequireAgentSellerOrBrowser accepts each documented purchase-read caller.
func RequireAgentSellerOrBrowser(authorizer *RequestAuthorizer, requirement func(*http.Request) (AuthorizationRequirement, error), next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if principal, valid := api.PrincipalFromContext(request.Context()); valid &&
			(principal.Kind == api.PrincipalAgent || principal.Kind == api.PrincipalSeller) {
			next.ServeHTTP(response, request)
			return
		}
		if principal, valid := api.AuthenticateAgentRequest(request.Context(), request.Header.Get(api.AgentKeyHeader)); valid {
			next.ServeHTTP(response, api.WithPrincipal(request, principal))
			return
		}
		if principal, valid := api.AuthenticateSellerRequest(request.Context(), request.Header.Get("Authorization")); valid {
			next.ServeHTTP(response, api.WithPrincipal(request, principal))
			return
		}
		if authorizer == nil {
			api.WriteError(response, request, http.StatusUnauthorized, api.ErrorCodeUnauthorized, "authentication is required", nil)
			return
		}
		authorizer.Middleware(requirement, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			authorization, _ := AuthorizationFromContext(request.Context())
			principal := api.Principal{
				Kind: api.PrincipalBrowser, Subject: authorization.BuyerID,
				SessionID: authorization.PurchaseSession.PurchaseSessionID.String(),
				ExpiresAt: authorization.PurchaseSession.AccessExpiresAt.Time(),
			}
			next.ServeHTTP(response, api.WithPrincipal(request, principal))
		})).ServeHTTP(response, request)
	})
}

func (authorizer *RequestAuthorizer) Middleware(requirement func(*http.Request) (AuthorizationRequirement, error), next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		required, err := requirement(request)
		if err != nil {
			writeError(response, request, err)
			return
		}
		authorization, err := authorizer.Authorize(request, required)
		if err != nil {
			writeError(response, request, err)
			return
		}
		next.ServeHTTP(response, request.WithContext(context.WithValue(request.Context(), authorizationContextKey{}, authorization)))
	})
}

func setNoStore(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Pragma", "no-cache")
}

func writeError(response http.ResponseWriter, request *http.Request, err error) {
	status := http.StatusServiceUnavailable
	code := api.ErrorCodeDependencyUnavailable
	message := "browser purchase authorization is unavailable"
	switch {
	case errors.Is(err, ErrValidation):
		status, code, message = http.StatusBadRequest, api.ErrorCodeBadRequest, err.Error()
	case errors.Is(err, ErrMaximumBelowQuote), errors.Is(err, ErrWalletMismatch), errors.Is(err, ErrWalletNotBound):
		status, code, message = http.StatusUnprocessableEntity, api.ErrorCodeUnprocessable, err.Error()
	case errors.Is(err, persistence.ErrNotFound):
		status, code, message = http.StatusNotFound, api.ErrorCodeNotFound, "browser purchase was not found"
	case errors.Is(err, ErrIdempotencyConflict):
		status, code, message = http.StatusConflict, api.ErrorCodeIdempotencyConflict, err.Error()
	case errors.Is(err, persistence.ErrAlreadyExists), errors.Is(err, ErrChallengeConsumed), errors.Is(err, ErrCommerceConsumed), errors.Is(err, ErrBindingMismatch):
		status, code, message = http.StatusConflict, api.ErrorCodeConflict, err.Error()
	case errors.Is(err, ErrCommerceExpired), errors.Is(err, ErrAccessExpired), errors.Is(err, ErrChallengeExpired):
		status, code, message = http.StatusGone, api.ErrorCodeGone, err.Error()
	case errors.Is(err, ErrInvalidGrant), errors.Is(err, ErrRecoveryProofInvalid):
		status, code, message = http.StatusUnauthorized, api.ErrorCodeInvalidCredential, err.Error()
	case errors.Is(err, ErrGrantRevoked):
		status, code, message = http.StatusUnauthorized, api.ErrorCodeTokenRevoked, err.Error()
	case errors.Is(err, ErrOriginDenied), errors.Is(err, ErrCSRFInvalid):
		status, code, message = http.StatusForbidden, api.ErrorCodePermissionDenied, err.Error()
	case errors.Is(err, domain.ErrCommerceUnavailable):
		status, code, message = http.StatusGone, api.ErrorCodeGone, "seller commerce is unavailable"
	}
	setNoStore(response)
	api.WriteError(response, request, status, code, message, nil)
}
