package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// InputValidationIssue identifies one published-schema rule violated by a request.
type InputValidationIssue struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// ValidateJSONAgainstSchema validates one request document against AgentPay's
// bounded published JSON Schema subset. An empty body is the empty object.
func ValidateJSONAgainstSchema(schema JSONSchema, body []byte, contentType string) []InputValidationIssue {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		trimmed = []byte("{}")
	} else if mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])); mediaType != "application/json" {
		return []InputValidationIssue{{Field: "$", Rule: "contentType", Message: "Request body must use application/json."}}
	}

	var document any
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return []InputValidationIssue{{Field: "$", Rule: "json", Message: "Request body must be valid JSON."}}
	}
	var schemaNode map[string]any
	if err := json.Unmarshal([]byte(schema.Canonical()), &schemaNode); err != nil {
		return []InputValidationIssue{{Field: "$", Rule: "schema", Message: "Published input schema is invalid."}}
	}
	return validateJSONValue(schemaNode, document, "")
}

func validateJSONValue(schema map[string]any, value any, path string) []InputValidationIssue {
	field := displayValidationPath(path)
	if constValue, exists := schema["const"]; exists && !jsonValuesEqual(value, constValue) {
		return []InputValidationIssue{{Field: field, Rule: "const", Message: fmt.Sprintf("%s must match the published value.", field)}}
	}
	if enumValues, exists := schema["enum"].([]any); exists {
		matched := false
		for _, enumValue := range enumValues {
			if jsonValuesEqual(value, enumValue) {
				matched = true
				break
			}
		}
		if !matched {
			return []InputValidationIssue{{Field: field, Rule: "enum", Message: fmt.Sprintf("%s must be one of the published choices.", field)}}
		}
	}

	issues := validateCompositions(schema, value, path)
	typeName, _ := schema["type"].(string)
	if typeName != "" && !matchesJSONType(typeName, value) {
		return append(issues, InputValidationIssue{Field: field, Rule: "type", Message: fmt.Sprintf("%s must be %s.", field, typeLabel(typeName))})
	}

	switch typed := value.(type) {
	case map[string]any:
		properties, _ := schema["properties"].(map[string]any)
		if required, ok := schema["required"].([]any); ok {
			for _, requiredValue := range required {
				name, _ := requiredValue.(string)
				if _, exists := typed[name]; !exists {
					requiredPath := joinValidationPath(path, name)
					issues = append(issues, InputValidationIssue{Field: requiredPath, Rule: "required", Message: fmt.Sprintf("%s is required.", requiredPath)})
				}
			}
		}
		unknown := make([]string, 0)
		for name := range typed {
			if _, exists := properties[name]; !exists {
				unknown = append(unknown, name)
			}
		}
		sort.Strings(unknown)
		for _, name := range unknown {
			unknownPath := joinValidationPath(path, name)
			issues = append(issues, InputValidationIssue{Field: unknownPath, Rule: "unknown", Message: fmt.Sprintf("%s is not accepted by this product.", unknownPath)})
		}
		propertyNames := make([]string, 0, len(properties))
		for name := range properties {
			propertyNames = append(propertyNames, name)
		}
		sort.Strings(propertyNames)
		for _, name := range propertyNames {
			propertyValue, exists := typed[name]
			if !exists {
				continue
			}
			propertySchema, _ := properties[name].(map[string]any)
			issues = append(issues, validateJSONValue(propertySchema, propertyValue, joinValidationPath(path, name))...)
		}
	case []any:
		if minimum, ok := schemaInteger(schema["minItems"]); ok && len(typed) < minimum {
			issues = append(issues, InputValidationIssue{Field: field, Rule: "minItems", Message: fmt.Sprintf("%s must contain at least %d items.", field, minimum)})
		}
		if maximum, ok := schemaInteger(schema["maxItems"]); ok && len(typed) > maximum {
			issues = append(issues, InputValidationIssue{Field: field, Rule: "maxItems", Message: fmt.Sprintf("%s must contain at most %d items.", field, maximum)})
		}
		if itemSchema, ok := schema["items"].(map[string]any); ok {
			for index, item := range typed {
				issues = append(issues, validateJSONValue(itemSchema, item, fmt.Sprintf("%s[%d]", path, index))...)
			}
		}
	case string:
		length := utf8.RuneCountInString(typed)
		if minimum, ok := schemaInteger(schema["minLength"]); ok && length < minimum {
			issues = append(issues, InputValidationIssue{Field: field, Rule: "minLength", Message: fmt.Sprintf("%s must be at least %d characters.", field, minimum)})
		}
		if maximum, ok := schemaInteger(schema["maxLength"]); ok && length > maximum {
			issues = append(issues, InputValidationIssue{Field: field, Rule: "maxLength", Message: fmt.Sprintf("%s must be at most %d characters.", field, maximum)})
		}
		if pattern, ok := schema["pattern"].(string); ok {
			compiled, err := regexp.Compile(pattern)
			if err != nil || !compiled.MatchString(typed) {
				issues = append(issues, InputValidationIssue{Field: field, Rule: "pattern", Message: fmt.Sprintf("%s has an invalid format.", field)})
			}
		}
		if format, ok := schema["format"].(string); ok && !matchesStringFormat(format, typed) {
			issues = append(issues, InputValidationIssue{Field: field, Rule: "format", Message: fmt.Sprintf("%s must be a valid %s.", field, format)})
		}
	case json.Number:
		if minimum, ok := schemaNumber(schema["minimum"]); ok && compareJSONNumber(typed, minimum) < 0 {
			issues = append(issues, InputValidationIssue{Field: field, Rule: "minimum", Message: fmt.Sprintf("%s must be at least %s.", field, minimum.String())})
		}
		if maximum, ok := schemaNumber(schema["maximum"]); ok && compareJSONNumber(typed, maximum) > 0 {
			issues = append(issues, InputValidationIssue{Field: field, Rule: "maximum", Message: fmt.Sprintf("%s must be at most %s.", field, maximum.String())})
		}
	}
	return issues
}

