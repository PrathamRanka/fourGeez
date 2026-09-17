package domain

import "strings"

// ValidationError describes one field-level domain validation failure.
type ValidationError struct {
	Field   string
	Rule    string
	Message string
}

// NewValidationError creates a field-level validation failure.
func NewValidationError(field, rule, message string) ValidationError {
	return ValidationError{Field: field, Rule: rule, Message: message}
}

// Error implements error.
func (validationError ValidationError) Error() string {
	if validationError.Field == "" {
		return validationError.Message
	}
	return validationError.Field + ": " + validationError.Message
}

// ValidationErrors contains multiple field-level failures.
type ValidationErrors []ValidationError

// Error implements error.
func (validationErrors ValidationErrors) Error() string {
	messages := make([]string, 0, len(validationErrors))
	for _, validationError := range validationErrors {
		messages = append(messages, validationError.Error())
	}
	return strings.Join(messages, "; ")
}

// HasField reports whether the collection contains an error for field.
func (validationErrors ValidationErrors) HasField(field string) bool {
	for _, validationError := range validationErrors {
		if validationError.Field == field {
			return true
		}
	}
	return false
}
