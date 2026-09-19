package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/integrations/analyzer"
	"github.com/fourgeez/agentpay/internal/integrations/discovery"
	"github.com/fourgeez/agentpay/internal/integrations/sandbox"
	"github.com/fourgeez/agentpay/internal/integrations/stacks"
	"github.com/fourgeez/agentpay/internal/persistence"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "agentpay"
	serverVersion = "0.5.0"
)

type principalContextKey struct{}

// OperationQuota consumes one authenticated seller MCP operation.
type OperationQuota interface {
	ConsumeMCPOperation(context.Context, domain.ID) error
}

type ConnectorVerificationRecorder interface {
	RecordAuthenticatedConnectorVerification(context.Context, domain.ID) error
}

// HTTPController authenticates and serves the remote MCP endpoint.
type HTTPController struct {
	authenticator                 AccessTokenAuthorizer
	resourceService               *Service
	mutationService               *MutationService
	analyzerService               *analyzer.Service
	discoveryService              *discovery.Service
	streamHandler                 http.Handler
	quotaEnforcer                 OperationQuota
	connectorVerificationRecorder ConnectorVerificationRecorder
}

// SetDiscoveryValidator configures deterministic storefront artifact checks.
func (controller *HTTPController) SetDiscoveryValidator(
	discoveryService *discovery.Service,
) {
	controller.discoveryService = discoveryService
}

// NewHTTPController creates an authenticated stateless MCP controller.
func NewHTTPController(
	authenticator AccessTokenAuthorizer,
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

// SetQuotaEnforcer configures per-seller MCP operation metering.
func (controller *HTTPController) SetQuotaEnforcer(quotaEnforcer OperationQuota) {
	controller.quotaEnforcer = quotaEnforcer
}

func (controller *HTTPController) SetConnectorVerificationRecorder(recorder ConnectorVerificationRecorder) {
	controller.connectorVerificationRecorder = recorder
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
		controller.writeAuthenticationError(response, request, authorization.ErrInvalidAccessToken)
		return
	}
	principal, err := controller.authenticator.AuthorizeAccessToken(
		request.Context(),
		rawToken,
	)
	if err != nil {
		controller.writeAuthenticationError(response, request, err)
		return
	}
	if controller.connectorVerificationRecorder != nil {
		if err := controller.connectorVerificationRecorder.RecordAuthenticatedConnectorVerification(request.Context(), principal.SellerID); err != nil {
			response.Header().Set("Cache-Control", "no-store")
			api.WriteError(response, request, http.StatusServiceUnavailable, api.ErrorCodeDependencyUnavailable, "connector verification state is unavailable", nil)
			return
		}
	}
	if request.Method == http.MethodPost && controller.quotaEnforcer != nil {
		if err := controller.quotaEnforcer.ConsumeMCPOperation(
			request.Context(),
			principal.SellerID,
		); err != nil {
			controller.writeQuotaError(response, request, err)
			return
		}
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

// writeQuotaError maps MCP quota failures to deterministic HTTP statuses.
func (controller *HTTPController) writeQuotaError(
	response http.ResponseWriter,
	request *http.Request,
	err error,
) {
	response.Header().Set("Cache-Control", "no-store")
	if errors.Is(err, domain.ErrPermissionDenied) {
		api.WriteError(response, request, http.StatusForbidden, api.ErrorCodePermissionDenied, "MCP operation is not permitted", nil)
		return
	}
	if errors.Is(err, domain.ErrRateLimitExceeded) {
		response.Header().Set("Retry-After", "1")
		api.WriteError(response, request, http.StatusTooManyRequests, api.ErrorCodeRateLimited, "MCP operation quota exceeded", nil)
		return
	}
	api.WriteError(response, request, http.StatusServiceUnavailable, api.ErrorCodeDependencyUnavailable, "quota state is unavailable", nil)
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
	controller.registerSetupPrompt(server, principal)
	if controller.mutationService != nil {
		controller.registerTools(server, principal)
		controller.registerSandboxTool(server, principal)
	}
	if controller.analyzerService != nil {
		controller.registerStackDetectionTool(server, principal)
		controller.registerAnalyzerTool(server, principal)
	}
	if controller.discoveryService != nil {
		controller.registerDiscoveryTool(server, principal)
	}
	return server
}

// registerStackDetectionTool exposes the bounded detector used before prompt selection.
func (controller *HTTPController) registerStackDetectionTool(
	server *protocol.Server,
	principal integrations.Principal,
) {
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:        "detect_repository_stacks",
			Description: "Detect maintained stacks from bounded committed repository evidence",
			Annotations: &protocol.ToolAnnotations{
				IdempotentHint: true,
				ReadOnlyHint:   true,
			},
		},
		func(
			_ context.Context,
			_ *protocol.CallToolRequest,
			input stacks.DetectionRequest,
		) (*protocol.CallToolResult, stacks.DetectionResult, error) {
			if !principal.HasScope(integrations.ScopeValidate) {
				return nil, stacks.DetectionResult{}, integrations.ErrScopeDenied
			}
			detections, err := stacks.NewService().Detect(input.Files)
			if err != nil {
				return nil, stacks.DetectionResult{}, err
			}
			return nil, stacks.DetectionResult{
				SchemaVersion: stacks.DetectionSchemaVersion,
				Detections:    detections,
			}, nil
		},
	)
}

