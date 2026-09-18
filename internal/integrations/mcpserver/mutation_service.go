package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/integrations/sandbox"
)

const (
	minimumConfirmationSummaryLength = 10
	maximumConfirmationSummaryLength = 500
	maximumConfirmationAge           = 10 * time.Minute
)

// ErrMutationConflict reports idempotency-key reuse with different arguments.
var ErrMutationConflict = api.ErrIdempotencyConflict

var (
	ErrSandboxValidationUnavailable = errors.New("sandbox validation is unavailable")
	ErrSandboxValidationFailed      = errors.New("sandbox validation failed")
	ErrSandboxValidationStale       = errors.New("sandbox validation did not cover the expected route version")
)

// MutationService executes confirmed, scoped, and replay-safe MCP operations.
type MutationService struct {
	catalogMutator   CatalogMutator
	idempotencyStore domain.IdempotencyStore
	clock            domain.Clock
	sandboxValidator SandboxValidator
	auditRecorder    audit.Recorder
}

// NewMutationService creates the MCP catalog mutation service.
func NewMutationService(
	catalogMutator CatalogMutator,
	idempotencyStore domain.IdempotencyStore,
	clock domain.Clock,
	sandboxValidator SandboxValidator,
	auditRecorder audit.Recorder,
) *MutationService {
	return &MutationService{
		catalogMutator:   catalogMutator,
		idempotencyStore: idempotencyStore,
		clock:            clock,
		sandboxValidator: sandboxValidator,
		auditRecorder:    auditRecorder,
	}
}

// SandboxValidateRoute runs live seller checks without persisting success.
func (service *MutationService) SandboxValidateRoute(
	ctx context.Context,
	principal integrations.Principal,
	input SandboxValidateRouteInput,
) (sandbox.Result, error) {
	if !principal.HasScope(integrations.ScopeValidate) {
		return sandbox.Result{}, integrations.ErrScopeDenied
	}
	if service.sandboxValidator == nil {
		return sandbox.Result{}, ErrSandboxValidationUnavailable
	}
	routeID, err := domain.ParseID(input.RouteID, domain.RouteIDPrefix)
	if err != nil {
		return sandbox.Result{}, err
	}
	return service.sandboxValidator.Validate(
		ctx,
		principal.SellerID,
		routeID,
	)
}

// ConfigureStorefront updates safe storefront fields after confirmation.
func (service *MutationService) ConfigureStorefront(
	ctx context.Context,
	principal integrations.Principal,
	input ConfigureStorefrontInput,
) (MutationResult, error) {
	return service.execute(
		ctx,
		principal,
		integrations.ScopeConfigure,
		"configure_storefront",
		principal.SellerID.String(),
		input.IdempotencyKey,
		input.Confirmation,
		input,
		func() (MutationResult, error) {
			seller, err := service.catalogMutator.ConfigureStorefrontForIntegration(
				ctx,
				principal.SellerID,
				input.Storefront,
			)
			if err != nil {
				return MutationResult{}, err
			}
			return MutationResult{Operation: "configure_storefront", Seller: &seller}, nil
		},
	)
}

// ConfigureRoute creates an unpublished route after confirmation.
func (service *MutationService) ConfigureRoute(
	ctx context.Context,
	principal integrations.Principal,
	input ConfigureRouteInput,
) (MutationResult, error) {
	return service.execute(
		ctx,
		principal,
		integrations.ScopeConfigure,
		"configure_route",
		"new",
		input.IdempotencyKey,
		input.Confirmation,
		input,
		func() (MutationResult, error) {
			routeRequest, err := routeRequest(input.Route)
			if err != nil {
				return MutationResult{}, err
			}
			route, err := service.catalogMutator.CreateDraftRouteForIntegration(
				ctx,
				principal.SellerID,
				routeRequest,
			)
			if err != nil {
				return MutationResult{}, err
			}
			if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
				SellerID:   principal.SellerID,
				ActorType:  audit.ActorTypeIntegrationCredential,
				ActorID:    principal.CredentialID.String(),
				Action:     audit.ActionRouteDraftCreated,
				TargetType: audit.TargetTypePaidRoute,
				TargetID:   route.RouteID.String(),
				Outcome:    audit.OutcomeSucceeded,
				ChangedFields: []string{
					"displayName",
					"productSlug",
					"method",
					"pathPattern",
					"description",
					"mimeType",
					"amount",
					"asset",
					"network",
					"payTo",
					"approvalThresholdAmount",
					"upstreamTimeoutSeconds",
					"enabled",
				},
			}); err != nil {
				return MutationResult{}, err
			}
			return MutationResult{Operation: "configure_route", Route: &route}, nil
		},
	)
}

