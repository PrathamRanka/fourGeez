package catalog

import (
	"context"
	"mime"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
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
	sellerSlugPattern = regexp.MustCompile(`^[a-z0-9-]{3,48}$`)
	routePathPattern  = regexp.MustCompile(`^/[A-Za-z0-9/_-]+$`)
)

// Service coordinates catalog domain rules with persistence boundaries.
type Service struct {
	repository  Repository
	idGenerator domain.IDGenerator
	clock       domain.Clock
}

// NewService creates the catalog application service.
func NewService(
	repository Repository,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
) *Service {
	return &Service{
		repository:  repository,
		idGenerator: idGenerator,
		clock:       clock,
	}
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
	route, err := NewPaidRoute(PaidRouteParams{
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
	return route, nil
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
	validationErrors := validatePaidRouteParams(params)
	if len(validationErrors) > 0 {
		return PaidRoute{}, validationErrors
	}

	var approvalThreshold *domain.Amount
	if params.ApprovalThresholdAmount != nil {
		thresholdCopy := *params.ApprovalThresholdAmount
		approvalThreshold = &thresholdCopy
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
		Enabled:                 true,
		CreatedAt:               params.CreatedAt,
		UpdatedAt:               params.CreatedAt,
		Version:                 1,
	}, nil
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
