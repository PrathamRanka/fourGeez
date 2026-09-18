//go:build !agentpay_dev

package main

import "testing"

// TestProductionBuildRejectsDevelopmentSeedProfile verifies seed code cannot run.
func TestProductionBuildRejectsDevelopmentSeedProfile(t *testing.T) {
	t.Parallel()

	err := configureDevelopmentSeed(nil, developmentSeedConfig{
		Environment:    "production",
		RepositoryMode: "dynamodb",
		HTTPAddress:    ":8080",
		ProfileName:    "launch-ready",
	}, developmentSeedRepositories{}, nil)
	if err == nil {
		t.Fatal("configureDevelopmentSeed() error = nil, want production rejection")
	}
}