// registerDiscoveryTool adds deterministic storefront quality validation.
func (controller *HTTPController) registerDiscoveryTool(
	server *protocol.Server,
	principal integrations.Principal,
) {
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:        "validate_storefront_artifacts",
			Description: "Validate generated SEO, AEO, discovery, accessibility, and performance artifacts",
			Annotations: &protocol.ToolAnnotations{
				IdempotentHint: true,
				ReadOnlyHint:   true,
			},
		},
		func(
			_ context.Context,
			_ *protocol.CallToolRequest,
			input discovery.Request,
		) (*protocol.CallToolResult, discovery.Result, error) {
			if !principal.HasScope(integrations.ScopeValidate) {
				return nil, discovery.Result{}, integrations.ErrScopeDenied
			}
			result, err := controller.discoveryService.Validate(input)
			return nil, result, err
		},
	)
}

// registerSandboxTool adds the read-only pre-publication validation flow.
func (controller *HTTPController) registerSandboxTool(
	server *protocol.Server,
	principal integrations.Principal,
) {
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:        "sandbox_validate_route",
			Description: "Probe discovery, signature gating, and replay before publication",
			Annotations: &protocol.ToolAnnotations{
				IdempotentHint: true,
				ReadOnlyHint:   true,
			},
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input SandboxValidateRouteInput,
		) (*protocol.CallToolResult, sandbox.Result, error) {
			result, err := controller.mutationService.SandboxValidateRoute(
				ctx,
				principal,
				input,
			)
			if err != nil {
				return nil, sandbox.Result{}, safeMutationError(err)
			}
			return nil, result, nil
		},
	)
}

// registerSetupPrompt publishes the read-scoped coding-agent workflow.
func (controller *HTTPController) registerSetupPrompt(
	server *protocol.Server,
	principal integrations.Principal,
) {
	server.AddPrompt(
		&protocol.Prompt{
			Name:        SetupPromptName,
			Title:       "Prepare AgentPay integration",
			Description: "Prepare a tested, reviewable AgentPay seller integration",
			Arguments: []*protocol.PromptArgument{
				{
					Name:        "host",
					Description: "claude-code, codex, or generic-mcp",
					Required:    true,
				},
				{
					Name:        "stack",
					Description: "detected stack identifier from the version-two support matrix",
					Required:    true,
				},
			},
		},
		func(
			_ context.Context,
			request *protocol.GetPromptRequest,
		) (*protocol.GetPromptResult, error) {
			if !principal.HasScope(integrations.ScopeRead) {
				return nil, integrations.ErrScopeDenied
			}
			prompt, err := controller.resourceService.SetupPrompt(
				request.Params.Arguments["host"],
				request.Params.Arguments["stack"],
			)
			if err != nil {
				return nil, err
			}
			return &protocol.GetPromptResult{
				Description: "AgentPay seller integration workflow",
				Messages: []*protocol.PromptMessage{
					{
						Role:    protocol.Role("user"),
						Content: &protocol.TextContent{Text: prompt},
					},
				},
			}, nil
		},
	)
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
			Name:         "configure_storefront",
			Description:  "Update the existing seller storefront after explicit confirmation",
			Annotations:  annotations,
			OutputSchema: mutationOutputSchema("seller", sellerOutputSchema()),
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input ConfigureStorefrontInput,
		) (*protocol.CallToolResult, MutationResult, error) {
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
			Name:         "configure_route",
			Description:  "Create an unpublished paid-route draft after explicit confirmation",
			Annotations:  annotations,
			OutputSchema: mutationOutputSchema("route", routeOutputSchema()),
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input ConfigureRouteInput,
		) (*protocol.CallToolResult, MutationResult, error) {
			result, err := controller.mutationService.ConfigureRoute(ctx, principal, input)
			return mutationToolResult(result, err)
		},
	)
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:         "change_route_price",
			Description:  "Change future-intent pricing after explicit confirmation",
			Annotations:  annotations,
			OutputSchema: mutationOutputSchema("route", routeOutputSchema()),
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input ChangeRoutePriceInput,
		) (*protocol.CallToolResult, MutationResult, error) {
			result, err := controller.mutationService.ChangeRoutePrice(ctx, principal, input)
			return mutationToolResult(result, err)
		},
	)
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:         "validate_route",
			Description:  "Run deterministic publication checks without publishing",
			OutputSchema: mutationOutputSchema("validation", validationOutputSchema()),
			Annotations: &protocol.ToolAnnotations{
				IdempotentHint: true,
				ReadOnlyHint:   true,
			},
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input ValidateRouteInput,
		) (*protocol.CallToolResult, MutationResult, error) {
			result, err := controller.mutationService.ValidateRoute(ctx, principal, input)
			return mutationToolResult(result, err)
		},
	)
	protocol.AddTool(
		server,
		&protocol.Tool{
			Name:         "publish_route",
			Description:  "Validate and publish one draft route after explicit confirmation",
			Annotations:  annotations,
			OutputSchema: mutationOutputSchema("route", routeOutputSchema()),
		},
		func(
			ctx context.Context,
			_ *protocol.CallToolRequest,
			input PublishRouteInput,
		) (*protocol.CallToolResult, MutationResult, error) {
			result, err := controller.mutationService.PublishRoute(ctx, principal, input)
			return mutationToolResult(result, err)
		},
	)
}

