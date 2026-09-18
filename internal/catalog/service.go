package catalog

import (
	"context"
	"errors"
	"mime"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const (
	maximumSellerNameLength       = 120
	maximumOwnerSubjectLength     = 256
	maximumSigningReferenceLength = 512
	maximumRouteDescriptionLength = 500
	maximumMIMETypeLength         = 120
	maximumAssetLength            = 160
	maximumNetworkLength          = 80
	maximumPayToLength            = 160
	minimumUpstreamTimeoutSeconds = 1
	maximumUpstreamTimeoutSeconds = 30
)

var (
	sellerSlugPattern           = regexp.MustCompile(`^[a-z0-9-]{3,48}$`)
	routePathPattern            = regexp.MustCompile(`^/[A-Za-z0-9/_-]+$`)
	ErrRoutePublished           = errors.New("paid route is already published")
	ErrRouteValidation          = errors.New("paid route failed publication validation")
	ErrRouteLifecycleTransition = errors.New("paid route lifecycle transition is not allowed")
)

// PublishedRouteQuota checks seller capacity before route publication.
type PublishedRouteQuota interface {
	AllowPublishedRoute(context.Context, domain.ID, uint64) error
}

// Service coordinates catalog domain rules with persistence boundaries.
type Service struct {
	repository    Repository
	idGenerator   domain.IDGenerator
	clock         domain.Clock
	auditRecorder audit.Recorder
	quotaEnforcer PublishedRouteQuota
}

// NewService creates the catalog application service.
func NewService(
	repository Repository,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
	auditRecorder audit.Recorder,
) *Service {
	return &Service{
		repository:    repository,
		idGenerator:   idGenerator,
		clock:         clock,
		auditRecorder: auditRecorder,
	}
}

// SetQuotaEnforcer configures plan quota checks for publication operations.
func (service *Service) SetQuotaEnforcer(quotaEnforcer PublishedRouteQuota) {
	service.quotaEnforcer = quotaEnforcer
}

// AuthorizeSeller verifies ownership without exposing another seller's record.
func (service *Service) AuthorizeSeller(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
) error {
	seller, err := service.repository.GetSeller(ctx, sellerID)
	if err != nil {
		return err
	}
	if seller.OwnerSubject != ownerSubject {
		return persistence.ErrNotFound
	}
	return nil
}

// CreateSeller creates a seller owned by the authenticated subject.
func (service *Service) CreateSeller(
	ctx context.Context,
	ownerSubject string,
	request CreateSellerRequest,
) (SellerResponse, error) {
	sellerID, err := service.idGenerator.New(domain.SellerIDPrefix)
	if err != nil {
		return SellerResponse{}, err
	}
	seller, err := NewSeller(SellerParams{
		SellerID:        sellerID,
		OwnerSubject:    ownerSubject,
		Slug:            request.Slug,
		Name:            request.Name,
		UpstreamBaseURL: request.UpstreamBaseURL,
		CreatedAt:       domain.NewTimestamp(service.clock.Now()),
	})
	if err != nil {
		return SellerResponse{}, err
	}
	if err := service.repository.CreateSeller(ctx, seller); err != nil {
		return SellerResponse{}, err
	}
	return sellerResponse(seller), nil
}

// CreateRoute creates a paid route for a seller owned by the caller.
func (service *Service) CreateRoute(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	request CreateRouteRequest,
) (PaidRoute, error) {
	seller, err := service.repository.GetSeller(ctx, sellerID)
	if err != nil {
		return PaidRoute{}, err
	}
	if seller.OwnerSubject != ownerSubject {
		return PaidRoute{}, domain.NewValidationError("sellerId", "owner", "seller is not owned by the caller")
	}
	routeID, err := service.idGenerator.New(domain.RouteIDPrefix)
	if err != nil {
		return PaidRoute{}, err
	}
	createRoute := NewPaidRoute
	if request.PublishImmediately != nil && !*request.PublishImmediately {
		createRoute = NewDraftPaidRoute
	}
	route, err := createRoute(PaidRouteParams{
		RouteID:                 routeID,
		SellerID:                sellerID,
		Method:                  request.Method,
		PathPattern:             request.PathPattern,
		Description:             request.Description,
		MIMEType:                request.MIMEType,
		Amount:                  request.Amount,
		Asset:                   request.Asset,
		Network:                 request.Network,
		PayTo:                   request.PayTo,
		ApprovalThresholdAmount: request.ApprovalThresholdAmount,
		UpstreamTimeoutSeconds:  request.UpstreamTimeoutSeconds,
		CreatedAt:               domain.NewTimestamp(service.clock.Now()),
	})
	if err != nil {
		return PaidRoute{}, err
	}
	if err := service.repository.CreateRoute(ctx, route); err != nil {
		return PaidRoute{}, err
	}
	action := audit.ActionRoutePublished
	if route.LifecycleStatus == RouteLifecycleDraft {
		action = audit.ActionRouteDraftCreated
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID:   sellerID,
		ActorType:  audit.ActorTypeSellerUser,
		ActorID:    ownerSubject,
		Action:     action,
		TargetType: audit.TargetTypePaidRoute,
		TargetID:   route.RouteID.String(),
		Outcome:    audit.OutcomeSucceeded,
		ChangedFields: []string{
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
			"lifecycleStatus",
			"enabled",
		},
	}); err != nil {
		return PaidRoute{}, err
	}
	return route, nil
}

