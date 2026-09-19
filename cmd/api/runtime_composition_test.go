package main

import "testing"

func TestValidateRuntimeCompositionAllowsOnlyLocalMemoryRuntime(t *testing.T) {
	t.Parallel()

	if err := validateRuntimeComposition("local", "memory"); err != nil {
		t.Fatalf("validateRuntimeComposition(local, memory) error = %v", err)
	}
}

func TestValidateRuntimeCompositionRejectsUnwiredAWSRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		environment    string
		repositoryMode string
	}{
		{name: "development environment with memory persistence", environment: "dev", repositoryMode: "memory"},
		{name: "development environment with unwired dynamodb persistence", environment: "dev", repositoryMode: "dynamodb"},
		{name: "demo environment", environment: "demo", repositoryMode: "dynamodb"},
		{name: "production environment", environment: "prod", repositoryMode: "dynamodb"},
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