// mutationToolResult converts domain JSON types into their exact MCP wire shape.
func mutationToolResult(
	result MutationResult,
	err error,
) (*protocol.CallToolResult, MutationResult, error) {
	if err != nil {
		return nil, MutationResult{}, safeMutationError(err)
	}
	return nil, result, nil
}

// safeMutationError preserves actionable domain errors and redacts internals.
func safeMutationError(err error) error {
	var validationError domain.ValidationError
	var validationErrors domain.ValidationErrors
	if errors.As(err, &validationError) ||
		errors.As(err, &validationErrors) ||
		errors.Is(err, integrations.ErrScopeDenied) ||
		errors.Is(err, domain.ErrPermissionDenied) ||
		errors.Is(err, authorization.ErrConfirmationDenied) ||
		errors.Is(err, authorization.ErrConfirmationReplayed) ||
		errors.Is(err, domain.ErrRateLimitExceeded) ||
		errors.Is(err, api.ErrIdempotencyConflict) ||
		errors.Is(err, catalog.ErrRouteValidation) ||
		errors.Is(err, catalog.ErrRouteContractStale) ||
		errors.Is(err, catalog.ErrRoutePublished) ||
		errors.Is(err, ErrSandboxValidationFailed) ||
		errors.Is(err, ErrSandboxValidationStale) ||
		errors.Is(err, ErrSandboxValidationUnavailable) ||
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
	request *http.Request,
	err error,
) {
	response.Header().Set("Cache-Control", "no-store")
	status := http.StatusServiceUnavailable
	code := api.ErrorCodeDependencyUnavailable
	message := "authorization state is unavailable"
	switch {
	case errors.Is(err, authorization.ErrInvalidAccessToken), errors.Is(err, integrations.ErrCredentialInvalid):
		status, code, message = http.StatusUnauthorized, api.ErrorCodeInvalidCredential, "access token is invalid"
	case errors.Is(err, authorization.ErrAccessTokenExpired), errors.Is(err, integrations.ErrCredentialExpired):
		status, code, message = http.StatusUnauthorized, api.ErrorCodeTokenExpired, "access token has expired"
	case errors.Is(err, authorization.ErrAccessTokenRevoked), errors.Is(err, integrations.ErrCredentialRevoked):
		status, code, message = http.StatusUnauthorized, api.ErrorCodeTokenRevoked, "access token has been revoked"
	case errors.Is(err, authorization.ErrSubscriptionInactive), errors.Is(err, integrations.ErrSubscriptionInactive):
		status, code, message = http.StatusForbidden, api.ErrorCodeSubscriptionInactive, "seller subscription is inactive"
	case errors.Is(err, authorization.ErrInsufficientScope), errors.Is(err, integrations.ErrScopeDenied):
		status, code, message = http.StatusForbidden, api.ErrorCodeInsufficientScope, "access token has insufficient scope"
	}
	if status == http.StatusUnauthorized {
		response.Header().Set("WWW-Authenticate", `Bearer realm="agentpay-mcp"`)
	}
	api.WriteError(response, request, status, code, message, nil)
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
