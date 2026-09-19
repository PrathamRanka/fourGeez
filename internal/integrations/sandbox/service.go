package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

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
	resultRecorder       ResultRecorder
}

func (service *Service) SetEndpointVerificationRecorder(recorder EndpointVerificationRecorder) {
	service.verificationRecorder = recorder
}

// SetResultRecorder stores the latest completed verification for onboarding.
func (service *Service) SetResultRecorder(recorder ResultRecorder) {
	service.resultRecorder = recorder
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
		service.signer == nil || service.forwarder == nil || service.clock == nil ||
		service.verificationRecorder == nil || service.resultRecorder == nil {
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

	schemaPassed := validatesRouteSchemas(route)
	transactionID, err := service.idGenerator.New(domain.TransactionIDPrefix)
	if err != nil {
		return Result{}, err
	}
	body, err := json.Marshal(struct {
		SchemaVersion string             `json:"schemaVersion"`
		RouteID       domain.ID          `json:"routeId"`
		RouteVersion  uint64             `json:"routeVersion"`
		InputSchema   catalog.JSONSchema `json:"inputSchema"`
		OutputSchema  catalog.JSONSchema `json:"outputSchema"`
	}{
		SchemaVersion: SchemaVersion,
		RouteID:       route.RouteID,
		RouteVersion:  route.Version,
		InputSchema:   route.InputSchema,
		OutputSchema:  route.OutputSchema,
	})
	if err != nil {
		return Result{}, err
	}
	request := sandboxForwardRequest(seller, route, body)

	unsignedResponse, err := service.forwarder.Forward(ctx, request)
	if err != nil {
		return service.finish(ctx, sellerID, route, false, false, false, false, false, false)
	}
	request.Signature = proxy.SignatureHeaders{
		ExecutionCapability: invalidSignatureValue,
		Transaction:         transactionID.String(),
	}
	invalidResponse, err := service.forwarder.Forward(ctx, request)
	if err != nil {
		return service.finish(ctx, sellerID, route, true, false, false, false, unsignedResponse.StatusCode == http.StatusUnauthorized, false)
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
		return service.finish(ctx, sellerID, route, true, false, false, false, unsignedResponse.StatusCode == http.StatusUnauthorized, false)
	}
	tamperedRequest := request
	tamperedRequest.Body = append(append([]byte(nil), request.Body...), ' ')
	tamperedResponse, err := service.forwarder.Forward(ctx, tamperedRequest)
	if err != nil {
		return service.finish(ctx, sellerID, route, true, false, false, false, unsignedResponse.StatusCode == http.StatusUnauthorized, false)
	}
	validResponse, err := service.forwarder.Forward(ctx, request)
	if err != nil {
		return service.finish(ctx, sellerID, route, true, false, false, false, unsignedResponse.StatusCode == http.StatusUnauthorized, false)
	}
	replayResponse, err := service.forwarder.Forward(ctx, request)
	if err != nil {
		return service.finish(ctx, sellerID, route, true, false, false, false, unsignedResponse.StatusCode == http.StatusUnauthorized, false)
	}

	signedExchangePassed := invalidResponse.StatusCode == http.StatusUnauthorized &&
		(tamperedResponse.StatusCode == http.StatusUnauthorized || tamperedResponse.StatusCode == http.StatusForbidden) &&
		validResponse.StatusCode == http.StatusOK
	responseContractPassed := validatesSandboxResponse(validResponse, route)
	fulfillmentReady := validResponse.StatusCode == http.StatusOK && responseContractPassed
	return service.finish(
		ctx,
		sellerID,
		route,
		true,
		signedExchangePassed,
		schemaPassed && responseContractPassed,
		fulfillmentReady,
		unsignedResponse.StatusCode == http.StatusUnauthorized,
		replayResponse.StatusCode == http.StatusConflict,
	)
}

func (service *Service) finish(
	ctx context.Context,
	sellerID domain.ID,
	route catalog.PaidRoute,
	reachable bool,
	signedExchange bool,
	schemaContract bool,
	fulfillmentReady bool,
	paymentGating bool,
	replayIdempotency bool,
) (Result, error) {
	checks := []Check{
		verificationCheck(CheckEndpointReachability, reachable, "The configured sandbox endpoint returned a bounded response.", "AgentPay could not safely reach the configured sandbox endpoint. Check public DNS, TLS, routing, and timeout settings."),
		verificationCheck(CheckSignedExchange, signedExchange, "Invalid and body-mismatched capabilities were rejected.", "The sandbox endpoint must reject malformed or body-mismatched capabilities before accepting the exact signed request."),
		verificationCheck(CheckSchemaContract, schemaContract, "The route schemas and sandbox response match the closed versioned contract.", "Keep the route schemas closed and return only the documented sandbox response fields for this route version."),
		verificationCheck(CheckFulfillmentReadiness, fulfillmentReady, "The signed no-op readiness request completed without invoking product fulfillment.", "Return 200 application/json with ready=true from the side-effect-free sandbox endpoint."),
		verificationCheck(CheckPaymentGating, paymentGating, "Unsigned requests were rejected before sandbox fulfillment.", "Require an AgentPay execution capability before the sandbox handler runs."),
		verificationCheck(CheckReplayIdempotency, replayIdempotency, "The accepted execution capability was rejected on replay.", "Use an atomic shared replay store and return 409 for a repeated execution capability."),
	}
	result := Result{
		SchemaVersion: SchemaVersion,
		SellerID:      sellerID,
		RouteID:       route.RouteID,
		RouteVersion:  route.Version,
		CompletedAt:   domain.NewTimestamp(service.clock.Now()),
		Valid:         allChecksPassed(checks),
		Checks:        checks,
	}
	if result.Valid {
		if err := service.verificationRecorder.RecordServiceEndpointVerification(ctx, sellerID, "sandbox-validation"); err != nil {
			return Result{}, err
		}
	}
	if err := service.resultRecorder.RecordIntegrationVerification(ctx, result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func verificationCheck(name string, passed bool, successMessage, failureMessage string) Check {
	message := failureMessage
	if passed {
		message = successMessage
	}
	return Check{Name: name, Passed: passed, Message: message}
}

func validatesRouteSchemas(route catalog.PaidRoute) bool {
	_, inputErr := catalog.NormalizeClosedJSONSchema([]byte(route.InputSchema.Canonical()))
	_, outputErr := catalog.NormalizeClosedJSONSchema([]byte(route.OutputSchema.Canonical()))
	return inputErr == nil && outputErr == nil
}

func validatesSandboxResponse(response proxy.ForwardResponse, route catalog.PaidRoute) bool {
	if response.StatusCode != http.StatusOK {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(response.ContentType)
	if err != nil || mediaType != sandboxResponseMIMEType {
		return false
	}
	var body struct {
		SchemaVersion string    `json:"schemaVersion"`
		RouteID       domain.ID `json:"routeId"`
		RouteVersion  uint64    `json:"routeVersion"`
		Ready         bool      `json:"ready"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response.Body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return false
	}
	return body.SchemaVersion == SchemaVersion && body.RouteID == route.RouteID &&
		body.RouteVersion == route.Version && body.Ready
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

// allChecksPassed reports whether every required sandbox check succeeded.
func allChecksPassed(checks []Check) bool {
	for _, check := range checks {
		if !check.Passed {
			return false
		}
	}
	return true
}
