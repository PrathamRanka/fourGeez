package agents

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrockdocument "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// TestBedrockInvokerUsesConfiguredModelAndTools verifies invocation boundaries.
func TestBedrockInvokerUsesConfiguredModelAndTools(t *testing.T) {
	t.Parallel()

	toolName := "getStorefrontManifest"
	toolUseID := "tool-1"
	client := &fakeBedrockClient{
		output: &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Role: bedrocktypes.ConversationRoleAssistant,
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberText{Value: "I found a route."},
						&bedrocktypes.ContentBlockMemberToolUse{
							Value: bedrocktypes.ToolUseBlock{
								Name:      &toolName,
								ToolUseId: &toolUseID,
								Input: bedrockdocument.NewLazyDocument(
									map[string]any{"slug": "demo-seller"},
								),
							},
						},
					},
				},
			},
		},
	}
	invoker, err := NewBedrockInvoker(
		client,
		"configured-model-id",
		testAgentLimits(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := invoker.Invoke(
		t.Context(),
		ModelRequest{Prompt: "Find weather data"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if client.input == nil || *client.input.ModelId != "configured-model-id" {
		t.Fatalf("model ID = %#v", client.input)
	}
	if client.input.ToolConfig == nil ||
		len(client.input.ToolConfig.Tools) != len(ToolDefinitions()) {
		t.Fatalf("tool config = %#v", client.input.ToolConfig)
	}
	if response.Text != "I found a route." ||
		len(response.ToolCalls) != 1 ||
		response.ToolCalls[0].Name != toolName {
		t.Fatalf("response = %#v", response)
	}
}

// TestBedrockInvokerRequiresConfiguredModel verifies no hardcoded fallback ID.
func TestBedrockInvokerRequiresConfiguredModel(t *testing.T) {
	t.Parallel()

	if _, err := NewBedrockInvoker(
		&fakeBedrockClient{},
		"",
		testAgentLimits(t),
	); err == nil {
		t.Fatal("NewBedrockInvoker() accepted an empty model ID")
	}
}

type fakeBedrockClient struct {
	input    *bedrockruntime.ConverseInput
	output   *bedrockruntime.ConverseOutput
	err      error
	converse func(context.Context) (*bedrockruntime.ConverseOutput, error)
}

// Converse records the SDK request and returns the configured output.
func (client *fakeBedrockClient) Converse(
	ctx context.Context,
	input *bedrockruntime.ConverseInput,
	_ ...func(*bedrockruntime.Options),
) (*bedrockruntime.ConverseOutput, error) {
	client.input = input
	if client.converse != nil {
		return client.converse(ctx)
	}
	return client.output, client.err
}
