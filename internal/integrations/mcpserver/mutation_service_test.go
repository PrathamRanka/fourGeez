package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/integrations/sandbox"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

func TestMutationServiceRequiresServerIssuedConfirmationGrant(t *testing.T) {
	t.Parallel()
	clock := domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)}
	consumer := &testConfirmationConsumer{err: authorization.ErrConfirmationDenied}
	mutator := &testCatalogMutator{}
	service := NewMutationService(mutator, memory.NewIdempotencyStore(), clock, &testSandboxValidator{valid: true}, audit.NoopRecorder{}, consumer)
	principal := integrations.Principal{
		SellerID: domain.ID(testSellerID), CredentialID: domain.ID(testCredentialID),
		Scopes: []integrations.Scope{integrations.ScopeConfigure},
	}
	_, err := service.ConfigureRoute(t.Context(), principal, ConfigureRouteInput{
		IdempotencyKey: "route-create-grant", ConfirmationGrant: "mcg1.mcg_01K5D09YJ0C0M7RJM4FWQ0K9HA." + strings.Repeat("s", 43),
		ExpectedSellerVersion: 1,
		Route:                 RouteConfiguration{DisplayName: "Research", ProductSlug: "research", Method: catalog.RouteMethodPost, PathPattern: "/research", Description: "Research", MIMEType: "application/json", Amount: "100", Asset: "USDC", Network: "eip155:84532", PayTo: "0x123", UpstreamTimeoutSeconds: 20},
	})
	if !errors.Is(err, authorization.ErrConfirmationDenied) || mutator.createCalls != 0 || consumer.calls != 1 {
		t.Fatalf("ConfigureRoute() = %v, create calls=%d confirmation calls=%d", err, mutator.createCalls, consumer.calls)
	}
}

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
		&testSandboxValidator{valid: true},
		audit.NoopRecorder{},
		&testConfirmationConsumer{},
	)
	principal := integrations.Principal{
		SellerID:     domain.ID(testSellerID),
		CredentialID: domain.ID(testCredentialID),
		Scopes:       []integrations.Scope{integrations.ScopeConfigure},
	}
	input := ConfigureRouteInput{
		IdempotencyKey:        "route-create-001",
		ConfirmationGrant:     "mcg1.test.secret",
		ExpectedSellerVersion: 1,
		Route: RouteConfiguration{
			DisplayName:            "Research Report",
			ProductSlug:            "research-report",
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
	if catalogMutator.lastCreateRequest.DisplayName != "Research Report" ||
		catalogMutator.lastCreateRequest.ProductSlug != "research-report" {
		t.Fatalf("product identity request = %#v", catalogMutator.lastCreateRequest)
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

// TestMutationServiceRequiresScopeAndServerConfirmation verifies authorization.
func TestMutationServiceRequiresScopeAndServerConfirmation(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	consumer := &testConfirmationConsumer{}
	service := NewMutationService(
		&testCatalogMutator{},
		memory.NewIdempotencyStore(),
		clock,
		&testSandboxValidator{valid: true},
		audit.NoopRecorder{},
		consumer,
	)
	input := ConfigureRouteInput{
		IdempotencyKey:        "route-create-002",
		ConfirmationGrant:     "mcg1.test.secret",
		ExpectedSellerVersion: 1,
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
	consumer.err = authorization.ErrConfirmationDenied
	if _, err := service.ConfigureRoute(
		t.Context(),
		principal,
		input,
	); err == nil {
		t.Fatal("ConfigureRoute() accepted a denied confirmation grant")
	}
}

// TestMutationServiceRequiresPassingSandboxBeforePublication verifies the gate.
func TestMutationServiceRequiresPassingSandboxBeforePublication(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	}
	validator := &testSandboxValidator{valid: false}
	mutator := &testCatalogMutator{}
	service := NewMutationService(
		mutator,
		memory.NewIdempotencyStore(),
		clock,
		validator,
		audit.NoopRecorder{},
		&testConfirmationConsumer{},
	)
	principal := integrations.Principal{
		SellerID:     domain.ID(testSellerID),
		CredentialID: domain.ID(testCredentialID),
		Scopes:       []integrations.Scope{integrations.ScopePublish},
	}
	input := PublishRouteInput{
		IdempotencyKey:    "route-publish-sandbox-001",
		ConfirmationGrant: "mcg1.test.secret",
		RouteID:           testRouteID,
		ExpectedVersion:   1,
		ContractHash:      "contract-hash",
	}

	if _, err := service.PublishRoute(
		t.Context(),
		principal,
		input,
	); !errors.Is(err, ErrSandboxValidationFailed) {
		t.Fatalf("PublishRoute() error = %v, want ErrSandboxValidationFailed", err)
	}
	if mutator.publishCalls != 0 {
		t.Fatalf("publish calls = %d, want 0", mutator.publishCalls)
	}

	validator.valid = true
	input.IdempotencyKey = "route-publish-sandbox-002"
	result, err := service.PublishRoute(t.Context(), principal, input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Route == nil || !result.Route.Enabled {
		t.Fatalf("publish result = %#v", result)
	}
	if mutator.publishCalls != 1 || validator.calls != 2 {
		t.Fatalf(
			"publish/validation calls = %d/%d, want 1/2",
			mutator.publishCalls,
			validator.calls,
		)
	}
}

// TestMutationServiceRejectsSandboxResultForAnotherRouteVersion prevents races.
func TestMutationServiceRejectsSandboxResultForAnotherRouteVersion(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
	}
	service := NewMutationService(
		&testCatalogMutator{},
		memory.NewIdempotencyStore(),
		clock,
		&testSandboxValidator{valid: true, routeVersion: 2},
		audit.NoopRecorder{},
		&testConfirmationConsumer{},
	)
	principal := integrations.Principal{
		SellerID:     domain.ID(testSellerID),
		CredentialID: domain.ID(testCredentialID),
		Scopes:       []integrations.Scope{integrations.ScopePublish},
	}
	_, err := service.PublishRoute(
		t.Context(),
		principal,
		PublishRouteInput{
			IdempotencyKey:    "route-publish-sandbox-version",
			ConfirmationGrant: "mcg1.test.secret",
			RouteID:           testRouteID,
			ExpectedVersion:   1,
			ContractHash:      "contract-hash",
		},
	)
	if !errors.Is(err, ErrSandboxValidationStale) {
		t.Fatalf("PublishRoute() error = %v, want ErrSandboxValidationStale", err)
	}
}

type testCatalogMutator struct {
	createCalls       int
	publishCalls      int
	lastCreateRequest catalog.CreateRouteRequest
}

func (*testCatalogMutator) GetSellerForIntegration(_ context.Context, sellerID domain.ID) (catalog.SellerResponse, error) {
	return catalog.SellerResponse{SellerID: sellerID, Version: 1}, nil
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

type testConfirmationConsumer struct {
	calls int
	err   error
	last  authorization.ConfirmationConsumption
}

func (consumer *testConfirmationConsumer) Consume(
	_ context.Context,
	_ integrations.Principal,
	request authorization.ConfirmationConsumption,
) error {
	consumer.calls++
	consumer.last = request
	return consumer.err
}

// CreateDraftRouteForIntegration returns one unpublished route fixture.
func (mutator *testCatalogMutator) CreateDraftRouteForIntegration(
	_ context.Context,
	sellerID domain.ID,
	request catalog.CreateRouteRequest,
) (catalog.PaidRoute, error) {
	mutator.createCalls++
	mutator.lastCreateRequest = request
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
		SellerID:     sellerID,
		RouteID:      routeID,
		Valid:        true,
		Version:      1,
		ContractHash: "contract-hash",
	}, nil
}

// PublishRouteForIntegration records publication after sandbox success.
func (mutator *testCatalogMutator) PublishRouteForIntegration(
	_ context.Context,
	sellerID domain.ID,
	routeID domain.ID,
	expectedVersion uint64,
	contractHash string,
) (catalog.PaidRoute, error) {
	mutator.publishCalls++
	if contractHash != "contract-hash" {
		return catalog.PaidRoute{}, catalog.ErrRouteContractStale
	}
	return catalog.PaidRoute{
		RouteID:  routeID,
		SellerID: sellerID,
		Enabled:  true,
		Version:  expectedVersion + 1,
	}, nil
}

type testSandboxValidator struct {
	valid        bool
	routeVersion uint64
	calls        int
}

// Validate returns one deterministic sandbox result.
func (validator *testSandboxValidator) Validate(
	_ context.Context,
	sellerID domain.ID,
	routeID domain.ID,
) (sandbox.Result, error) {
	validator.calls++
	routeVersion := validator.routeVersion
	if routeVersion == 0 {
		routeVersion = 1
	}
	return sandbox.Result{
		SchemaVersion: sandbox.SchemaVersion,
		SellerID:      sellerID,
		RouteID:       routeID,
		RouteVersion:  routeVersion,
		CompletedAt:   domain.NewTimestamp(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)),
		Valid:         validator.valid,
		Checks: []sandbox.Check{
			{Name: sandbox.CheckEndpointReachability, Passed: validator.valid, Message: "Endpoint check."},
			{Name: sandbox.CheckSignedExchange, Passed: validator.valid, Message: "Signed exchange check."},
			{Name: sandbox.CheckSchemaContract, Passed: validator.valid, Message: "Schema check."},
			{Name: sandbox.CheckFulfillmentReadiness, Passed: validator.valid, Message: "Fulfillment check."},
			{Name: sandbox.CheckPaymentGating, Passed: validator.valid, Message: "Payment gating check."},
			{Name: sandbox.CheckReplayIdempotency, Passed: validator.valid, Message: "Replay check."},
		},
	}, nil
}
