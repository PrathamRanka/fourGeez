package main

import (
	"errors"
	"strings"
)

func validateRuntimeComposition(environment, repositoryMode string) error {
	environment = strings.TrimSpace(environment)
	repositoryMode = strings.TrimSpace(repositoryMode)
	if environment == "local" && repositoryMode == "memory" {
		return nil
	}
	if environment == "local" {
		return errors.New("local runtime requires memory persistence")
	}
	return errors.New("AWS runtime is disabled until durable persistence, secret, signing, evidence, and Lambda transport adapters are configured")
}