// ChangeRoutePrice updates the authoritative future-intent price.
func (service *MutationService) ChangeRoutePrice(
	ctx context.Context,
	principal integrations.Principal,
	input ChangeRoutePriceInput,
) (MutationResult, error) {
	routeID, err := domain.ParseID(input.RouteID, domain.RouteIDPrefix)
	if err != nil {
		return MutationResult{}, err
	}
	amount, err := domain.ParseAmount(input.Price.Amount)
	if err != nil {
		return MutationResult{}, err
	}
	return service.execute(
		ctx,
		principal,
		integrations.ScopeConfigure,
		"change_route_price",
		routeID.String(),
		input.IdempotencyKey,
		input.Confirmation,
		input,
		func() (MutationResult, error) {
			route, err := service.catalogMutator.UpdateRoutePriceForIntegration(
				ctx,
				principal.SellerID,
				routeID,
				catalog.UpdateRoutePriceRequest{
					Amount:          amount,
					ExpectedVersion: input.Price.ExpectedVersion,
				},
			)
			if err != nil {
				return MutationResult{}, err
			}
			if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
				SellerID:   principal.SellerID,
				ActorType:  audit.ActorTypeIntegrationCredential,
				ActorID:    principal.CredentialID.String(),
				Action:     audit.ActionRoutePriceChanged,
				TargetType: audit.TargetTypePaidRoute,
				TargetID:   routeID.String(),
				Outcome:    audit.OutcomeSucceeded,
				ChangedFields: []string{
					"amount",
				},
			}); err != nil {
				return MutationResult{}, err
			}
			return MutationResult{Operation: "change_route_price", Route: &route}, nil
		},
	)
}

// routeRequest validates MCP atomic-unit strings into the catalog request.
func routeRequest(configuration RouteConfiguration) (catalog.CreateRouteRequest, error) {
	amount, err := domain.ParseAmount(configuration.Amount)
	if err != nil {
		return catalog.CreateRouteRequest{}, err
	}
	var approvalThreshold *domain.Amount
	if configuration.ApprovalThresholdAmount != nil {
		parsedThreshold, err := domain.ParseAmount(*configuration.ApprovalThresholdAmount)
		if err != nil {
			return catalog.CreateRouteRequest{}, err
		}
		approvalThreshold = &parsedThreshold
	}
	return catalog.CreateRouteRequest{
		DisplayName:             configuration.DisplayName,
		ProductSlug:             configuration.ProductSlug,
		Method:                  configuration.Method,
		PathPattern:             configuration.PathPattern,
		Description:             configuration.Description,
		MIMEType:                configuration.MIMEType,
		Amount:                  amount,
		Asset:                   configuration.Asset,
		Network:                 configuration.Network,
		PayTo:                   configuration.PayTo,
		ApprovalThresholdAmount: approvalThreshold,
		UpstreamTimeoutSeconds:  configuration.UpstreamTimeoutSeconds,
	}, nil
}

// ValidateRoute runs deterministic checks without publishing the route.
func (service *MutationService) ValidateRoute(
	ctx context.Context,
	principal integrations.Principal,
	input ValidateRouteInput,
) (MutationResult, error) {
	routeID, err := domain.ParseID(input.RouteID, domain.RouteIDPrefix)
	if err != nil {
		return MutationResult{}, err
	}
	return service.execute(
		ctx,
		principal,
		integrations.ScopeValidate,
		"validate_route",
		routeID.String(),
		input.IdempotencyKey,
		input.Confirmation,
		input,
		func() (MutationResult, error) {
			validation, err := service.catalogMutator.ValidateRouteForIntegration(
				ctx,
				principal.SellerID,
				routeID,
			)
			if err != nil {
				return MutationResult{}, err
			}
			return MutationResult{Operation: "validate_route", Validation: &validation}, nil
		},
	)
}

