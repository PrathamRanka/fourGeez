package sandbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const (
	testSellerID      = "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	testRouteID       = "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	testTransactionID = "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7"
)

// TestServiceValidatesSandboxLifecycle verifies all pre-publication probes.
func TestServiceValidatesSandboxLifecycle(t *testing.T) {
	t.Parallel()

	forwarder := &testForwarder{
		responses: []proxy.ForwardResponse{
			{StatusCode: 401, ContentType: "application/json"},
			{StatusCode: 401, ContentType: "application/json"},
			{StatusCode: 200, ContentType: "application/json"},
			{StatusCode: 409, ContentType: "application/json"},
		},
	}
	catalogReader := validCatalogReader()
	signer := &testSigner{}
	service := NewService(
		catalogReader,
		testIDGenerator{},
		signer,
		forwarder,
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)},
	)

	result, err := service.Validate(
		t.Context(),
		domain.ID(testSellerID),
		domain.ID(testRouteID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || len(result.Checks) != 4 {
		t.Fatalf("validation result = %#v", result)
	}
	if len(forwarder.requests) != 4 {
		t.Fatalf("forward request count = %d, want 4", len(forwarder.requests))
	}
	for _, request := range forwarder.requests {
		if request.Path != EndpointPath || request.Route.PathPattern != EndpointPath {
			t.Fatalf("sandbox request path = %q/%q", request.Path, request.Route.PathPattern)
		}
		if !request.Route.Enabled {
			t.Fatal("sandbox route copy was not enabled for the guarded forwarder")
		}
	}
	if forwarder.requests[0].Signature.ExecutionCapability != "" {
		t.Fatal("payment-gating request unexpectedly carried an execution capability")
	}
	if forwarder.requests[1].Signature.ExecutionCapability != invalidSignatureValue {
		t.Fatal("invalid-capability probe did not use the fixed invalid capability")
	}
	if forwarder.requests[2].Signature != forwarder.requests[3].Signature {
		t.Fatal("replay probe did not reuse the accepted capability")
	}
	if signer.input.SellerID != domain.ID(testSellerID) ||
		signer.input.RouteID != domain.ID(testRouteID) ||
		signer.input.PaymentFinality != transactions.PaymentFinalityFinalized {
		t.Fatalf("capability binding = %#v", signer.input)
	}
	if catalogReader.route.Enabled {
		t.Fatal("sandbox validation mutated the stored draft fixture")
	}
}

// TestServiceReportsFailedChecks verifies fail-closed status evaluation.
func TestServiceReportsFailedChecks(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		responses []proxy.ForwardResponse
		wantCheck string
	}{
		{
			name: "unsigned request accepted",
			responses: []proxy.ForwardResponse{
				{StatusCode: 200},
				{StatusCode: 401},
				{StatusCode: 200},
				{StatusCode: 409},
			},
			wantCheck: "payment_gating",
		},
		{
			name: "invalid signature accepted",
			responses: []proxy.ForwardResponse{
				{StatusCode: 401},
				{StatusCode: 200},
				{StatusCode: 200},
				{StatusCode: 409},
			},
			wantCheck: "signature_handling",
		},
		{
			name: "valid signature rejected",
			responses: []proxy.ForwardResponse{
				{StatusCode: 401},
				{StatusCode: 401},
				{StatusCode: 401},
				{StatusCode: 409},
			},
			wantCheck: "signature_handling",
		},
		{
			name: "replay accepted",
			responses: []proxy.ForwardResponse{
				{StatusCode: 401},
				{StatusCode: 401},
				{StatusCode: 200},
				{StatusCode: 200},
			},
			wantCheck: "exactly_once_fulfillment",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			signer := &testSigner{}
			service := NewService(
				validCatalogReader(),
				testIDGenerator{},
				signer,
				&testForwarder{responses: testCase.responses},
				domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)},
			)
			result, err := service.Validate(
				t.Context(),
				domain.ID(testSellerID),
				domain.ID(testRouteID),
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.Valid {
				t.Fatalf("validation unexpectedly passed: %#v", result)
			}
			if checkPassed(result, testCase.wantCheck) {
				t.Fatalf("check %q unexpectedly passed: %#v", testCase.wantCheck, result)
			}
		})
	}
}

