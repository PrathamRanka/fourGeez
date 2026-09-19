package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const sandboxResponseMIMEType = "application/json"

// Service probes the no-op seller endpoint before route publication.
type Service struct {
	catalogReader        CatalogReader
	idGenerator          domain.IDGenerator
	signer               proxy.RequestSigner
	forwarder            proxy.SellerForwarder
	clock                domain.Clock
	verificationRecorder EndpointVerificationRecorder
}

func (service *Service) SetEndpointVerificationRecorder(recorder EndpointVerificationRecorder) {
	service.verificationRecorder = recorder
}

// NewService creates a sandbox validator with explicit external boundaries.
func NewService(
	catalogReader CatalogReader,
	idGenerator domain.IDGenerator,
	signer proxy.RequestSigner,
	forwarder proxy.SellerForwarder,
	clock domain.Clock,
) *Service {
	return &Service{
		catalogReader: catalogReader,
		idGenerator:   idGenerator,
		signer:        signer,
		forwarder:     forwarder,
		clock:         clock,
	}
}

// Validate runs discovery, gating, signature, and replay checks in order.
func (service *Service) Validate(
	ctx context.Context,
	sellerID domain.ID,
	routeID domain.ID,
) (Result, error) {
	if service.catalogReader == nil || service.idGenerator == nil ||
		service.signer == nil || service.forwarder == nil || service.clock == nil {
		return Result{}, ErrDependenciesUnavailable
	}
	seller, err := service.catalogReader.GetSeller(ctx, sellerID)
	if err != nil {
		return Result{}, err
	}
	route, err := service.catalogReader.GetRoute(ctx, routeID)
	if err != nil {
		return Result{}, err
	}
	if route.SellerID != sellerID {
		return Result{}, ErrRouteOwnership
	}
	if route.Enabled {
		return Result{}, ErrRouteAlreadyPublished
	}

	discoveryPassed, err := validatesProspectiveDiscovery(seller, route)
	if err != nil {
		return Result{}, err
	}
	transactionID, err := service.idGenerator.New(domain.TransactionIDPrefix)
	if err != nil {
		return Result{}, err
	}
	body, err := json.Marshal(map[string]string{
		"schemaVersion": SchemaVersion,
		"routeId":       route.RouteID.String(),
	})
	if err != nil {
		return Result{}, err
	}
	request := sandboxForwardRequest(seller, route, body)

	unsignedResponse, err := service.forwarder.Forward(ctx, request)
	if err != nil {
		return Result{}, err
	}
	request.Signature = proxy.SignatureHeaders{
		ExecutionCapability: invalidSignatureValue,
		Transaction:         transactionID.String(),
	}
	invalidResponse, err := service.forwarder.Forward(ctx, request)
	if err != nil {
		return Result{}, err
	}
	request.Signature, err = service.signer.Sign(
		ctx,
		seller.SigningSecretRef,
		proxy.SigningInput{
			TransactionID:   transactionID,
			SellerID:        sellerID,
			RouteID:         routeID,
			Method:          request.Method,
			Path:            request.Path,
			Body:            request.Body,
			PaymentFinality: transactions.PaymentFinalityFinalized,
		},
	)
	if err != nil {
		return Result{}, err
	}
	validResponse, err := service.forwarder.Forward(ctx, request)
	if err != nil {
		return Result{}, err
	}
	replayResponse, err := service.forwarder.Forward(ctx, request)
	if err != nil {
		return Result{}, err
	}

	checks := []Check{
		{
			Name:    "discovery",
			Passed:  discoveryPassed,
			Message: "draft route must produce an unchanged storefront entry",
		},
		{
			Name:    "payment_gating",
			Passed:  unsignedResponse.StatusCode == http.StatusUnauthorized,
			Message: "unsigned requests must not reach sandbox fulfillment",
		},
		{
			Name: "signature_handling",
			Passed: invalidResponse.StatusCode == http.StatusUnauthorized &&
				validResponse.StatusCode == http.StatusOK,
			Message: "invalid signatures must fail and valid signatures must succeed",
		},
		{
			Name:    "exactly_once_fulfillment",
			Passed:  replayResponse.StatusCode == http.StatusConflict,
			Message: "an accepted transaction identifier must reject replay",
		},
	}
	result := Result{
		SchemaVersion: SchemaVersion,
		SellerID:      sellerID,
		RouteID:       routeID,
		RouteVersion:  route.Version,
		Valid:         allChecksPassed(checks),
		Checks:        checks,
	}
	if result.Valid && service.verificationRecorder != nil {
		if err := service.verificationRecorder.RecordServiceEndpointVerification(ctx, sellerID, "sandbox-validation"); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

// sandboxForwardRequest builds a temporary no-op route without mutating storage.
func sandboxForwardRequest(
	seller catalog.Seller,
	route catalog.PaidRoute,
	body []byte,
) proxy.ForwardRequest {
	sandboxRoute := route
	sandboxRoute.Method = catalog.RouteMethodPost
	sandboxRoute.PathPattern = EndpointPath
	sandboxRoute.MIMEType = sandboxResponseMIMEType
	sandboxRoute.Enabled = true
	return proxy.ForwardRequest{
		Seller:      seller,
		Route:       sandboxRoute,
		Method:      catalog.RouteMethodPost,
		Path:        EndpointPath,
		Body:        body,
		ContentType: sandboxResponseMIMEType,
	}
}

// validatesProspectiveDiscovery verifies the exact stored route is representable.
func validatesProspectiveDiscovery(
	seller catalog.Seller,
	route catalog.PaidRoute,
) (bool, error) {
	prospectiveRoute := route
	prospectiveRoute.Enabled = true
	manifest := catalog.StorefrontManifest{
		Seller: catalog.StorefrontSeller{
			Name: seller.Name,
			Slug: seller.Slug,
		},
		Routes: []catalog.PaidRoute{prospectiveRoute},
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(encoded), route.RouteID.String()) &&
		strings.Contains(string(encoded), route.PathPattern), nil
}

// allChecksPassed reports whether every required sandbox check succeeded.
func allChecksPassed(checks []Check) bool {
	for _, check := range checks {
		if !check.Passed {
			return false
		}
	}
	return true
}