// UpdateRoutePrice updates only future intent pricing for an owned route.
func (service *Service) UpdateRoutePrice(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	routeID domain.ID,
	request UpdateRoutePriceRequest,
) (PaidRoute, error) {
	seller, err := service.repository.GetSeller(ctx, sellerID)
	if err != nil {
		return PaidRoute{}, err
	}
	if seller.OwnerSubject != ownerSubject {
		return PaidRoute{}, domain.NewValidationError("sellerId", "owner", "seller is not owned by the caller")
	}
	route, err := service.repository.GetRoute(ctx, routeID)
	if err != nil {
		return PaidRoute{}, err
	}
	if route.SellerID != sellerID {
		return PaidRoute{}, domain.NewValidationError("routeId", "owner", "route does not belong to the seller")
	}
	if err := route.ChangePrice(request.Amount, domain.NewTimestamp(service.clock.Now())); err != nil {
		return PaidRoute{}, err
	}
	if err := service.repository.UpdateRoute(ctx, route, request.ExpectedVersion); err != nil {
		return PaidRoute{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID:   sellerID,
		ActorType:  audit.ActorTypeSellerUser,
		ActorID:    ownerSubject,
		Action:     audit.ActionRoutePriceChanged,
		TargetType: audit.TargetTypePaidRoute,
		TargetID:   routeID.String(),
		Outcome:    audit.OutcomeSucceeded,
		ChangedFields: []string{
			"amount",
		},
	}); err != nil {
		return PaidRoute{}, err
	}
	return route, nil
}

// ListSellerRoutes returns every route owned by the authenticated seller.
func (service *Service) ListSellerRoutes(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
) (PaidRouteList, error) {
	if err := service.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return PaidRouteList{}, err
	}
	routes, err := service.repository.ListRoutesBySeller(ctx, sellerID)
	if err != nil {
		return PaidRouteList{}, err
	}
	for index := range routes {
		routes[index].normalizeLifecycleStatus()
	}
	return PaidRouteList{Items: routes}, nil
}

// GetSellerRoute returns one route only when it belongs to the authenticated seller.
func (service *Service) GetSellerRoute(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	routeID domain.ID,
) (PaidRoute, error) {
	if err := service.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return PaidRoute{}, err
	}
	return service.ownedRoute(ctx, sellerID, routeID)
}

// ValidateSellerRoute computes current publication checks for an authenticated seller.
func (service *Service) ValidateSellerRoute(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	routeID domain.ID,
) (RouteValidationResult, error) {
	if err := service.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return RouteValidationResult{}, err
	}
	return service.ValidateRouteForIntegration(ctx, sellerID, routeID)
}

// PublishSellerRoute validates and publishes or resumes one seller route.
func (service *Service) PublishSellerRoute(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	routeID domain.ID,
	request RouteVersionRequest,
) (PaidRoute, error) {
	if err := service.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return PaidRoute{}, err
	}
	route, err := service.PublishRouteForIntegration(
		ctx,
		sellerID,
		routeID,
		request.ExpectedVersion,
	)
	if err != nil {
		return PaidRoute{}, err
	}
	if err := service.recordSellerRouteLifecycle(
		ctx,
		ownerSubject,
		route,
		audit.ActionRoutePublished,
	); err != nil {
		return PaidRoute{}, err
	}
	return route, nil
}