// TestServiceReturnsProbeFailure verifies infrastructure errors fail closed.
func TestServiceReturnsProbeFailure(t *testing.T) {
	t.Parallel()

	probeError := errors.New("private upstream failure")
	signer := &testSigner{}
	service := NewService(
		validCatalogReader(),
		testIDGenerator{},
		signer,
		&testForwarder{err: probeError},
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)},
	)
	if _, err := service.Validate(
		t.Context(),
		domain.ID(testSellerID),
		domain.ID(testRouteID),
	); !errors.Is(err, probeError) {
		t.Fatalf("Validate() error = %v, want probe error", err)
	}
}

// checkPassed returns one named check outcome from a validation result.
func checkPassed(result Result, name string) bool {
	for _, check := range result.Checks {
		if check.Name == name {
			return check.Passed
		}
	}
	return false
}

// validCatalogReader creates one active seller and unpublished route fixture.
func validCatalogReader() *testCatalogReader {
	createdAt := domain.NewTimestamp(
		time.Date(2026, time.September, 18, 9, 0, 0, 0, time.UTC),
	)
	return &testCatalogReader{
		seller: catalog.Seller{
			SellerID:         domain.ID(testSellerID),
			Slug:             "demo-seller",
			Name:             "Demo Seller",
			UpstreamBaseURL:  "https://seller.example",
			SigningSecretRef: "secret/demo-seller",
			Status:           catalog.SellerStatusActive,
			CreatedAt:        createdAt,
			UpdatedAt:        createdAt,
			Version:          1,
		},
		route: catalog.PaidRoute{
			RouteID:                domain.ID(testRouteID),
			SellerID:               domain.ID(testSellerID),
			Method:                 catalog.RouteMethodPost,
			PathPattern:            "/generate",
			Description:            "Generate a report",
			MIMEType:               "application/json",
			Amount:                 domain.MustParseAmount("100"),
			Asset:                  "USDC",
			Network:                "eip155:84532",
			PayTo:                  "0x123",
			UpstreamTimeoutSeconds: 20,
			Enabled:                false,
			CreatedAt:              createdAt,
			UpdatedAt:              createdAt,
			Version:                1,
		},
	}
}

type testCatalogReader struct {
	seller catalog.Seller
	route  catalog.PaidRoute
}

// GetSeller returns the exact sandbox seller fixture.
func (reader *testCatalogReader) GetSeller(
	_ context.Context,
	sellerID domain.ID,
) (catalog.Seller, error) {
	if sellerID != reader.seller.SellerID {
		return catalog.Seller{}, errors.New("seller not found")
	}
	return reader.seller, nil
}

// GetRoute returns the exact sandbox route fixture.
func (reader *testCatalogReader) GetRoute(
	_ context.Context,
	routeID domain.ID,
) (catalog.PaidRoute, error) {
	if routeID != reader.route.RouteID {
		return catalog.PaidRoute{}, errors.New("route not found")
	}
	return reader.route, nil
}

type testSigner struct {
	input proxy.SigningInput
}

type testIDGenerator struct{}

// New returns the fixed transaction identifier used by sandbox probes.
func (testIDGenerator) New(prefix domain.IDPrefix) (domain.ID, error) {
	if prefix != domain.TransactionIDPrefix {
		return "", errors.New("unexpected identifier prefix")
	}
	return domain.ID(testTransactionID), nil
}

// Sign returns deterministic accepted sandbox authentication headers.
func (signer *testSigner) Sign(
	_ context.Context,
	_ string,
	input proxy.SigningInput,
) (proxy.SignatureHeaders, error) {
	signer.input = input
	return proxy.SignatureHeaders{
		ExecutionCapability: "signed.jwt.value",
		Transaction:         input.TransactionID.String(),
	}, nil
}

type testForwarder struct {
	requests  []proxy.ForwardRequest
	responses []proxy.ForwardResponse
	err       error
}

// Forward records sandbox probes and returns the configured response sequence.
func (forwarder *testForwarder) Forward(
	_ context.Context,
	request proxy.ForwardRequest,
) (proxy.ForwardResponse, error) {
	forwarder.requests = append(forwarder.requests, request)
	if forwarder.err != nil {
		return proxy.ForwardResponse{}, forwarder.err
	}
	responseIndex := len(forwarder.requests) - 1
	return forwarder.responses[responseIndex], nil
}
