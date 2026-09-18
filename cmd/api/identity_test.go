package main

import (
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/identity"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

func TestNewSellerIdentityServiceUsesLocalAdapterOnlyInLocalEnvironment(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	service, err := newSellerIdentityService(sellerIdentityConfig{
		Environment: "local", LocalToken: "local-secret", LocalSubject: "local-seller",
	}, memory.NewSellerSessionRevocationRepository(), domain.FixedClock{Value: now})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := service.Authenticate(t.Context(), "local-secret")
	if err != nil || principal.Subject != "local-seller" {
		t.Fatalf("principal = %#v, error = %v", principal, err)
	}

	_, err = newSellerIdentityService(sellerIdentityConfig{
		Environment: "prod", LocalToken: "local-secret", LocalSubject: "local-seller",
	}, memory.NewSellerSessionRevocationRepository(), domain.FixedClock{Value: now})
	if err == nil {
		t.Fatal("production identity accepted local token configuration without Cognito")
	}
}

func TestNewSellerIdentityServiceRequiresCompleteCognitoConfiguration(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	_, err := newSellerIdentityService(sellerIdentityConfig{
		Environment: "prod", AWSRegion: "us-east-1", UserPoolID: "", ClientID: "client",
	}, memory.NewSellerSessionRevocationRepository(), domain.FixedClock{Value: now})
	if err == nil || errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatalf("configuration error = %v", err)
	}
}
