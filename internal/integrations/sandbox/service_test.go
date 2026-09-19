package sandbox

import (
	"context"
	"errors"
	"strings"
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
			{StatusCode: 403, ContentType: "application/json"},
			{StatusCode: 200, ContentType: "application/json", Body: validSandboxResponseBody()},
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
	endpointRecorder := &testEndpointRecorder{}
	resultRecorder := &testResultRecorder{}
	service.SetEndpointVerificationRecorder(endpointRecorder)
	service.SetResultRecorder(resultRecorder)

	result, err := service.Validate(
		t.Context(),
		domain.ID(testSellerID),
		domain.ID(testRouteID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || len(result.Checks) != 6 {
		t.Fatalf("validation result = %#v", result)
	}
	if result.CompletedAt.Time().IsZero() {
		t.Fatal("validation result omitted completion time")
	}
	if len(forwarder.requests) != 5 {
		t.Fatalf("forward request count = %d, want 5", len(forwarder.requests))
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
	if forwarder.requests[2].Signature != forwarder.requests[3].Signature ||
		forwarder.requests[3].Signature != forwarder.requests[4].Signature {
		t.Fatal("binding and replay probes did not reuse the accepted capability")
	}
	if string(forwarder.requests[2].Body) == string(forwarder.requests[3].Body) {
		t.Fatal("binding probe did not modify the signed request body")
	}
	if signer.input.SellerID != domain.ID(testSellerID) ||
		signer.input.RouteID != domain.ID(testRouteID) ||
		signer.input.PaymentFinality != transactions.PaymentFinalityFinalized {
		t.Fatalf("capability binding = %#v", signer.input)
	}
	if catalogReader.route.Enabled {
		t.Fatal("sandbox validation mutated the stored draft fixture")
	}
	if endpointRecorder.calls != 1 || len(resultRecorder.results) != 1 || !resultRecorder.results[0].Valid {
		t.Fatalf("recorders = endpoint:%d results:%#v", endpointRecorder.calls, resultRecorder.results)
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
				{StatusCode: 403},
				{StatusCode: 200, ContentType: "application/json", Body: validSandboxResponseBody()},
				{StatusCode: 409},
			},
			wantCheck: "payment_gating",
		},
		{
			name: "invalid signature accepted",
			responses: []proxy.ForwardResponse{
				{StatusCode: 401},
				{StatusCode: 200},
				{StatusCode: 403},
				{StatusCode: 200, ContentType: "application/json", Body: validSandboxResponseBody()},
				{StatusCode: 409},
			},
			wantCheck: "signed_exchange",
		},
		{
			name: "body binding accepted",
			responses: []proxy.ForwardResponse{
				{StatusCode: 401},
				{StatusCode: 401},
				{StatusCode: 200},
				{StatusCode: 200, ContentType: "application/json", Body: validSandboxResponseBody()},
				{StatusCode: 409},
			},
			wantCheck: "signed_exchange",
		},
		{
			name: "valid signed request rejected",
			responses: []proxy.ForwardResponse{
				{StatusCode: 401},
				{StatusCode: 401},
				{StatusCode: 403},
				{StatusCode: 401},
				{StatusCode: 409},
			},
			wantCheck: "fulfillment_readiness",
		},
		{
			name: "invalid response contract",
			responses: []proxy.ForwardResponse{
				{StatusCode: 401},
				{StatusCode: 401},
				{StatusCode: 403},
				{StatusCode: 200, ContentType: "application/json", Body: []byte(`{"ready":true,"secret":"must-not-pass"}`)},
				{StatusCode: 409},
			},
			wantCheck: "schema_contract",
		},
		{
			name: "replay accepted",
			responses: []proxy.ForwardResponse{
				{StatusCode: 401},
				{StatusCode: 401},
				{StatusCode: 403},
				{StatusCode: 200, ContentType: "application/json", Body: validSandboxResponseBody()},
				{StatusCode: 200, ContentType: "application/json"},
			},
			wantCheck: "replay_idempotency",
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
			service.SetEndpointVerificationRecorder(&testEndpointRecorder{})
			service.SetResultRecorder(&testResultRecorder{})
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

// TestServiceReportsRedactedProbeFailure verifies transport errors become actionable failed results.
func TestServiceReportsRedactedProbeFailure(t *testing.T) {
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
	service.SetEndpointVerificationRecorder(&testEndpointRecorder{})
	resultRecorder := &testResultRecorder{}
	service.SetResultRecorder(resultRecorder)
	result, err := service.Validate(
		t.Context(),
		domain.ID(testSellerID),
		domain.ID(testRouteID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid || checkPassed(result, CheckEndpointReachability) || checkPassed(result, CheckSchemaContract) {
		t.Fatalf("transport failure result = %#v", result)
	}
	for _, check := range result.Checks {
		if strings.Contains(check.Message, probeError.Error()) {
			t.Fatalf("check leaked private error: %#v", check)
		}
	}
	if len(resultRecorder.results) != 1 || resultRecorder.results[0].Valid {
		t.Fatalf("failed result was not recorded: %#v", resultRecorder.results)
	}
}

func validSandboxResponseBody() []byte {
	return []byte(`{"schemaVersion":"agentpay.sandbox.v2","routeId":"` + testRouteID + `","routeVersion":1,"ready":true}`)
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
			InputSchema:            catalog.DefaultClosedObjectSchema,
			OutputSchema:           catalog.DefaultClosedObjectSchema,
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

type testEndpointRecorder struct {
	calls int
}

func (recorder *testEndpointRecorder) RecordServiceEndpointVerification(context.Context, domain.ID, string) error {
	recorder.calls++
	return nil
}

type testResultRecorder struct {
	results []Result
}

func (recorder *testResultRecorder) RecordIntegrationVerification(_ context.Context, result Result) error {
	recorder.results = append(recorder.results, result)
	return nil
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