// PauseSellerRoute stops a published route through a guarded seller mutation.
func (service *Service) PauseSellerRoute(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	routeID domain.ID,
	request RouteVersionRequest,
) (PaidRoute, error) {
	return service.mutateSellerRouteLifecycle(
		ctx,
		ownerSubject,
		sellerID,
		routeID,
		request.ExpectedVersion,
		audit.ActionRoutePaused,
		func(route *PaidRoute, changedAt domain.Timestamp) error {
			return route.Pause(changedAt)
		},
	)
}

// ArchiveSellerRoute permanently retires a non-published seller route.
func (service *Service) ArchiveSellerRoute(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	routeID domain.ID,
	request RouteVersionRequest,
) (PaidRoute, error) {
	return service.mutateSellerRouteLifecycle(
		ctx,
		ownerSubject,
		sellerID,
		routeID,
		request.ExpectedVersion,
		audit.ActionRouteArchived,
		func(route *PaidRoute, changedAt domain.Timestamp) error {
			return route.Archive(changedAt)
		},
	)
}

// EmergencyDisableSellerRoute immediately stops a published seller route.
func (service *Service) EmergencyDisableSellerRoute(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	routeID domain.ID,
	request RouteVersionRequest,
) (PaidRoute, error) {
	return service.mutateSellerRouteLifecycle(
		ctx,
		ownerSubject,
		sellerID,
		routeID,
		request.ExpectedVersion,
		audit.ActionRouteEmergencyDisabled,
		func(route *PaidRoute, changedAt domain.Timestamp) error {
			return route.EmergencyDisable(changedAt)
		},
	)
}

// mutateSellerRouteLifecycle applies one authorized non-publication transition.
func (service *Service) mutateSellerRouteLifecycle(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	routeID domain.ID,
	expectedVersion uint64,
	action audit.Action,
	transition func(*PaidRoute, domain.Timestamp) error,
) (PaidRoute, error) {
	if err := service.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return PaidRoute{}, err
	}
	route, err := service.ownedRoute(ctx, sellerID, routeID)
	if err != nil {
		return PaidRoute{}, err
	}
	if err := transition(
		&route,
		domain.NewTimestamp(service.clock.Now()),
	); err != nil {
		return PaidRoute{}, err
	}
	if err := service.repository.UpdateRoute(
		ctx,
		route,
		expectedVersion,
	); err != nil {
		return PaidRoute{}, err
	}
	if err := service.recordSellerRouteLifecycle(
		ctx,
		ownerSubject,
		route,
		action,
	); err != nil {
		return PaidRoute{}, err
	}
	return route, nil
}

// recordSellerRouteLifecycle appends safe lifecycle metadata to seller history.
func (service *Service) recordSellerRouteLifecycle(
	ctx context.Context,
	ownerSubject string,
	route PaidRoute,
	action audit.Action,
) error {
	return service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID:   route.SellerID,
		ActorType:  audit.ActorTypeSellerUser,
		ActorID:    ownerSubject,
		Action:     action,
		TargetType: audit.TargetTypePaidRoute,
		TargetID:   route.RouteID.String(),
		Outcome:    audit.OutcomeSucceeded,
		ChangedFields: []string{
			"lifecycleStatus",
			"enabled",
		},
	})
}

// ConfigureStorefrontForIntegration updates the credential-bound seller.
func (service *Service) ConfigureStorefrontForIntegration(
	ctx context.Context,
	sellerID domain.ID,
	request ConfigureStorefrontRequest,
) (SellerResponse, error) {
	seller, err := service.repository.GetSeller(ctx, sellerID)
	if err != nil {
		return SellerResponse{}, err
	}
	if err := seller.Configure(
		request.Name,
		request.UpstreamBaseURL,
		domain.NewTimestamp(service.clock.Now()),
	); err != nil {
		return SellerResponse{}, err
	}
	if err := service.repository.UpdateSeller(
		ctx,
		seller,
		request.ExpectedVersion,
	); err != nil {
		return SellerResponse{}, err
	}
	return sellerResponse(seller), nil
}

