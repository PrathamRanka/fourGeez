package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/integrations/analyzer"
	"github.com/fourgeez/agentpay/internal/persistence"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "agentpay"
	serverVersion = "0.4.0"
)

type principalContextKey struct{}

// HTTPController authenticates and serves the remote MCP endpoint.
type HTTPController struct {
	authenticator   CredentialAuthenticator
	resourceService *Service
	mutationService *MutationService
	analyzerService *analyzer.Service
	streamHandler   http.Handler
}

// NewHTTPController creates an authenticated stateless MCP controller.
func NewHTTPController(
	authenticator CredentialAuthenticator,
	resourceService *Service,
	mutationService *MutationService,
	analyzerService *analyzer.Service,
) *HTTPController {
	controller := &HTTPController{
		authenticator:   authenticator,
		resourceService: resourceService,
		mutationService: mutationService,
		analyzerService: analyzerService,
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
	principal, err := controller.authenticator.AuthenticateToken(
		request.Context(),
		rawToken,
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
				if !principal.HasScope(integrations.ScopeRead) {
					return nil, integrations.ErrScopeDenied
				}
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
	if controller.mutationService != nil {
		controller.registerTools(server, principal)
	}
	if controller.analyzerService != nil {
		controller.registerAnalyzerTool(server, principal)
	}
	return server
}

// registerAnalyzerTool adds deterministic non-publishing repository analysis.
func (controller *HTTPController) registerAnalyzerTool(
	server *protocol.Server,
	principal integrations.Principal,
) {
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:        "analyze_repository",
			Description: "Propose supported routes from an allowlisted manifest and OpenAPI contract",
			Annotations: &protocol.ToolAnnotations{
				IdempotentHint: true,
				ReadOnlyHint:   true,
			},
		},
		func(
			_ context.Context,
			_ *protocol.CallToolRequest,
			input analyzer.Request,
		) (*protocol.CallToolResult, analyzer.Result, error) {
			if !principal.HasScope(integrations.ScopeValidate) {
				return nil, analyzer.Result{}, integrations.ErrScopeDenied
			}
			result, err := controller.analyzerService.Analyze(input)
			return nil, result, err
		},
	)
}

// registerTools adds the fixed, scoped AUT-004 mutation surface.
func (controller *HTTPController) registerTools(
	server *protocol.Server,
	principal integrations.Principal,
) {
	annotations := &protocol.ToolAnnotations{
		IdempotentHint: true,
		ReadOnlyHint:   false,
	}
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:        "configure_storefront",
			Description: "Update the existing seller storefront after explicit confirmation",
			Annotations: annotations,
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input ConfigureStorefrontInput,
		) (*protocol.CallToolResult, map[string]any, error) {
			result, err := controller.mutationService.ConfigureStorefront(
				ctx,
				principal,
				input,
			)
			return mutationToolResult(result, err)
		},
	)
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:        "configure_route",
			Description: "Create an unpublished paid-route draft after explicit confirmation",
			Annotations: annotations,
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input ConfigureRouteInput,
		) (*protocol.CallToolResult, map[string]any, error) {
			result, err := controller.mutationService.ConfigureRoute(ctx, principal, input)
			return mutationToolResult(result, err)
		},
	)
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:        "change_route_price",
			Description: "Change future-intent pricing after explicit confirmation",
			Annotations: annotations,
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input ChangeRoutePriceInput,
		) (*protocol.CallToolResult, map[string]any, error) {
			result, err := controller.mutationService.ChangeRoutePrice(ctx, principal, input)
			return mutationToolResult(result, err)
		},
	)
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:        "validate_route",
			Description: "Run deterministic publication checks without publishing",
			Annotations: &protocol.ToolAnnotations{
				IdempotentHint: true,
				ReadOnlyHint:   true,
			},
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input ValidateRouteInput,
		) (*protocol.CallToolResult, map[string]any, error) {
			result, err := controller.mutationService.ValidateRoute(ctx, principal, input)
			return mutationToolResult(result, err)
		},
	)
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:        "publish_route",
			Description: "Validate and publish one draft route after explicit confirmation",
			Annotations: annotations,
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input PublishRouteInput,
		) (*protocol.CallToolResult, map[string]any, error) {
			result, err := controller.mutationService.PublishRoute(ctx, principal, input)
			return mutationToolResult(result, err)
		},
	)
}

// mutationToolResult converts domain JSON types into their exact MCP wire shape.
func mutationToolResult(
	result MutationResult,
	err error,
) (*protocol.CallToolResult, map[string]any, error) {
	if err != nil {
		return nil, nil, safeMutationError(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, nil, err
	}
	var structuredResult map[string]any
	if err := json.Unmarshal(encoded, &structuredResult); err != nil {
		return nil, nil, err
	}
	return nil, structuredResult, nil
}

// safeMutationError preserves actionable domain errors and redacts internals.
func safeMutationError(err error) error {
	var validationError domain.ValidationError
	var validationErrors domain.ValidationErrors
	if errors.As(err, &validationError) ||
		errors.As(err, &validationErrors) ||
		errors.Is(err, integrations.ErrScopeDenied) ||
		errors.Is(err, api.ErrIdempotencyConflict) ||
		errors.Is(err, catalog.ErrRouteValidation) ||
		errors.Is(err, catalog.ErrRoutePublished) ||
		errors.Is(err, persistence.ErrNotFound) ||
		errors.Is(err, persistence.ErrAlreadyExists) ||
		errors.Is(err, persistence.ErrConditionFailed) {
		return err
	}
	return errors.New("AgentPay mutation failed")
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