// PublishRoute validates and conditionally enables one route draft.
func (service *MutationService) PublishRoute(
	ctx context.Context,
	principal integrations.Principal,
	input PublishRouteInput,
) (MutationResult, error) {
	routeID, err := domain.ParseID(input.RouteID, domain.RouteIDPrefix)
	if err != nil {
		return MutationResult{}, err
	}
	return service.execute(
		ctx,
		principal,
		integrations.ScopePublish,
		"publish_route",
		routeID.String(),
		input.IdempotencyKey,
		input.Confirmation,
		input,
		func() (MutationResult, error) {
			if service.sandboxValidator == nil {
				return MutationResult{}, ErrSandboxValidationUnavailable
			}
			sandboxResult, err := service.sandboxValidator.Validate(
				ctx,
				principal.SellerID,
				routeID,
			)
			if err != nil {
				return MutationResult{}, err
			}
			if !sandboxResult.Valid {
				return MutationResult{}, ErrSandboxValidationFailed
			}
			if sandboxResult.RouteVersion != input.ExpectedVersion {
				return MutationResult{}, ErrSandboxValidationStale
			}
			route, err := service.catalogMutator.PublishRouteForIntegration(
				ctx,
				principal.SellerID,
				routeID,
				input.ExpectedVersion,
			)
			if err != nil {
				return MutationResult{}, err
			}
			if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
				SellerID:   principal.SellerID,
				ActorType:  audit.ActorTypeIntegrationCredential,
				ActorID:    principal.CredentialID.String(),
				Action:     audit.ActionRoutePublished,
				TargetType: audit.TargetTypePaidRoute,
				TargetID:   routeID.String(),
				Outcome:    audit.OutcomeSucceeded,
				ChangedFields: []string{
					"enabled",
				},
			}); err != nil {
				return MutationResult{}, err
			}
			return MutationResult{Operation: "publish_route", Route: &route}, nil
		},
	)
}

// execute applies scope, replay, confirmation, execution, and response storage.
func (service *MutationService) execute(
	ctx context.Context,
	principal integrations.Principal,
	requiredScope integrations.Scope,
	operation string,
	target string,
	idempotencyKey string,
	confirmation Confirmation,
	input any,
	execute func() (MutationResult, error),
) (MutationResult, error) {
	if !principal.HasScope(requiredScope) {
		return MutationResult{}, integrations.ErrScopeDenied
	}
	requestBody, err := json.Marshal(input)
	if err != nil {
		return MutationResult{}, err
	}
	scope := principal.CredentialID.String() + ":" + operation + ":" + target
	decision, err := api.CheckIdempotency(
		ctx,
		service.idempotencyStore,
		scope,
		idempotencyKey,
		requestBody,
	)
	if err != nil {
		return MutationResult{}, err
	}
	if decision.Replay {
		var result MutationResult
		if err := json.Unmarshal(decision.Body, &result); err != nil {
			return MutationResult{}, err
		}
		return result, nil
	}
	if err := service.validateConfirmation(confirmation); err != nil {
		return MutationResult{}, err
	}
	result, err := execute()
	if err != nil {
		return MutationResult{}, err
	}
	responseBody, err := json.Marshal(result)
	if err != nil {
		return MutationResult{}, err
	}
	if err := api.SaveIdempotency(
		ctx,
		service.idempotencyStore,
		scope,
		decision,
		http.StatusOK,
		responseBody,
		service.clock.Now(),
	); err != nil {
		return MutationResult{}, err
	}
	return result, nil
}

// validateConfirmation rejects fabricated, stale, or ambiguous approval data.
func (service *MutationService) validateConfirmation(
	confirmation Confirmation,
) error {
	if !confirmation.Approved {
		return domain.NewValidationError(
			"confirmation.approved",
			"required",
			"must be explicitly approved",
		)
	}
	summary := strings.TrimSpace(confirmation.Summary)
	if len(summary) < minimumConfirmationSummaryLength ||
		len(summary) > maximumConfirmationSummaryLength {
		return domain.NewValidationError(
			"confirmation.summary",
			"length",
			"must contain 10-500 characters",
		)
	}
	now := service.clock.Now()
	confirmedAt, err := domain.ParseTimestamp(confirmation.ConfirmedAt)
	if err != nil {
		return domain.NewValidationError(
			"confirmation.confirmedAt",
			"rfc3339",
			"must be a valid RFC 3339 timestamp",
		)
	}
	if confirmedAt.Time().After(now) ||
		now.Sub(confirmedAt.Time()) > maximumConfirmationAge {
		return domain.NewValidationError(
			"confirmation.confirmedAt",
			"freshness",
			"must be within the previous ten minutes",
		)
	}
	return nil
}
