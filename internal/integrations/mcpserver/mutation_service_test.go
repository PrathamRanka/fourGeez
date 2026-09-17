package mcpserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

// TestMutationServiceCreatesDraftRouteIdempotently verifies confirmation and replay.
func TestMutationServiceCreatesDraftRouteIdempotently(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	catalogMutator := &testCatalogMutator{}
	service := NewMutationService(
		catalogMutator,
		memory.NewIdempotencyStore(),
		clock,
	)
	principal := integrations.Principal{
		SellerID:     domain.ID(testSellerID),
		CredentialID: domain.ID(testCredentialID),
		Scopes:       []integrations.Scope{integrations.ScopeConfigure},
	}
	input := ConfigureRouteInput{
		IdempotencyKey: "route-create-001",
		Confirmation:   validConfirmation(clock.Now()),
		Route: RouteConfiguration{
			Method:                 catalog.RouteMethodPost,
			PathPattern:            "/research",
			Description:            "Research",
			MIMEType:               "application/json",
			Amount:                 "100",
			Asset:                  "USDC",
			Network:                "eip155:84532",
			PayTo:                  "0x123",
			UpstreamTimeoutSeconds: 20,
		},
	}

	first, err := service.ConfigureRoute(t.Context(), principal, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ConfigureRoute(t.Context(), principal, input)
	if err != nil {
		t.Fatal(err)
	}
	if catalogMutator.createCalls != 1 {
		t.Fatalf("create calls = %d, want 1", catalogMutator.createCalls)
	}
	if first.Route == nil || second.Route == nil ||
		first.Route.RouteID != second.Route.RouteID || first.Route.Enabled {
		t.Fatalf("results = %#v %#v", first, second)
	}

	input.Route.Amount = "200"
	if _, err := service.ConfigureRoute(
		t.Context(),
		principal,
		input,
	); !errors.Is(err, ErrMutationConflict) {
		t.Fatalf("conflicting replay error = %v", err)
	}
}

// TestMutationServiceRequiresScopeAndFreshConfirmation verifies authorization.
func TestMutationServiceRequiresScopeAndFreshConfirmation(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	service := NewMutationService(
		&testCatalogMutator{},
		memory.NewIdempotencyStore(),
		clock,
	)
	input := ConfigureRouteInput{
		IdempotencyKey: "route-create-002",
		Confirmation:   validConfirmation(clock.Now()),
	}
	principal := integrations.Principal{
		SellerID:     domain.ID(testSellerID),
		CredentialID: domain.ID(testCredentialID),
		Scopes:       []integrations.Scope{integrations.ScopeRead},
	}
	if _, err := service.ConfigureRoute(
		t.Context(),
		principal,
		input,
	); !errors.Is(err, integrations.ErrScopeDenied) {
		t.Fatalf("scope error = %v", err)
	}

	principal.Scopes = []integrations.Scope{integrations.ScopeConfigure}
	input.Confirmation.ConfirmedAt = clock.Now().Add(-11 * time.Minute).Format(time.RFC3339)
	if _, err := service.ConfigureRoute(
		t.Context(),
		principal,
		input,
	); err == nil {
		t.Fatal("ConfigureRoute() accepted stale confirmation")
	}
}

// validConfirmation creates seller-approved metadata within the allowed window.
func validConfirmation(now time.Time) Confirmation {
	return Confirmation{
		Approved:    true,
		Summary:     "Create the reviewed paid research route",
		ConfirmedAt: now.UTC().Format(time.RFC3339),
	}
}

type testCatalogMutator struct {
	createCalls int
}

// ConfigureStorefrontForIntegration returns the configured seller fixture.
func (*testCatalogMutator) ConfigureStorefrontForIntegration(
	_ context.Context,
	sellerID domain.ID,
	request catalog.ConfigureStorefrontRequest,
) (catalog.SellerResponse, error) {
	return catalog.SellerResponse{
		SellerID: sellerID,
		Name:     request.Name,
		Version:  request.ExpectedVersion + 1,
	}, nil
}

// CreateDraftRouteForIntegration returns one unpublished route fixture.
func (mutator *testCatalogMutator) CreateDraftRouteForIntegration(
	_ context.Context,
	sellerID domain.ID,
	request catalog.CreateRouteRequest,
) (catalog.PaidRoute, error) {
	mutator.createCalls++
	return catalog.PaidRoute{
		RouteID:  domain.ID(testRouteID),
		SellerID: sellerID,
		Method:   request.Method,
		Amount:   request.Amount,
		Enabled:  false,
		Version:  1,
	}, nil
}

// UpdateRoutePriceForIntegration returns an updated route fixture.
func (*testCatalogMutator) UpdateRoutePriceForIntegration(
	_ context.Context,
	sellerID domain.ID,
	routeID domain.ID,
	request catalog.UpdateRoutePriceRequest,
) (catalog.PaidRoute, error) {
	return catalog.PaidRoute{
		RouteID:  routeID,
		SellerID: sellerID,
		Amount:   request.Amount,
		Version:  request.ExpectedVersion + 1,
	}, nil
}

// ValidateRouteForIntegration returns a passing validation fixture.
func (*testCatalogMutator) ValidateRouteForIntegration(
	_ context.Context,
	sellerID domain.ID,
	routeID domain.ID,
) (catalog.RouteValidationResult, error) {
	return catalog.RouteValidationResult{
		SellerID: sellerID,
		RouteID:  routeID,
		Valid:    true,
		Version:  1,
	}, nil
}

// PublishRouteForIntegration returns an enabled route fixture.
func (*testCatalogMutator) PublishRouteForIntegration(
	_ context.Context,
	sellerID domain.ID,
	routeID domain.ID,
	expectedVersion uint64,
) (catalog.PaidRoute, error) {
	return catalog.PaidRoute{
		RouteID:  routeID,
		SellerID: sellerID,
		Enabled:  true,
		Version:  expectedVersion + 1,
	}, nil
}