// CreateDraftRouteForIntegration creates an unpublished seller route.
func (service *Service) CreateDraftRouteForIntegration(
	ctx context.Context,
	sellerID domain.ID,
	request CreateRouteRequest,
) (PaidRoute, error) {
	if _, err := service.repository.GetSeller(ctx, sellerID); err != nil {
		return PaidRoute{}, err
	}
	routeID, err := service.idGenerator.New(domain.RouteIDPrefix)
	if err != nil {
		return PaidRoute{}, err
	}
	route, err := NewDraftPaidRoute(PaidRouteParams{
		RouteID:                 routeID,
		SellerID:                sellerID,
		Method:                  request.Method,
		PathPattern:             request.PathPattern,
		Description:             request.Description,
		MIMEType:                request.MIMEType,
		Amount:                  request.Amount,
		Asset:                   request.Asset,
		Network:                 request.Network,
		PayTo:                   request.PayTo,
		ApprovalThresholdAmount: request.ApprovalThresholdAmount,
		UpstreamTimeoutSeconds:  request.UpstreamTimeoutSeconds,
		CreatedAt:               domain.NewTimestamp(service.clock.Now()),
	})
	if err != nil {
		return PaidRoute{}, err
	}
	if err := service.repository.CreateRoute(ctx, route); err != nil {
		return PaidRoute{}, err
	}
	return route, nil
}

// UpdateRoutePriceForIntegration changes one credential-bound seller route.
func (service *Service) UpdateRoutePriceForIntegration(
	ctx context.Context,
	sellerID domain.ID,
	routeID domain.ID,
	request UpdateRoutePriceRequest,
) (PaidRoute, error) {
	route, err := service.ownedRoute(ctx, sellerID, routeID)
	if err != nil {
		return PaidRoute{}, err
	}
	if err := route.ChangePrice(
		request.Amount,
		domain.NewTimestamp(service.clock.Now()),
	); err != nil {
		return PaidRoute{}, err
	}
	if err := service.repository.UpdateRoute(
		ctx,
		route,
		request.ExpectedVersion,
	); err != nil {
		return PaidRoute{}, err
	}
	return route, nil
}

// ValidateRouteForIntegration computes deterministic publication checks.
func (service *Service) ValidateRouteForIntegration(
	ctx context.Context,
	sellerID domain.ID,
	routeID domain.ID,
) (RouteValidationResult, error) {
	seller, err := service.repository.GetSeller(ctx, sellerID)
	if err != nil {
		return RouteValidationResult{}, err
	}
	route, err := service.ownedRoute(ctx, sellerID, routeID)
	if err != nil {
		return RouteValidationResult{}, err
	}
	checks := []RouteValidationCheck{
		{
			Name:    "seller_active",
			Passed:  seller.Status == SellerStatusActive,
			Message: "seller must be active",
		},
		{
			Name:    "signing_secret_configured",
			Passed:  strings.TrimSpace(seller.SigningSecretRef) != "",
			Message: "seller request signing must be configured",
		},
		{
			Name:    "route_configuration_valid",
			Passed:  len(validatePaidRouteParams(routeParams(route))) == 0,
			Message: "stored route must satisfy current validation",
		},
		{
			Name:    "route_not_archived",
			Passed:  route.LifecycleStatus != RouteLifecycleArchived,
			Message: "archived routes cannot be published",
		},
	}
	valid := true
	for _, check := range checks {
		if !check.Passed {
			valid = false
		}
	}
	return RouteValidationResult{
		SellerID: sellerID,
		RouteID:  routeID,
		Valid:    valid,
		Checks:   checks,
		Version:  route.Version,
	}, nil
}

