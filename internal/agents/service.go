package agents

// ToolDefinitions returns the fixed non-payment buyer tool surface.
func ToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "getStorefrontManifest",
			Description: "Read the published routes for one seller slug.",
			InputSchema: objectSchema(
				map[string]JSONSchema{
					"slug": stringSchema("^[a-z0-9-]+$"),
				},
				"slug",
			),
		},
		{
			Name:        "createPurchaseIntent",
			Description: "Create an immutable intent for a published route.",
			InputSchema: objectSchema(
				map[string]JSONSchema{
					"routeId":         stringSchema("^rte_[A-Za-z0-9]+$"),
					"requestBodyHash": stringSchema("^[a-f0-9]{64}$"),
					"maximumAmount":   stringSchema("^[0-9]+$"),
				},
				"routeId",
				"requestBodyHash",
				"maximumAmount",
			),
		},
		{
			Name:        "getPurchaseIntent",
			Description: "Read an immutable purchase intent.",
			InputSchema: objectSchema(
				map[string]JSONSchema{
					"intentId": stringSchema("^int_[A-Za-z0-9]+$"),
				},
				"intentId",
			),
		},
		{
			Name:        "createApprovalSession",
			Description: "Request two-person approval for an intent.",
			InputSchema: approvalSessionSchema(),
		},
		{
			Name:        "getApprovalSession",
			Description: "Read the current approval-session status.",
			InputSchema: objectSchema(
				map[string]JSONSchema{
					"sessionId": stringSchema("^aps_[A-Za-z0-9]+$"),
				},
				"sessionId",
			),
		},
	}
}

// approvalSessionSchema builds the documented two-approver input contract.
func approvalSessionSchema() JSONSchema {
	two := 2
	return objectSchema(
		map[string]JSONSchema{
			"intentId": stringSchema("^int_[A-Za-z0-9]+$"),
			"approvers": {
				Type: "array",
				Items: &JSONSchema{
					Type: "object",
					Properties: map[string]JSONSchema{
						"label": {Type: "string"},
					},
					Required:             []string{"label"},
					AdditionalProperties: false,
				},
				MinItems: &two,
				MaxItems: &two,
			},
		},
		"intentId",
		"approvers",
	)
}

// objectSchema creates a closed JSON object schema.
func objectSchema(
	properties map[string]JSONSchema,
	required ...string,
) JSONSchema {
	return JSONSchema{
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: false,
	}
}

// stringSchema creates a string schema with an optional validation pattern.
func stringSchema(pattern string) JSONSchema {
	return JSONSchema{Type: "string", Pattern: pattern}
}
