package main

import "testing"

func TestValidateRuntimeCompositionAllowsOnlyLocalMemoryRuntime(t *testing.T) {
	t.Parallel()

	if err := validateRuntimeComposition("local", "memory"); err != nil {
		t.Fatalf("validateRuntimeComposition(local, memory) error = %v", err)
	}
}

func TestValidateRuntimeCompositionAllowsDurableAWSRuntime(t *testing.T) {
	t.Parallel()

	for _, environment := range []string{"dev", "demo"} {
		if err := validateRuntimeComposition(environment, "dynamodb"); err != nil {
			t.Fatalf("validateRuntimeComposition(%q, dynamodb) error = %v", environment, err)
		}
	}
}

func TestValidateRuntimeCompositionRejectsUnsafeRuntimePairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		environment    string
		repositoryMode string
	}{
		{name: "development environment with memory persistence", environment: "dev", repositoryMode: "memory"},
		{name: "production environment remains gated", environment: "prod", repositoryMode: "dynamodb"},
		{name: "local environment with unsupported persistence", environment: "local", repositoryMode: "dynamodb"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := validateRuntimeComposition(test.environment, test.repositoryMode); err == nil {
				t.Fatalf("validateRuntimeComposition(%q, %q) error = nil", test.environment, test.repositoryMode)
			}
		})
	}
}
