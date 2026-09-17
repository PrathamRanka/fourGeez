package sandbox

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	SchemaVersion         = "agentpay.sandbox.v1"
	EndpointPath          = "/.well-known/agentpay/sandbox"
	invalidSignatureValue = "aW52YWxpZA=="
)

var (
	ErrDependenciesUnavailable = errors.New("sandbox validation dependencies are unavailable")
	ErrRouteAlreadyPublished   = errors.New("sandbox validation requires an unpublished route")
	ErrRouteOwnership          = errors.New("sandbox route does not belong to the seller")
)

// Check records one observable sandbox requirement.
type Check struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// Result contains the complete non-persistent validation outcome.
type Result struct {
	SchemaVersion string    `json:"schemaVersion"`
	SellerID      domain.ID `json:"sellerId"`
	RouteID       domain.ID `json:"routeId"`
	RouteVersion  uint64    `json:"routeVersion"`
	Valid         bool      `json:"valid"`
	Checks        []Check   `json:"checks"`
}

// CatalogReader loads the credential-bound seller and route.
type CatalogReader interface {
	GetSeller(context.Context, domain.ID) (catalog.Seller, error)
	GetRoute(context.Context, domain.ID) (catalog.PaidRoute, error)
}
