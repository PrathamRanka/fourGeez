package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/integrations"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "agentpay"
	serverVersion = "0.2.0"
)

type principalContextKey struct{}

// HTTPController authenticates and serves the remote MCP endpoint.
type HTTPController struct {
	authenticator   CredentialAuthenticator
	resourceService *Service
	streamHandler   http.Handler
}

// NewHTTPController creates an authenticated stateless MCP controller.
func NewHTTPController(
	authenticator CredentialAuthenticator,
	resourceService *Service,
) *HTTPController {
	controller := &HTTPController{
		authenticator:   authenticator,
		resourceService: resourceService,
	}
	controller.streamHandler = protocol.NewStreamableHTTPHandler(
		controller.serverForRequest,
		&protocol.StreamableHTTPOptions{
			Stateless:                    true,
			JSONResponse:                 true,
			MaxRequestBodyBytes:          api.MaximumJSONBodyBytes,
			PropagateRequestCancellation: true,
		},
	)
	return controller
}

// RegisterRoutes registers the single remote MCP endpoint.
func (controller *HTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/mcp", controller)
}

// ServeHTTP authenticates one integration credential before protocol handling.
func (controller *HTTPController) ServeHTTP(
	response http.ResponseWriter,
	request *http.Request,
) {
	rawToken, ok := bearerToken(request.Header.Get("Authorization"))
	if !ok {
		http.Error(response, "integration credential is required", http.StatusUnauthorized)
		return
	}
	principal, err := controller.authenticator.Authenticate(
		request.Context(),
		rawToken,
		integrations.ScopeRead,
	)
	if err != nil {
		controller.writeAuthenticationError(response, err)
		return
	}
	requestContext := context.WithValue(
		request.Context(),
		principalContextKey{},
		principal,
	)
	controller.streamHandler.ServeHTTP(
		response,
		request.WithContext(requestContext),
	)
}

// serverForRequest binds all resources to the authenticated seller principal.
func (controller *HTTPController) serverForRequest(
	request *http.Request,
) *protocol.Server {
	principal, ok := principalFromContext(request.Context())
	if !ok {
		return nil
	}
	server := protocol.NewServer(
		&protocol.Implementation{
			Name:    serverName,
			Version: serverVersion,
		},
		nil,
	)
	for _, descriptor := range controller.resourceService.Resources() {
		resourceDescriptor := descriptor
		server.AddResource(
			&protocol.Resource{
				URI:         resourceDescriptor.URI,
				Name:        resourceDescriptor.Name,
				Description: resourceDescriptor.Description,
				MIMEType:    resourceDescriptor.MIMEType,
			},
			func(
				ctx context.Context,
				request *protocol.ReadResourceRequest,
			) (*protocol.ReadResourceResult, error) {
				document, err := controller.resourceService.Read(
					ctx,
					principal,
					request.Params.URI,
				)
				if errors.Is(err, ErrResourceNotFound) {
					return nil, protocol.ResourceNotFoundError(request.Params.URI)
				}
				if err != nil {
					return nil, err
				}
				return &protocol.ReadResourceResult{
					Contents: []*protocol.ResourceContents{
						{
							URI:      document.URI,
							MIMEType: document.MIMEType,
							Text:     document.Text,
						},
					},
				}, nil
			},
		)
	}
	return server
}

// writeAuthenticationError maps credential failures without leaking internals.
func (controller *HTTPController) writeAuthenticationError(
	response http.ResponseWriter,
	err error,
) {
	response.Header().Set("Cache-Control", "no-store")
	if errors.Is(err, integrations.ErrScopeDenied) {
		http.Error(response, "integration credential lacks read scope", http.StatusForbidden)
		return
	}
	if errors.Is(err, integrations.ErrCredentialInvalid) ||
		errors.Is(err, integrations.ErrCredentialExpired) ||
		errors.Is(err, integrations.ErrCredentialRevoked) {
		http.Error(response, "integration credential is invalid", http.StatusUnauthorized)
		return
	}
	http.Error(response, "credential verification failed", http.StatusInternalServerError)
}

// bearerToken parses one strict HTTP bearer authorization value.
func bearerToken(authorization string) (string, bool) {
	fields := strings.Fields(authorization)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return "", false
	}
	if fields[1] == "" {
		return "", false
	}
	return fields[1], true
}

// principalFromContext returns the credential identity bound by ServeHTTP.
func principalFromContext(ctx context.Context) (integrations.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(integrations.Principal)
	return principal, ok
}
