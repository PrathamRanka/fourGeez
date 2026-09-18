package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/identity"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

const testLocalIdentitySigningSecret = "test-local-identity-signing-secret-32-bytes"

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

func TestNewSellerIdentityServiceAcceptsSignedLocalTokens(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	service, err := newSellerIdentityService(sellerIdentityConfig{
		Environment: "local", LocalSigningSecret: testLocalIdentitySigningSecret,
	}, memory.NewSellerSessionRevocationRepository(), domain.FixedClock{Value: now})
	if err != nil {
		t.Fatal(err)
	}
	rawToken := signLocalTokenForTest(t, testLocalIdentitySigningSecret, identity.Claims{
		Subject: "local:unique-account", TokenID: "local-token-unique", SessionID: "local-session-unique",
		IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	})
	principal, err := service.Authenticate(t.Context(), rawToken)
	if err != nil || principal.Subject != "local:unique-account" {
		t.Fatalf("principal = %#v, error = %v", principal, err)
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

func signLocalTokenForTest(t *testing.T, secret string, claims identity.Claims) string {
	t.Helper()
	encode := func(value string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(value))
	}
	unsigned := strings.Join([]string{
		"apls1",
		encode(claims.Subject),
		encode(claims.TokenID),
		encode(claims.SessionID),
		strconv.FormatInt(claims.IssuedAt.Unix(), 10),
		strconv.FormatInt(claims.ExpiresAt.Unix(), 10),
	}, ".")
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(unsigned)); err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
