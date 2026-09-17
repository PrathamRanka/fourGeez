package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

const (
	testRouteID   = "rte_01ARZ3NDEKTSV4RRFFQ69G5FAV"
	testIntentID  = "int_01ARZ3NDEKTSV4RRFFQ69G5FAW"
	testSessionID = "aps_01ARZ3NDEKTSV4RRFFQ69G5FAX"
	testBodyHash  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

// TestControllerValidatesCreateIntentInput verifies model values cross domain parsers.
func TestControllerValidatesCreateIntentInput(t *testing.T) {
	t.Parallel()

	intentService := &agentIntentService{}
	controller := NewController(
		&agentCatalogService{},
		intentService,
		&agentApprovalService{},
		testAgentLimits(t),
	)

	_, err := controller.Execute(
		t.Context(),
		"buyer-123",
		ToolCall{
			ID:   "tool-1",
			Name: "createPurchaseIntent",
			Input: map[string]any{
				"routeId":         testRouteID,
				"requestBodyHash": testBodyHash,
				"maximumAmount":   "2500",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if intentService.buyerID != "buyer-123" {
		t.Fatalf("buyer ID = %q", intentService.buyerID)
	}
	if intentService.createRequest.RouteID.String() != testRouteID {
		t.Fatalf("route ID = %q", intentService.createRequest.RouteID)
	}
	if intentService.createRequest.MaximumAmount.String() != "2500" {
		t.Fatalf("maximum amount = %q", intentService.createRequest.MaximumAmount)
	}
}

// TestControllerDispatchesEveryDeclaredTool verifies the complete safe surface.
func TestControllerDispatchesEveryDeclaredTool(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		call ToolCall
	}{
		{
			name: "storefront manifest",
			call: ToolCall{
				Name:  "getStorefrontManifest",
				Input: map[string]any{"slug": "demo-seller"},
			},
		},
		{
			name: "create purchase intent",
			call: ToolCall{
				Name: "createPurchaseIntent",
				Input: map[string]any{
					"routeId":         testRouteID,
					"requestBodyHash": testBodyHash,
					"maximumAmount":   "2500",
				},
			},
		},
		{
			name: "get purchase intent",
			call: ToolCall{
				Name:  "getPurchaseIntent",
				Input: map[string]any{"intentId": testIntentID},
			},
		},
		{
			name: "create approval session",
			call: ToolCall{
				Name: "createApprovalSession",
				Input: map[string]any{
					"intentId": testIntentID,
					"approvers": []any{
						map[string]any{"label": "Finance"},
						map[string]any{"label": "Security"},
					},
				},
			},
		},
		{
			name: "get approval session",
			call: ToolCall{
				Name:  "getApprovalSession",
				Input: map[string]any{"sessionId": testSessionID},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			controller := NewController(
				&agentCatalogService{},
				&agentIntentService{},
				&agentApprovalService{},
				testAgentLimits(t),
			)
			if _, err := controller.Execute(
				t.Context(),
				"buyer-123",
				testCase.call,
			); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestControllerRedactsApprovalSecrets verifies model results omit token fields.
func TestControllerRedactsApprovalSecrets(t *testing.T) {
	t.Parallel()

	controller := NewController(
		&agentCatalogService{},
		&agentIntentService{},
		&agentApprovalService{
			response: approvals.SessionResponse{
				Invitations: []approvals.InvitationLink{
					{Label: "Finance", URL: "https://example.test/?token=secret"},
				},
				ApprovalToken: "approval-secret",
			},
		},
		testAgentLimits(t),
	)
	result, err := controller.Execute(
		t.Context(),
		"buyer-123",
		ToolCall{
			Name:  "getApprovalSession",
			Input: map[string]any{"sessionId": testSessionID},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	encodedResult, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encodedResult), "secret") {
		t.Fatalf("model result leaked an approval secret: %s", encodedResult)
	}
}

// TestControllerRejectsUntrustedToolCalls verifies the closed model boundary.
func TestControllerRejectsUntrustedToolCalls(t *testing.T) {
	t.Parallel()

	unknownRoute := errors.New("route not found")
	testCases := []struct {
		name        string
		call        ToolCall
		intentError error
	}{
		{
			name: "unknown tool",
			call: ToolCall{
				Name:  "updatePurchaseIntent",
				Input: map[string]any{},
			},
		},
		{
			name: "unknown input field",
			call: ToolCall{
				Name: "getStorefrontManifest",
				Input: map[string]any{
					"slug":         "demo-seller",
					"walletSecret": "forbidden",
				},
			},
		},
		{
			name: "invalid domain identifier",
			call: ToolCall{
				Name: "getPurchaseIntent",
				Input: map[string]any{
					"intentId": "rte_wrong-type",
				},
			},
		},
		{
			name: "authoritative route rejection",
			call: ToolCall{
				Name: "createPurchaseIntent",
				Input: map[string]any{
					"routeId":         testRouteID,
					"requestBodyHash": testBodyHash,
					"maximumAmount":   "2500",
				},
			},
			intentError: unknownRoute,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			controller := NewController(
				&agentCatalogService{},
				&agentIntentService{createError: testCase.intentError},
				&agentApprovalService{},
				testAgentLimits(t),
			)
			_, err := controller.Execute(
				t.Context(),
				"buyer-123",
				testCase.call,
			)
			if err == nil {
				t.Fatal("Execute() accepted an untrusted tool call")
			}
		})
	}
}

type agentCatalogService struct {
	manifest catalog.StorefrontManifest
	err      error
}

// GetStorefrontManifest returns the configured test manifest.
func (service *agentCatalogService) GetStorefrontManifest(
	_ context.Context,
	_ string,
) (catalog.StorefrontManifest, error) {
	return service.manifest, service.err
}

type agentIntentService struct {
	buyerID       string
	createRequest intents.CreateIntentRequest
	createError   error
	getError      error
}

// Create records the domain request passed by the agent controller.
func (service *agentIntentService) Create(
	_ context.Context,
	buyerID string,
	request intents.CreateIntentRequest,
) (intents.PurchaseIntent, error) {
	service.buyerID = buyerID
	service.createRequest = request
	return intents.PurchaseIntent{}, service.createError
}

// Get returns the configured test result.
func (service *agentIntentService) Get(
	_ context.Context,
	_ domain.ID,
) (intents.PurchaseIntent, error) {
	return intents.PurchaseIntent{}, service.getError
}

type agentApprovalService struct {
	response approvals.SessionResponse
	err      error
}

// Create returns the configured redacted approval result.
func (service *agentApprovalService) Create(
	_ context.Context,
	_ domain.ID,
	_ approvals.CreateSessionRequest,
) (approvals.SessionResponse, error) {
	return service.response, service.err
}

// Get returns the configured redacted approval result.
func (service *agentApprovalService) Get(
	_ context.Context,
	_ domain.ID,
) (approvals.SessionResponse, error) {
	return service.response, service.err
}
