package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// Config contains dependencies shared by HTTP middleware.
type Config struct {
	AllowedOrigin        string
	Authenticator        Authenticator
	SellerRequestLimiter SellerRequestLimiter
	SellerAuthorizer     SellerRequestAuthorizer
	Logger               *slog.Logger
}

// Middleware applies request identity, recovery, CORS, logging, and auth context.
func Middleware(config Config, next http.Handler) http.Handler {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestID := incomingRequestID(request.Header.Get(RequestIDHeader))
		if requestID == "" {
			requestID = newRequestID()
		}

		response.Header().Set(RequestIDHeader, requestID)
		requestContext := context.WithValue(
			request.Context(),
			requestIDContextKey{},
			requestID,
		)
		requestContext = context.WithValue(
			requestContext,
			authenticatorContextKey{},
			config.Authenticator,
		)
		request = request.WithContext(requestContext)

		if handleCORS(response, request, config.AllowedOrigin) {
			return
		}

		statusResponse := &statusWriter{
			ResponseWriter: response,
			status:         http.StatusOK,
		}
		startedAt := time.Now()
		defer func() {
			if recover() != nil {
				WriteError(
					statusResponse,
					request,
					http.StatusInternalServerError,
					ErrorCodeInternal,
					"an internal error occurred",
					nil,
				)
			}
			logger.Info(
				"http request",
				"requestId",
				requestID,
				"method",
				request.Method,
				"path",
				request.URL.Path,
				"status",
				statusResponse.status,
				"durationMs",
				time.Since(startedAt).Milliseconds(),
			)
		}()
		if sellerID, ok := sellerIDFromPath(request.URL.Path); ok {
			token, validBearer := bearerToken(request.Header.Get("Authorization"))
			principal, valid := authenticateSeller(request.Context(), config.Authenticator, token)
			if !validBearer || !valid {
				writeSellerAuthenticationError(statusResponse, request)
				return
			}
			request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, principal))
			if config.SellerAuthorizer != nil {
				if err := config.SellerAuthorizer.AuthorizeSeller(request.Context(), principal.Subject, sellerID); err != nil {
					writeSellerAuthorizationError(statusResponse, request, err)
					return
				}
			}
			if config.SellerRequestLimiter != nil {
				if err := config.SellerRequestLimiter.ConsumeAPIRequest(request.Context(), sellerID); err != nil {
					writeQuotaError(statusResponse, request, err)
					return
				}
			}
		}

		next.ServeHTTP(statusResponse, request)
	})
}

// RequireSeller authenticates a seller bearer token before calling next.
func RequireSeller(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if principal, ok := PrincipalFromContext(request.Context()); ok && principal.Kind == PrincipalSeller {
			next.ServeHTTP(response, request)
			return
		}
		authenticator := authenticatorFromContext(request.Context())
		token, validBearer := bearerToken(request.Header.Get("Authorization"))
		principal, valid := authenticateSeller(request.Context(), authenticator, token)
		if !validBearer || !valid {
			writeSellerAuthenticationError(response, request)
			return
		}

		request = request.WithContext(context.WithValue(
			request.Context(),
			principalContextKey{},
			principal,
		))
		next.ServeHTTP(response, request)
	})
}

// sellerIDFromPath extracts a seller identifier from control-plane paths.
func sellerIDFromPath(path string) (domain.ID, bool) {
	const prefix = "/v1/sellers/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	segment, _, _ := strings.Cut(strings.TrimPrefix(path, prefix), "/")
	sellerID, err := domain.ParseID(segment, domain.SellerIDPrefix)
	return sellerID, err == nil
}

// writeQuotaError maps quota failures to stable HTTP errors.
func writeQuotaError(response http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, domain.ErrPermissionDenied) {
		WriteError(response, request, http.StatusForbidden, ErrorCodePermissionDenied, err.Error(), nil)
		return
	}
	if errors.Is(err, domain.ErrRateLimitExceeded) {
		response.Header().Set("Retry-After", "1")
		WriteError(response, request, http.StatusTooManyRequests, ErrorCodeRateLimited, err.Error(), nil)
		return
	}
	WriteError(response, request, http.StatusInternalServerError, ErrorCodeInternal, "quota enforcement failed", nil)
}

