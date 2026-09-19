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
	if (environment == "dev" || environment == "demo") && repositoryMode == "dynamodb" {
		return nil
	}
	if environment == "local" {
		return errors.New("local runtime requires memory persistence")
	}
	if environment == "prod" {
		return errors.New("production runtime remains disabled until external security and legal release gates pass")
	}
	return errors.New("AWS runtime requires dev or demo environment with DynamoDB persistence")
}
