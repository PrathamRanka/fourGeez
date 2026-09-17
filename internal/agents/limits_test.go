package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrockdocument "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// TestNewLimitsRejectsInvalidConfiguration verifies every limit is explicit.
func TestNewLimitsRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		budget            string
		maximumPrice      string
		invocationTimeout time.Duration
		maximumToolCalls  int
	}{
		{name: "invalid budget", budget: "1.5", maximumPrice: "100", invocationTimeout: time.Second, maximumToolCalls: 1},
		{name: "zero budget", budget: "0", maximumPrice: "100", invocationTimeout: time.Second, maximumToolCalls: 1},
		{name: "invalid maximum", budget: "100", maximumPrice: "-1", invocationTimeout: time.Second, maximumToolCalls: 1},
		{name: "zero maximum", budget: "100", maximumPrice: "0", invocationTimeout: time.Second, maximumToolCalls: 1},
		{name: "zero timeout", budget: "100", maximumPrice: "100", invocationTimeout: 0, maximumToolCalls: 1},
		{name: "zero calls", budget: "100", maximumPrice: "100", invocationTimeout: time.Second, maximumToolCalls: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if _, err := NewLimits(
				testCase.budget,
				testCase.maximumPrice,
				testCase.invocationTimeout,
				testCase.maximumToolCalls,
			); err == nil {
				t.Fatal("NewLimits() accepted invalid configuration")
			}
		})
	}
}

// TestControllerEnforcesBudgetAndMaximumPrice verifies atomic ceiling checks.
func TestControllerEnforcesBudgetAndMaximumPrice(t *testing.T) {
	t.Parallel()

	limits, err := NewLimits("1000", "500", time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		name          string
		maximumAmount string
		wantError     error
	}{
		{name: "exact maximum", maximumAmount: "500"},
		{name: "over maximum price", maximumAmount: "501", wantError: ErrMaximumPriceExceeded},
		{name: "over budget", maximumAmount: "1001", wantError: ErrBudgetExceeded},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			controller := NewController(
				&agentCatalogService{},
				&agentIntentService{},
				&agentApprovalService{},
				limits,
			)
			_, err := controller.Execute(
				t.Context(),
				"buyer-123",
				ToolCall{
					Name: "createPurchaseIntent",
					Input: map[string]any{
						"routeId":         testRouteID,
						"requestBodyHash": testBodyHash,
						"maximumAmount":   testCase.maximumAmount,
					},
				},
			)
			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("Execute() error = %v, want %v", err, testCase.wantError)
			}
		})
	}
}

// TestBedrockInvokerEnforcesTimeout verifies calls cannot outlive configuration.
func TestBedrockInvokerEnforcesTimeout(t *testing.T) {
	t.Parallel()

	limits, err := NewLimits("1000", "500", 10*time.Millisecond, 2)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeBedrockClient{
		converse: func(ctx context.Context) (*bedrockruntime.ConverseOutput, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	invoker, err := NewBedrockInvoker(client, "configured-model-id", limits)
	if err != nil {
		t.Fatal(err)
	}

	_, err = invoker.Invoke(t.Context(), ModelRequest{Prompt: "find data"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Invoke() error = %v", err)
	}
}

// TestBedrockInvokerEnforcesToolCallCount verifies bounded model actions.
func TestBedrockInvokerEnforcesToolCallCount(t *testing.T) {
	t.Parallel()

	limits, err := NewLimits("1000", "500", time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	toolName := "getStorefrontManifest"
	firstID := "tool-1"
	secondID := "tool-2"
	client := &fakeBedrockClient{
		output: &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberToolUse{
							Value: bedrocktypes.ToolUseBlock{
								Name:      &toolName,
								ToolUseId: &firstID,
								Input: bedrockdocument.NewLazyDocument(
									map[string]any{"slug": "demo-seller"},
								),
							},
						},
						&bedrocktypes.ContentBlockMemberToolUse{
							Value: bedrocktypes.ToolUseBlock{
								Name:      &toolName,
								ToolUseId: &secondID,
								Input: bedrockdocument.NewLazyDocument(
									map[string]any{"slug": "other-seller"},
								),
							},
						},
					},
				},
			},
		},
	}
	invoker, err := NewBedrockInvoker(client, "configured-model-id", limits)
	if err != nil {
		t.Fatal(err)
	}

	_, err = invoker.Invoke(t.Context(), ModelRequest{Prompt: "find data"})
	if !errors.Is(err, ErrToolCallLimitExceeded) {
		t.Fatalf("Invoke() error = %v", err)
	}
}

// testAgentLimits returns explicit safe limits for focused unit tests.
func testAgentLimits(t *testing.T) Limits {
	t.Helper()

	limits, err := NewLimits("10000", "5000", time.Second, 5)
	if err != nil {
		t.Fatal(err)
	}
	return limits
}
