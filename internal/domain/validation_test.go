package domain

import (
	"errors"
	"testing"
)

func TestValidationErrors(t *testing.T) {
	t.Parallel()

	validationErrors := ValidationErrors{
		NewValidationError("amount", "atomic_units", "must contain decimal digits only"),
		NewValidationError("network", "required", "is required"),
	}

	if validationErrors.Error() != "amount: must contain decimal digits only; network: is required" {
		t.Fatalf("ValidationErrors.Error() = %q", validationErrors.Error())
	}
	if !validationErrors.HasField("network") {
		t.Fatal("ValidationErrors.HasField() did not find network")
	}
	if validationErrors.HasField("sellerId") {
		t.Fatal("ValidationErrors.HasField() found an absent field")
	}
}

func assertValidationError(t *testing.T, err error, field, rule string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected validation error for field %q", field)
	}
	var validationError ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("error type = %T, want ValidationError", err)
	}
	if validationError.Field != field || validationError.Rule != rule {
		t.Fatalf("validation error = %#v, want field=%q rule=%q", validationError, field, rule)
	}
}
