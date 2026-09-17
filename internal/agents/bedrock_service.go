package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrockdocument "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

var errBedrockModelIDRequired = errors.New("Bedrock model ID is required")

// BedrockAPI is the external model-invocation boundary used by the buyer agent.
type BedrockAPI interface {
	Converse(
		context.Context,
		*bedrockruntime.ConverseInput,
		...func(*bedrockruntime.Options),
	) (*bedrockruntime.ConverseOutput, error)
}

// BedrockInvoker proposes buyer actions through a configured Bedrock model.
type BedrockInvoker struct {
	client  BedrockAPI
	modelID string
}

// NewBedrockInvoker creates an invoker without selecting a model implicitly.
func NewBedrockInvoker(
	client BedrockAPI,
	modelID string,
) (*BedrockInvoker, error) {
	if client == nil {
		return nil, errors.New("Bedrock client is required")
	}
	if strings.TrimSpace(modelID) == "" {
		return nil, errBedrockModelIDRequired
	}

	return &BedrockInvoker{
		client:  client,
		modelID: modelID,
	}, nil
}

// Invoke sends the prompt and fixed safe tool definitions to Bedrock.
func (invoker *BedrockInvoker) Invoke(
	ctx context.Context,
	request ModelRequest,
) (ModelResponse, error) {
	toolConfiguration, err := bedrockToolConfiguration()
	if err != nil {
		return ModelResponse{}, err
	}

	output, err := invoker.client.Converse(
		ctx,
		&bedrockruntime.ConverseInput{
			ModelId: aws.String(invoker.modelID),
			Messages: []bedrocktypes.Message{
				{
					Role: bedrocktypes.ConversationRoleUser,
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberText{
							Value: request.Prompt,
						},
					},
				},
			},
			ToolConfig: toolConfiguration,
		},
	)
	if err != nil {
		return ModelResponse{}, fmt.Errorf("invoke Bedrock model: %w", err)
	}

	return parseBedrockResponse(output), nil
}

// bedrockToolConfiguration converts the closed AgentPay schemas for the SDK.
func bedrockToolConfiguration() (*bedrocktypes.ToolConfiguration, error) {
	definitions := ToolDefinitions()
	tools := make([]bedrocktypes.Tool, 0, len(definitions))

	for _, definition := range definitions {
		schema, err := schemaDocument(definition.InputSchema)
		if err != nil {
			return nil, err
		}

		tools = append(
			tools,
			&bedrocktypes.ToolMemberToolSpec{
				Value: bedrocktypes.ToolSpecification{
					Name:        aws.String(definition.Name),
					Description: aws.String(definition.Description),
					InputSchema: &bedrocktypes.ToolInputSchemaMemberJson{
						Value: bedrockdocument.NewLazyDocument(schema),
					},
				},
			},
		)
	}

	return &bedrocktypes.ToolConfiguration{Tools: tools}, nil
}

// schemaDocument removes Go-specific schema types before SDK serialization.
func schemaDocument(schema JSONSchema) (map[string]any, error) {
	encodedSchema, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal tool schema: %w", err)
	}

	var document map[string]any
	if err := json.Unmarshal(encodedSchema, &document); err != nil {
		return nil, fmt.Errorf("decode tool schema document: %w", err)
	}

	return document, nil
}

// parseBedrockResponse keeps model output untrusted and performs no tool action.
func parseBedrockResponse(output *bedrockruntime.ConverseOutput) ModelResponse {
	response := ModelResponse{}
	if output == nil {
		return response
	}

	messageOutput, ok := output.Output.(*bedrocktypes.ConverseOutputMemberMessage)
	if !ok {
		return response
	}

	for _, contentBlock := range messageOutput.Value.Content {
		switch block := contentBlock.(type) {
		case *bedrocktypes.ContentBlockMemberText:
			response.Text += block.Value
		case *bedrocktypes.ContentBlockMemberToolUse:
			toolInput := decodeToolInput(block.Value.Input)
			response.ToolCalls = append(
				response.ToolCalls,
				ToolCall{
					ID:    aws.ToString(block.Value.ToolUseId),
					Name:  aws.ToString(block.Value.Name),
					Input: toolInput,
				},
			)
		}
	}

	return response
}

// decodeToolInput converts the SDK document without trusting its contents.
func decodeToolInput(input bedrockdocument.Interface) any {
	if input == nil {
		return nil
	}

	var decodedInput any
	if err := input.UnmarshalSmithyDocument(&decodedInput); err != nil {
		return nil
	}

	return decodedInput
}
