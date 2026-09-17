package catalog

import (
	"errors"
	"testing"

	"github.com/fourgeez/agentpay/internal/domain"
)

func mustCatalogID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatalf("ParseID(%q) error = %v", raw, err)
	}
	return identifier
}

func assertCatalogValidationField(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error for %q", field)
	}

	var validationErrors domain.ValidationErrors
	if errors.As(err, &validationErrors) {
		if !validationErrors.HasField(field) {
			t.Fatalf("validation errors = %#v, want field %q", validationErrors, field)
		}
		return
	}

	var validationError domain.ValidationError
	if errors.As(err, &validationError) && validationError.Field == field {
		return
	}
	t.Fatalf("error = %#v, want validation error for %q", err, field)
}
