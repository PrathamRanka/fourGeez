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

// OperationQuota consumes one authenticated seller MCP operation.
type OperationQuota interface {
	ConsumeMCPOperation(context.Context, domain.ID) error
}

// HTTPController authenticates and serves the remote MCP endpoint.
type HTTPController struct {
	authenticator   CredentialAuthenticator
	resourceService *Service
	mutationService *MutationService
	analyzerService *analyzer.Service
	streamHandler   http.Handler
	quotaEnforcer   OperationQuota
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

// SetQuotaEnforcer configures per-seller MCP operation metering.
func (controller *HTTPController) SetQuotaEnforcer(quotaEnforcer OperationQuota) {
	controller.quotaEnforcer = quotaEnforcer
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
	if request.Method == http.MethodPost && controller.quotaEnforcer != nil {
		if err := controller.quotaEnforcer.ConsumeMCPOperation(
			request.Context(),
			principal.SellerID,
		); err != nil {
			controller.writeQuotaError(response, err)
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
	err error,
) {
	response.Header().Set("Cache-Control", "no-store")
	if errors.Is(err, domain.ErrPermissionDenied) {
		http.Error(response, api.ErrorCodePermissionDenied, http.StatusForbidden)
		return
	}
	if errors.Is(err, domain.ErrRateLimitExceeded) {
		response.Header().Set("Retry-After", "1")
		http.Error(response, api.ErrorCodeRateLimited, http.StatusTooManyRequests)
		return
	}
	http.Error(response, "quota enforcement failed", http.StatusInternalServerError)
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
		controller.registerAnalyzerTool(server, principal)
	}
	return server
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
		) (*protocol.CallToolResult, map[string]any, error) {
			result, err := controller.mutationService.SandboxValidateRoute(
				ctx,
				principal,
				input,
			)
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
					Name:        "framework",
					Description: "go, node, or python",
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
				request.Params.Arguments["framework"],
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
		errors.Is(err, domain.ErrPermissionDenied) ||
		errors.Is(err, domain.ErrRateLimitExceeded) ||
		errors.Is(err, api.ErrIdempotencyConflict) ||
		errors.Is(err, catalog.ErrRouteValidation) ||
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