// PublishRouteForIntegration validates and conditionally enables one draft.
func (service *Service) PublishRouteForIntegration(
	ctx context.Context,
	sellerID domain.ID,
	routeID domain.ID,
	expectedVersion uint64,
) (PaidRoute, error) {
	route, err := service.ownedRoute(ctx, sellerID, routeID)
	if err != nil {
		return PaidRoute{}, err
	}
	if route.LifecycleStatus == RouteLifecyclePublished {
		return PaidRoute{}, ErrRoutePublished
	}
	if route.LifecycleStatus == RouteLifecycleArchived {
		return PaidRoute{}, ErrRouteLifecycleTransition
	}
	validation, err := service.ValidateRouteForIntegration(ctx, sellerID, routeID)
	if err != nil {
		return PaidRoute{}, err
	}
	if !validation.Valid {
		return PaidRoute{}, ErrRouteValidation
	}
	routes, err := service.repository.ListRoutesBySeller(ctx, sellerID)
	if err != nil {
		return PaidRoute{}, err
	}
	var publishedRouteCount uint64
	for _, sellerRoute := range routes {
		sellerRoute.normalizeLifecycleStatus()
		if sellerRoute.LifecycleStatus == RouteLifecyclePublished {
			publishedRouteCount++
		}
	}
	if service.quotaEnforcer != nil {
		if err := service.quotaEnforcer.AllowPublishedRoute(
			ctx,
			sellerID,
			publishedRouteCount,
		); err != nil {
			return PaidRoute{}, err
		}
	}
	if err := route.Publish(domain.NewTimestamp(service.clock.Now())); err != nil {
		return PaidRoute{}, err
	}
	if err := service.repository.UpdateRoute(
		ctx,
		route,
		expectedVersion,
	); err != nil {
		return PaidRoute{}, err
	}
	return route, nil
}

// ownedRoute loads one route and revalidates its seller binding.
func (service *Service) ownedRoute(
	ctx context.Context,
	sellerID domain.ID,
	routeID domain.ID,
) (PaidRoute, error) {
	route, err := service.repository.GetRoute(ctx, routeID)
	if err != nil {
		return PaidRoute{}, err
	}
	if route.SellerID != sellerID {
		return PaidRoute{}, persistence.ErrNotFound
	}
	route.normalizeLifecycleStatus()
	return route, nil
}

// routeParams reconstructs validation input from one stored route.
func routeParams(route PaidRoute) PaidRouteParams {
	return PaidRouteParams{
		RouteID:                 route.RouteID,
		SellerID:                route.SellerID,
		Method:                  route.Method,
		PathPattern:             route.PathPattern,
		Description:             route.Description,
		MIMEType:                route.MIMEType,
		Amount:                  route.Amount,
		Asset:                   route.Asset,
		Network:                 route.Network,
		PayTo:                   route.PayTo,
		ApprovalThresholdAmount: route.ApprovalThresholdAmount,
		UpstreamTimeoutSeconds:  route.UpstreamTimeoutSeconds,
		CreatedAt:               route.CreatedAt,
	}
}

// GetStorefrontManifest resolves a seller and its enabled public routes.
func (service *Service) GetStorefrontManifest(
	ctx context.Context,
	slug string,
) (StorefrontManifest, error) {
	seller, err := service.repository.ResolveSellerBySlug(ctx, slug)
	if err != nil {
		return StorefrontManifest{}, err
	}
	routes, err := service.repository.ListRoutesBySeller(ctx, seller.SellerID)
	if err != nil {
		return StorefrontManifest{}, err
	}

	enabledRoutes := make([]PaidRoute, 0, len(routes))
	for _, route := range routes {
		route.normalizeLifecycleStatus()
		if route.Enabled {
			enabledRoutes = append(enabledRoutes, route)
		}
	}
	return StorefrontManifest{
		Seller: StorefrontSeller{
			Name: seller.Name,
			Slug: seller.Slug,
		},
		Routes: enabledRoutes,
	}, nil
}

// GetStorefrontLLMSText renders deterministic plain-text agent discovery.
func (service *Service) GetStorefrontLLMSText(
	ctx context.Context,
	slug string,
) (string, error) {
	manifest, err := service.GetStorefrontManifest(ctx, slug)
	if err != nil {
		return "", err
	}

	var document strings.Builder
	document.WriteString("# ")
	document.WriteString(manifest.Seller.Name)
	document.WriteString("\\n\\nManifest: /store/")
	document.WriteString(manifest.Seller.Slug)
	document.WriteString("/manifest.json\\n\\nRoutes:\\n")
	for _, route := range manifest.Routes {
		document.WriteString("- ")
		document.WriteString(string(route.Method))
		document.WriteString(" ")
		document.WriteString(route.PathPattern)
		document.WriteString(" — ")
		document.WriteString(route.Description)
		document.WriteString(" (")
		document.WriteString(route.Amount.String())
		document.WriteString(" ")
		document.WriteString(route.Asset)
		document.WriteString(" on ")
		document.WriteString(route.Network)
		document.WriteString(")\\n")
	}
	return document.String(), nil
}

