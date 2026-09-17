package agents

import "context"

// JSONSchema describes the bounded object schemas exposed to a model.
type JSONSchema struct {
	Type                 string                `json:"type"`
	Properties           map[string]JSONSchema `json:"properties,omitempty"`
	Required             []string              `json:"required,omitempty"`
	Items                *JSONSchema           `json:"items,omitempty"`
	Pattern              string                `json:"pattern,omitempty"`
	Minimum              *int                  `json:"minimum,omitempty"`
	Maximum              *int                  `json:"maximum,omitempty"`
	MinItems             *int                  `json:"minItems,omitempty"`
	MaxItems             *int                  `json:"maxItems,omitempty"`
	AdditionalProperties bool                  `json:"additionalProperties"`
}

// ToolDefinition is one versioned model-callable AgentPay operation.
type ToolDefinition struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema JSONSchema `json:"inputSchema"`
}

// ModelRequest contains user text supplied to the buyer model.
type ModelRequest struct {
	Prompt string
}

// ToolCall is an untrusted tool request returned by the model.
type ToolCall struct {
	ID    string
	Name  string
	Input any
}

// ModelResponse contains model text and untrusted requested tool calls.
type ModelResponse struct {
	Text      string
	ToolCalls []ToolCall
}

// ModelInvoker proposes buyer actions without executing them.
type ModelInvoker interface {
	Invoke(context.Context, ModelRequest) (ModelResponse, error)
}
