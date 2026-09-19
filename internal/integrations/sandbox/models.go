package sandbox

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
)

const (
	SchemaVersion         = integrations.IntegrationVerificationSchemaVersion
	EndpointPath          = "/.well-known/agentpay/sandbox"
	invalidSignatureValue = "aW52YWxpZA=="

	CheckEndpointReachability = integrations.IntegrationVerificationCheckEndpointReachability
	CheckSignedExchange       = integrations.IntegrationVerificationCheckSignedExchange
	CheckSchemaContract       = integrations.IntegrationVerificationCheckSchemaContract
	CheckFulfillmentReadiness = integrations.IntegrationVerificationCheckFulfillmentReadiness
	CheckPaymentGating        = integrations.IntegrationVerificationCheckPaymentGating
	CheckReplayIdempotency    = integrations.IntegrationVerificationCheckReplayIdempotency
)

var (
	ErrDependenciesUnavailable = errors.New("sandbox validation dependencies are unavailable")
	ErrRouteAlreadyPublished   = errors.New("sandbox validation requires an unpublished route")
	ErrRouteOwnership          = errors.New("sandbox route does not belong to the seller")
)

type Check = integrations.IntegrationVerificationCheck
type Result = integrations.IntegrationVerificationResult

// CatalogReader loads the credential-bound seller and route.
type CatalogReader interface {
	GetSeller(context.Context, domain.ID) (catalog.Seller, error)
	GetRoute(context.Context, domain.ID) (catalog.PaidRoute, error)
}

// EndpointVerificationRecorder persists only a complete successful sandbox proof.
type EndpointVerificationRecorder interface {
	RecordServiceEndpointVerification(context.Context, domain.ID, string) error
}

// ResultRecorder persists the latest authoritative integration verification.
type ResultRecorder interface {
	RecordIntegrationVerification(context.Context, Result) error
}