func validateCompositions(schema map[string]any, value any, path string) []InputValidationIssue {
	field := displayValidationPath(path)
	var issues []InputValidationIssue
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		alternatives, ok := schema[keyword].([]any)
		if !ok {
			continue
		}
		validCount := 0
		var allIssues []InputValidationIssue
		for _, alternative := range alternatives {
			alternativeSchema, _ := alternative.(map[string]any)
			alternativeIssues := validateJSONValue(alternativeSchema, value, path)
			if len(alternativeIssues) == 0 {
				validCount++
			}
			allIssues = append(allIssues, alternativeIssues...)
		}
		switch keyword {
		case "allOf":
			issues = append(issues, allIssues...)
		case "anyOf":
			if validCount == 0 {
				issues = append(issues, InputValidationIssue{Field: field, Rule: keyword, Message: fmt.Sprintf("%s does not match an accepted option.", field)})
			}
		case "oneOf":
			if validCount != 1 {
				issues = append(issues, InputValidationIssue{Field: field, Rule: keyword, Message: fmt.Sprintf("%s must match exactly one accepted option.", field)})
			}
		}
	}
	return issues
}

func matchesJSONType(typeName string, value any) bool {
	switch typeName {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(json.Number)
		return ok
	case "integer":
		number, ok := value.(json.Number)
		return ok && !strings.ContainsAny(number.String(), ".eE")
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	default:
		return false
	}
}

func typeLabel(typeName string) string {
	if typeName == "object" || typeName == "array" {
		return "a JSON " + typeName
	}
	return "a " + typeName
}

func joinValidationPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func displayValidationPath(path string) string {
	if path == "" {
		return "Request body"
	}
	return path
}

func schemaInteger(value any) (int, bool) {
	number, ok := value.(float64)
	return int(number), ok
}

func schemaNumber(value any) (*big.Rat, bool) {
	var raw string
	switch typed := value.(type) {
	case float64:
		raw = fmt.Sprintf("%v", typed)
	case json.Number:
		raw = typed.String()
	default:
		return nil, false
	}
	number, ok := new(big.Rat).SetString(raw)
	return number, ok
}

func compareJSONNumber(value json.Number, limit *big.Rat) int {
	number, ok := new(big.Rat).SetString(value.String())
	if !ok {
		return 0
	}
	return number.Cmp(limit)
}

func jsonValuesEqual(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func matchesStringFormat(format, value string) bool {
	switch format {
	case "email":
		address, err := mail.ParseAddress(value)
		return err == nil && address.Address == value
	case "uri":
		parsed, err := url.ParseRequestURI(value)
		return err == nil && parsed.IsAbs()
	case "date-time":
		_, err := time.Parse(time.RFC3339, value)
		return err == nil
	default:
		return true
	}
}