// sellerResponse removes private seller fields from public responses.
func sellerResponse(seller Seller) SellerResponse {
	return SellerResponse{
		SellerID:        seller.SellerID,
		Name:            seller.Name,
		Slug:            seller.Slug,
		UpstreamBaseURL: seller.UpstreamBaseURL,
		Status:          seller.Status,
		CreatedAt:       seller.CreatedAt,
		UpdatedAt:       seller.UpdatedAt,
		Version:         seller.Version,
	}
}

// NewSeller validates and creates a draft seller.
func NewSeller(params SellerParams) (Seller, error) {
	validationErrors := validateSellerParams(params)
	if len(validationErrors) > 0 {
		return Seller{}, validationErrors
	}

	return Seller{
		SellerID:        params.SellerID,
		OwnerSubject:    strings.TrimSpace(params.OwnerSubject),
		Slug:            params.Slug,
		Name:            strings.TrimSpace(params.Name),
		UpstreamBaseURL: normalizeUpstreamBaseURL(params.UpstreamBaseURL),
		Status:          SellerStatusDraft,
		CreatedAt:       params.CreatedAt,
		UpdatedAt:       params.CreatedAt,
		Version:         1,
	}, nil
}

// NewPaidRoute validates and creates an enabled paid route.
func NewPaidRoute(params PaidRouteParams) (PaidRoute, error) {
	return newPaidRoute(params, true)
}

// NewDraftPaidRoute validates and creates an unpublished paid route.
func NewDraftPaidRoute(params PaidRouteParams) (PaidRoute, error) {
	return newPaidRoute(params, false)
}

// newPaidRoute creates one validated route with explicit publication state.
func newPaidRoute(params PaidRouteParams, enabled bool) (PaidRoute, error) {
	validationErrors := validatePaidRouteParams(params)
	if len(validationErrors) > 0 {
		return PaidRoute{}, validationErrors
	}

	var approvalThreshold *domain.Amount
	if params.ApprovalThresholdAmount != nil {
		thresholdCopy := *params.ApprovalThresholdAmount
		approvalThreshold = &thresholdCopy
	}

	lifecycleStatus := RouteLifecycleDraft
	if enabled {
		lifecycleStatus = RouteLifecyclePublished
	}

	return PaidRoute{
		RouteID:                 params.RouteID,
		SellerID:                params.SellerID,
		Method:                  params.Method,
		PathPattern:             params.PathPattern,
		Description:             strings.TrimSpace(params.Description),
		MIMEType:                strings.TrimSpace(params.MIMEType),
		Amount:                  params.Amount,
		Asset:                   strings.TrimSpace(params.Asset),
		Network:                 strings.TrimSpace(params.Network),
		PayTo:                   strings.TrimSpace(params.PayTo),
		ApprovalThresholdAmount: approvalThreshold,
		UpstreamTimeoutSeconds:  params.UpstreamTimeoutSeconds,
		LifecycleStatus:         lifecycleStatus,
		Enabled:                 enabled,
		CreatedAt:               params.CreatedAt,
		UpdatedAt:               params.CreatedAt,
		Version:                 1,
	}, nil
}

