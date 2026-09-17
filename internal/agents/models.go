package agents

import (
	"context"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

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

// CatalogService exposes the same storefront use case used by HTTP clients.
type CatalogService interface {
	GetStorefrontManifest(context.Context, string) (catalog.StorefrontManifest, error)
}

// IntentService exposes the same immutable-intent use cases used by HTTP clients.
type IntentService interface {
	Create(
		context.Context,
		string,
		intents.CreateIntentRequest,
	) (intents.PurchaseIntent, error)
	Get(context.Context, domain.ID) (intents.PurchaseIntent, error)
}

// ApprovalService exposes the same approval use cases used by HTTP clients.
type ApprovalService interface {
	Create(
		context.Context,
		domain.ID,
		approvals.CreateSessionRequest,
	) (approvals.SessionResponse, error)
	Get(context.Context, domain.ID) (approvals.SessionResponse, error)
}

// ToolResult associates validated domain output with the originating model call.
type ToolResult struct {
	ToolCallID string `json:"toolCallId"`
	Value      any    `json:"value"`
}

// PurchaseIntentView is the immutable model-safe intent representation.
type PurchaseIntentView struct {
	IntentID         domain.ID                    `json:"intentId"`
	SellerID         domain.ID                    `json:"sellerId"`
	RouteID          domain.ID                    `json:"routeId"`
	RequestMethod    intents.RequestMethod        `json:"requestMethod"`
	RequestPath      string                       `json:"requestPath"`
	RequestBodyHash  intents.SHA256Digest         `json:"requestBodyHash"`
	Amount           domain.Amount                `json:"amount"`
	Asset            string                       `json:"asset"`
	Network          string                       `json:"network"`
	MaximumAmount    domain.Amount                `json:"maximumAmount"`
	RequiresApproval bool                         `json:"requiresApproval"`
	IntentHash       intents.SHA256Digest         `json:"intentHash"`
	Status           intents.PurchaseIntentStatus `json:"status"`
	ExpiresAt        domain.Timestamp             `json:"expiresAt"`
}

// ApprovalSessionView excludes invitation URLs and approval tokens from models.
type ApprovalSessionView struct {
	SessionID         domain.ID                `json:"sessionId"`
	IntentID          domain.ID                `json:"intentId"`
	IntentHash        intents.SHA256Digest     `json:"intentHash"`
	RequiredApprovals int                      `json:"requiredApprovals"`
	Decisions         []approvals.DecisionView `json:"decisions"`
	Status            approvals.SessionStatus  `json:"status"`
	ExpiresAt         domain.Timestamp         `json:"expiresAt"`
	UpdatedAt         domain.Timestamp         `json:"updatedAt"`
}
