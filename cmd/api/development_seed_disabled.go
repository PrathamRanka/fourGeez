//go:build !agentpay_dev

package main

import (
	"errors"
	"net/http"

	"github.com/fourgeez/agentpay/internal/evidence"
)

var errDevelopmentSeedDisabled = errors.New(
	"development seed profile requires an agentpay_dev build and cannot run in production",
)

// configureDevelopmentSeed rejects every seed request in production builds.
func configureDevelopmentSeed(
	_ *http.ServeMux,
	config developmentSeedConfig,
	_ developmentSeedRepositories,
	_ evidence.Signer,
) error {
	if config.ProfileName != "" {
		return errDevelopmentSeedDisabled
	}
	return nil
}