// Configure updates seller fields that coding-agent setup may safely change.
func (seller *Seller) Configure(
	name string,
	upstreamBaseURL string,
	changedAt domain.Timestamp,
) error {
	params := SellerParams{
		SellerID:        seller.SellerID,
		OwnerSubject:    seller.OwnerSubject,
		Slug:            seller.Slug,
		Name:            name,
		UpstreamBaseURL: upstreamBaseURL,
		CreatedAt:       seller.CreatedAt,
	}
	validationErrors := validateSellerParams(params)
	if len(validationErrors) > 0 {
		return validationErrors
	}
	if changedAt.Before(seller.UpdatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	seller.Name = strings.TrimSpace(name)
	seller.UpstreamBaseURL = normalizeUpstreamBaseURL(upstreamBaseURL)
	seller.UpdatedAt = changedAt
	seller.Version++
	return nil
}

// Activate enables the seller after a signing secret has been provisioned.
func (seller *Seller) Activate(
	signingSecretRef string,
	changedAt domain.Timestamp,
) error {
	trimmedReference := strings.TrimSpace(signingSecretRef)
	if trimmedReference == "" || len(trimmedReference) > maximumSigningReferenceLength {
		return domain.NewValidationError(
			"signingSecretRef",
			"required",
			"must reference a provisioned seller signing secret",
		)
	}
	if changedAt.Before(seller.UpdatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	seller.SigningSecretRef = trimmedReference
	seller.Status = SellerStatusActive
	seller.UpdatedAt = changedAt
	seller.Version++
	return nil
}

// Suspend prevents the seller from issuing new payment challenges.
func (seller *Seller) Suspend(changedAt domain.Timestamp) error {
	if changedAt.Before(seller.UpdatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	seller.Status = SellerStatusSuspended
	seller.UpdatedAt = changedAt
	seller.Version++
	return nil
}

// ChangePrice updates the route price used by future purchase intents.
func (paidRoute *PaidRoute) ChangePrice(
	amount domain.Amount,
	changedAt domain.Timestamp,
) error {
	if amount.IsZero() {
		return domain.NewValidationError(
			"amount",
			"positive",
			"must be greater than zero",
		)
	}
	if changedAt.Before(paidRoute.UpdatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	if paidRoute.Amount == amount {
		return nil
	}
	paidRoute.Amount = amount
	paidRoute.UpdatedAt = changedAt
	paidRoute.Version++
	return nil
}

// Publish enables a validated draft route for new purchase intents.
func (paidRoute *PaidRoute) Publish(changedAt domain.Timestamp) error {
	status := paidRoute.effectiveLifecycleStatus()
	if status == RouteLifecyclePublished {
		return ErrRoutePublished
	}
	if status != RouteLifecycleDraft &&
		status != RouteLifecyclePaused &&
		status != RouteLifecycleEmergencyDisabled {
		return ErrRouteLifecycleTransition
	}
	return paidRoute.transitionLifecycle(RouteLifecyclePublished, changedAt)
}

// Pause removes a published route from new purchase flows until it is resumed.
func (paidRoute *PaidRoute) Pause(changedAt domain.Timestamp) error {
	if paidRoute.effectiveLifecycleStatus() != RouteLifecyclePublished {
		return ErrRouteLifecycleTransition
	}
	return paidRoute.transitionLifecycle(RouteLifecyclePaused, changedAt)
}

// Archive permanently retires a route after it has stopped accepting purchases.
func (paidRoute *PaidRoute) Archive(changedAt domain.Timestamp) error {
	status := paidRoute.effectiveLifecycleStatus()
	if status != RouteLifecycleDraft &&
		status != RouteLifecyclePaused &&
		status != RouteLifecycleEmergencyDisabled {
		return ErrRouteLifecycleTransition
	}
	return paidRoute.transitionLifecycle(RouteLifecycleArchived, changedAt)
}

// EmergencyDisable records an urgent stop separately from a planned pause.
func (paidRoute *PaidRoute) EmergencyDisable(changedAt domain.Timestamp) error {
	if paidRoute.effectiveLifecycleStatus() != RouteLifecyclePublished {
		return ErrRouteLifecycleTransition
	}
	return paidRoute.transitionLifecycle(
		RouteLifecycleEmergencyDisabled,
		changedAt,
	)
}

// effectiveLifecycleStatus reads legacy records through the documented compatibility rule.
func (paidRoute *PaidRoute) effectiveLifecycleStatus() RouteLifecycleStatus {
	if paidRoute.LifecycleStatus != "" {
		return paidRoute.LifecycleStatus
	}
	if paidRoute.Enabled {
		return RouteLifecyclePublished
	}
	return RouteLifecycleDraft
}

// normalizeLifecycleStatus writes the compatibility-derived state into API responses and updates.
func (paidRoute *PaidRoute) normalizeLifecycleStatus() {
	paidRoute.LifecycleStatus = paidRoute.effectiveLifecycleStatus()
	paidRoute.Enabled = paidRoute.LifecycleStatus == RouteLifecyclePublished
}

// transitionLifecycle applies one validated lifecycle change atomically in memory.
func (paidRoute *PaidRoute) transitionLifecycle(
	status RouteLifecycleStatus,
	changedAt domain.Timestamp,
) error {
	if changedAt.Before(paidRoute.UpdatedAt) {
		return domain.NewValidationError(
			"updatedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	paidRoute.LifecycleStatus = status
	paidRoute.Enabled = status == RouteLifecyclePublished
	paidRoute.UpdatedAt = changedAt
	paidRoute.Version++
	return nil
}

// validateSellerParams validates seller creation fields.
func validateSellerParams(params SellerParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.SellerID.String(), domain.SellerIDPrefix); err != nil {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("sellerId", "format", "must be a seller identifier"),
		)
	}

	ownerSubject := strings.TrimSpace(params.OwnerSubject)
	if ownerSubject == "" || len(ownerSubject) > maximumOwnerSubjectLength {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("ownerSubject", "length", "must contain 1-256 characters"),
		)
	}

	name := strings.TrimSpace(params.Name)
	if name == "" || len(name) > maximumSellerNameLength {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("name", "length", "must contain 1-120 characters"),
		)
	}

	if !sellerSlugPattern.MatchString(params.Slug) {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("slug", "format", "must contain 3-48 lowercase letters, digits, or hyphens"),
		)
	}

	if err := validateUpstreamBaseURL(params.UpstreamBaseURL); err != nil {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("upstreamBaseUrl", "format", err.Error()),
		)
	}

	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("createdAt", "required", "is required"),
		)
	}
	return validationErrors
}