// RequireAgent authenticates an agent API key before calling next.
func RequireAgent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		authenticator := authenticatorFromContext(request.Context())
		principal, valid := authenticateAgent(
			request.Context(),
			authenticator,
			request.Header.Get(AgentKeyHeader),
		)
		if !valid {
			WriteError(
				response,
				request,
				http.StatusUnauthorized,
				ErrorCodeUnauthorized,
				"agent authentication is required",
				nil,
			)
			return
		}

		request = request.WithContext(context.WithValue(
			request.Context(),
			principalContextKey{},
			principal,
		))
		next.ServeHTTP(response, request)
	})
}

// RequireAgentOrSeller accepts either documented authenticated caller type.
func RequireAgentOrSeller(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		authenticator := authenticatorFromContext(request.Context())
		principal, valid := authenticateAgent(
			request.Context(),
			authenticator,
			request.Header.Get(AgentKeyHeader),
		)
		if !valid {
			token, validBearer := bearerToken(request.Header.Get("Authorization"))
			if validBearer {
				principal, valid = authenticateSeller(request.Context(), authenticator, token)
			}
		}
		if !valid {
			WriteError(response, request, http.StatusUnauthorized, ErrorCodeUnauthorized, "authentication is required", nil)
			return
		}

		request = request.WithContext(context.WithValue(
			request.Context(),
			principalContextKey{},
			principal,
		))
		next.ServeHTTP(response, request)
	})
}

// RequestIDFromContext returns the middleware request identifier.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDContextKey{}).(string)
	return requestID, ok
}

// PrincipalFromContext returns the authenticated caller.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

// WithPrincipal attaches an already authenticated principal for feature-owned middleware.
func WithPrincipal(request *http.Request, principal Principal) *http.Request {
	return request.WithContext(context.WithValue(request.Context(), principalContextKey{}, principal))
}

func bearerToken(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func writeSellerAuthenticationError(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("WWW-Authenticate", `Bearer realm="agentpay-seller"`)
	response.Header().Set("Cache-Control", "no-store")
	WriteError(response, request, http.StatusUnauthorized, ErrorCodeUnauthorized, "seller authentication is required", nil)
}

func writeSellerAuthorizationError(response http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, persistence.ErrNotFound) {
		WriteError(response, request, http.StatusNotFound, ErrorCodeNotFound, "seller was not found", nil)
		return
	}
	if errors.Is(err, domain.ErrPermissionDenied) {
		WriteError(response, request, http.StatusForbidden, ErrorCodePermissionDenied, "seller access is denied", nil)
		return
	}
	WriteError(response, request, http.StatusServiceUnavailable, ErrorCodeDependencyUnavailable, "seller authorization is unavailable", nil)
}

// authenticatorFromContext returns the configured authentication boundary.
func authenticatorFromContext(ctx context.Context) Authenticator {
	authenticator, _ := ctx.Value(authenticatorContextKey{}).(Authenticator)
	return authenticator
}

// authenticateSeller safely invokes the configured seller authenticator.
func authenticateSeller(
	ctx context.Context,
	authenticator Authenticator,
	token string,
) (Principal, bool) {
	if authenticator == nil {
		return Principal{}, false
	}
	return authenticator.AuthenticateSeller(ctx, token)
}

// authenticateAgent safely invokes the configured agent authenticator.
func authenticateAgent(
	ctx context.Context,
	authenticator Authenticator,
	key string,
) (Principal, bool) {
	if authenticator == nil {
		return Principal{}, false
	}
	return authenticator.AuthenticateAgent(ctx, key)
}

// handleCORS applies configured CORS headers and handles preflight requests.
func handleCORS(
	response http.ResponseWriter,
	request *http.Request,
	allowedOrigin string,
) bool {
	origin := request.Header.Get("Origin")
	if allowedOrigin != "" && origin == allowedOrigin {
		response.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		response.Header().Set("Vary", "Origin")
		response.Header().Set(
			"Access-Control-Allow-Headers",
			"Authorization, Content-Type, Idempotency-Key, X-AgentPay-Agent-Key",
		)
		response.Header().Set(
			"Access-Control-Allow-Methods",
			"DELETE, GET, POST, PATCH, OPTIONS",
		)
	}
	if request.Method == http.MethodOptions {
		response.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// statusWriter captures the status code for structured request logging.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// WriteHeader records the first status code written by a handler.
func (writer *statusWriter) WriteHeader(status int) {
	if writer.wroteHeader {
		return
	}
	writer.status = status
	writer.wroteHeader = true
	writer.ResponseWriter.WriteHeader(status)
}
