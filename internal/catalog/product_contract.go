package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/gowebpki/jcs"
)

const (
	DefaultClosedObjectSchema   JSONSchema = `{"additionalProperties":false,"properties":{},"type":"object"}`
	maximumSchemaBytes                     = 16 * 1024
	publishedContractHashDomain            = "agentpay.published-route-contract.v1"
)

// JSONSchema stores one canonical, closed JSON Schema while remaining comparable.
type JSONSchema string

func (schema JSONSchema) MarshalJSON() ([]byte, error) {
	if schema == "" {
		return []byte(DefaultClosedObjectSchema), nil
	}
	return []byte(schema), nil
}

func (schema *JSONSchema) UnmarshalJSON(encoded []byte) error {
	normalized, err := NormalizeClosedJSONSchema(encoded)
	if err != nil {
		return err
	}
	*schema = normalized
	return nil
}

func (schema JSONSchema) Canonical() string {
	if schema == "" {
		return string(DefaultClosedObjectSchema)
	}
	return string(schema)
}

// NormalizeClosedJSONSchema accepts the bounded schema subset AgentPay publishes.
func NormalizeClosedJSONSchema(encoded []byte) (JSONSchema, error) {
	if len(encoded) == 0 || bytes.Equal(bytes.TrimSpace(encoded), []byte("null")) {
		return DefaultClosedObjectSchema, nil
	}
	if len(encoded) > maximumSchemaBytes {
		return "", domain.NewValidationError("schema", "size", "must not exceed 16384 bytes")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", domain.NewValidationError("schema", "json", "must be a JSON object")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "", domain.NewValidationError("schema", "json", "must contain exactly one JSON value")
	}
	root, ok := value.(map[string]any)
	if !ok {
		return "", domain.NewValidationError("schema", "object", "must be a JSON object")
	}
	if err := validateClosedSchemaNode(root, true); err != nil {
		return "", err
	}
	canonicalInput, err := json.Marshal(root)
	if err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(canonicalInput)
	if err != nil {
		return "", err
	}
	return JSONSchema(canonical), nil
}

func validateClosedSchemaNode(node map[string]any, root bool) error {
	allowed := map[string]bool{
		"type": true, "additionalProperties": true, "properties": true,
		"required": true, "description": true, "format": true, "enum": true,
		"const": true, "items": true, "minLength": true, "maxLength": true,
		"minimum": true, "maximum": true, "minItems": true, "maxItems": true,
		"pattern": true, "oneOf": true, "anyOf": true, "allOf": true,
	}
	for key := range node {
		if !allowed[key] {
			return domain.NewValidationError("schema", "keyword", "contains an unsupported schema keyword")
		}
	}
	typeName, _ := node["type"].(string)
	if root && typeName != "object" {
		return domain.NewValidationError("schema", "type", "root type must be object")
	}
	if typeName == "object" || node["properties"] != nil {
		if node["additionalProperties"] != false {
			return domain.NewValidationError("schema", "closed", "object schemas must set additionalProperties to false")
		}
		properties, ok := node["properties"].(map[string]any)
		if !ok {
			return domain.NewValidationError("schema", "properties", "object schemas must define a properties map")
		}
		for _, property := range properties {
			child, ok := property.(map[string]any)
			if !ok {
				return domain.NewValidationError("schema", "property", "each property must contain a schema object")
			}
			if err := validateClosedSchemaNode(child, false); err != nil {
				return err
			}
		}
		if required, exists := node["required"]; exists {
			items, ok := required.([]any)
			if !ok {
				return domain.NewValidationError("schema", "required", "must be an array")
			}
			seen := make(map[string]struct{}, len(items))
			for _, item := range items {
				name, ok := item.(string)
				if !ok {
					return domain.NewValidationError("schema", "required", "must contain property names")
				}
				if _, exists := properties[name]; !exists {
					return domain.NewValidationError("schema", "required", "must reference a declared property")
				}
				if _, duplicate := seen[name]; duplicate {
					return domain.NewValidationError("schema", "required", "must not contain duplicates")
				}
				seen[name] = struct{}{}
			}
		}
	}
	if items, exists := node["items"]; exists {
		child, ok := items.(map[string]any)
		if !ok {
			return domain.NewValidationError("schema", "items", "must contain a schema object")
		}
		if err := validateClosedSchemaNode(child, false); err != nil {
			return err
		}
	}
	for _, keyword := range []string{"oneOf", "anyOf", "allOf"} {
		alternatives, exists := node[keyword]
		if !exists {
			continue
		}
		items, ok := alternatives.([]any)
		if !ok || len(items) == 0 {
			return domain.NewValidationError("schema", keyword, "must contain schema objects")
		}
		for _, item := range items {
			child, ok := item.(map[string]any)
			if !ok {
				return domain.NewValidationError("schema", keyword, "must contain schema objects")
			}
			if err := validateClosedSchemaNode(child, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// PublishedContractHash binds validation and approval to one exact route version.
func PublishedContractHash(route PaidRoute) (string, error) {
	payload, err := json.Marshal(struct {
		RouteID                domain.ID   `json:"routeId"`
		SellerID               domain.ID   `json:"sellerId"`
		DisplayName            string      `json:"displayName"`
		ProductSlug            string      `json:"productSlug"`
		Method                 RouteMethod `json:"method"`
		PathPattern            string      `json:"pathPattern"`
		Description            string      `json:"description"`
		InputSchema            JSONSchema  `json:"inputSchema"`
		OutputSchema           JSONSchema  `json:"outputSchema"`
		MIMEType               string      `json:"mimeType"`
		Amount                 string      `json:"amount"`
		Asset                  string      `json:"asset"`
		Network                string      `json:"network"`
		PayTo                  string      `json:"payTo"`
		UpstreamTimeoutSeconds int         `json:"upstreamTimeoutSeconds"`
		Version                uint64      `json:"version"`
	}{route.RouteID, route.SellerID, route.DisplayName, route.ProductSlug, route.Method, route.PathPattern, route.Description, route.InputSchema, route.OutputSchema, route.MIMEType, route.Amount.String(), route.Asset, route.Network, route.PayTo, route.UpstreamTimeoutSeconds, route.Version})
	if err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(append(append([]byte(publishedContractHashDomain), 0), canonical...))
	return hex.EncodeToString(digest[:]), nil
}
