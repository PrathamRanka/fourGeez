package mcpserver

func closedObject(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func mutationOutputSchema(resultProperty string, resultSchema map[string]any) map[string]any {
	return closedObject(map[string]any{
		"schemaVersion": map[string]any{"type": "string", "const": MCPToolResultSchemaVersion},
		"operation":     map[string]any{"type": "string"},
		resultProperty:  resultSchema,
	}, "schemaVersion", "operation", resultProperty)
}

func sellerOutputSchema() map[string]any {
	return closedObject(map[string]any{
		"sellerId":        map[string]any{"type": "string"},
		"name":            map[string]any{"type": "string"},
		"slug":            map[string]any{"type": "string"},
		"upstreamBaseUrl": map[string]any{"type": "string"},
		"status":          map[string]any{"type": "string"},
		"createdAt":       map[string]any{"type": "string"},
		"updatedAt":       map[string]any{"type": "string"},
		"version":         map[string]any{"type": "integer"},
	}, "sellerId", "name", "slug", "upstreamBaseUrl", "status", "createdAt", "updatedAt", "version")
}

func routeOutputSchema() map[string]any {
	return closedObject(map[string]any{
		"routeId":                 map[string]any{"type": "string"},
		"sellerId":                map[string]any{"type": "string"},
		"displayName":             map[string]any{"type": "string"},
		"productSlug":             map[string]any{"type": "string"},
		"method":                  map[string]any{"type": "string"},
		"pathPattern":             map[string]any{"type": "string"},
		"description":             map[string]any{"type": "string"},
		"mimeType":                map[string]any{"type": "string"},
		"inputSchema":             map[string]any{"type": "object"},
		"outputSchema":            map[string]any{"type": "object"},
		"amount":                  map[string]any{"type": "string"},
		"asset":                   map[string]any{"type": "string"},
		"network":                 map[string]any{"type": "string"},
		"payTo":                   map[string]any{"type": "string"},
		"approvalThresholdAmount": map[string]any{"type": []any{"string", "null"}},
		"upstreamTimeoutSeconds":  map[string]any{"type": "integer"},
		"lifecycleStatus":         map[string]any{"type": "string"},
		"enabled":                 map[string]any{"type": "boolean"},
		"createdAt":               map[string]any{"type": "string"},
		"updatedAt":               map[string]any{"type": "string"},
		"version":                 map[string]any{"type": "integer"},
	}, "routeId", "sellerId", "displayName", "productSlug", "method", "pathPattern", "description", "mimeType", "inputSchema", "outputSchema", "amount", "asset", "network", "payTo", "upstreamTimeoutSeconds", "lifecycleStatus", "enabled", "createdAt", "updatedAt", "version")
}

func validationOutputSchema() map[string]any {
	check := closedObject(map[string]any{
		"name":    map[string]any{"type": "string"},
		"passed":  map[string]any{"type": "boolean"},
		"message": map[string]any{"type": "string"},
	}, "name", "passed", "message")
	return closedObject(map[string]any{
		"sellerId":     map[string]any{"type": "string"},
		"routeId":      map[string]any{"type": "string"},
		"valid":        map[string]any{"type": "boolean"},
		"checks":       map[string]any{"type": "array", "items": check},
		"version":      map[string]any{"type": "integer"},
		"contractHash": map[string]any{"type": "string", "pattern": "^[a-f0-9]{64}$"},
	}, "sellerId", "routeId", "valid", "checks", "version", "contractHash")
}