// validatePaidRouteParams validates paid-route creation fields.
func validatePaidRouteParams(params PaidRouteParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.RouteID.String(), domain.RouteIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("routeId", "format", "must be a route identifier"))
	}
	if _, err := domain.ParseID(params.SellerID.String(), domain.SellerIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "format", "must be a seller identifier"))
	}
	if params.Method != RouteMethodGet && params.Method != RouteMethodPost {
		validationErrors = append(validationErrors, domain.NewValidationError("method", "supported", "must be GET or POST"))
	}
	if !routePathPattern.MatchString(params.PathPattern) {
		validationErrors = append(validationErrors, domain.NewValidationError("pathPattern", "format", "must be a literal absolute route path"))
	}

	description := strings.TrimSpace(params.Description)
	if description == "" || len(description) > maximumRouteDescriptionLength {
		validationErrors = append(validationErrors, domain.NewValidationError("description", "length", "must contain 1-500 characters"))
	}

	mimeType := strings.TrimSpace(params.MIMEType)
	if mimeType == "" || len(mimeType) > maximumMIMETypeLength {
		validationErrors = append(validationErrors, domain.NewValidationError("mimeType", "length", "must contain 1-120 characters"))
	} else if _, _, err := mime.ParseMediaType(mimeType); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("mimeType", "format", "must be a valid media type"))
	}

	if params.Amount.IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("amount", "positive", "must be greater than zero"))
	}
	if value := strings.TrimSpace(params.Asset); value == "" || len(value) > maximumAssetLength {
		validationErrors = append(validationErrors, domain.NewValidationError("asset", "length", "must contain 1-160 characters"))
	}
	if value := strings.TrimSpace(params.Network); value == "" || len(value) > maximumNetworkLength {
		validationErrors = append(validationErrors, domain.NewValidationError("network", "length", "must contain 1-80 characters"))
	}
	if value := strings.TrimSpace(params.PayTo); value == "" || len(value) > maximumPayToLength {
		validationErrors = append(validationErrors, domain.NewValidationError("payTo", "length", "must contain 1-160 characters"))
	}
	if params.UpstreamTimeoutSeconds < minimumUpstreamTimeoutSeconds ||
		params.UpstreamTimeoutSeconds > maximumUpstreamTimeoutSeconds {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("upstreamTimeoutSeconds", "range", "must be between 1 and 30 seconds"),
		)
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	return validationErrors
}

// validateUpstreamBaseURL validates the seller service origin.
func validateUpstreamBaseURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return domain.NewValidationError("upstreamBaseUrl", "url", "must be an absolute URL")
	}
	if parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return domain.NewValidationError("upstreamBaseUrl", "url", "must not contain credentials, query parameters, or fragments")
	}
	if parsedURL.Scheme == "https" {
		return nil
	}
	if parsedURL.Scheme == "http" && localDevelopmentHost(parsedURL.Hostname()) {
		return nil
	}
	return domain.NewValidationError("upstreamBaseUrl", "scheme", "must use HTTPS except for a local development host")
}

// normalizeUpstreamBaseURL removes a trailing slash from the stored origin.
func normalizeUpstreamBaseURL(rawURL string) string {
	return strings.TrimSuffix(rawURL, "/")
}

// localDevelopmentHost reports whether a host is safe for local HTTP use.
func localDevelopmentHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
