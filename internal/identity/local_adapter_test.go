package identity

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
)

const testLocalIdentitySigningSecret = "test-local-identity-signing-secret-32-bytes"

func TestSignedLocalAdapterVerifiesBoundedHMACTokens(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	adapter, err := NewSignedLocalAdapter([]byte(testLocalIdentitySigningSecret), domain.FixedClock{Value: now})
	if err != nil {
		t.Fatal(err)
	}
	claims := Claims{
		Subject: "local:account-one", TokenID: "token-one", SessionID: "session-one",
		IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	rawToken := signLocalTokenForTest(t, testLocalIdentitySigningSecret, claims)

	verified, err := adapter.Verify(t.Context(), rawToken)
	if err != nil || verified != claims {
		t.Fatalf("verified = %#v, error = %v", verified, err)
	}
}

func TestSignedLocalAdapterRejectsTamperingExpiryWrongSecretAndMalformedTokens(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	adapter, err := NewSignedLocalAdapter([]byte(testLocalIdentitySigningSecret), domain.FixedClock{Value: now})
	if err != nil {
		t.Fatal(err)
	}
	validClaims := Claims{
		Subject: "local:account-one", TokenID: "token-one", SessionID: "session-one",
		IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	validToken := signLocalTokenForTest(t, testLocalIdentitySigningSecret, validClaims)
	tamperedParts := strings.Split(validToken, ".")
	tamperedParts[1] = base64.RawURLEncoding.EncodeToString([]byte("local:attacker"))

	tests := []struct {
		name     string
		rawToken string
		wantErr  error
	}{
		{name: "tampered subject", rawToken: strings.Join(tamperedParts, "."), wantErr: ErrTokenInvalid},
		{
			name: "expired",
			rawToken: signLocalTokenForTest(t, testLocalIdentitySigningSecret, Claims{
				Subject: "local:account-one", TokenID: "token-expired", SessionID: "session-expired",
				IssuedAt: now.Add(-2 * time.Hour), ExpiresAt: now,
			}),
			wantErr: ErrTokenExpired,
		},
		{name: "wrong secret", rawToken: signLocalTokenForTest(t, "different-local-identity-secret-32-bytes", validClaims), wantErr: ErrTokenInvalid},
		{name: "extra segment", rawToken: validToken + ".extra", wantErr: ErrTokenInvalid},
		{name: "oversized", rawToken: strings.Repeat("x", 4097), wantErr: ErrTokenInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := adapter.Verify(t.Context(), test.rawToken); !errors.Is(err, test.wantErr) {
				t.Fatalf("Verify() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestSignedLocalAdapterRejectsWeakSecretsAndExcessiveLifetimes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	if _, err := NewSignedLocalAdapter([]byte("too-short"), domain.FixedClock{Value: now}); err == nil {
		t.Fatal("NewSignedLocalAdapter() error = nil")
	}
	adapter, err := NewSignedLocalAdapter([]byte(testLocalIdentitySigningSecret), domain.FixedClock{Value: now})
	if err != nil {
		t.Fatal(err)
	}
	rawToken := signLocalTokenForTest(t, testLocalIdentitySigningSecret, Claims{
		Subject: "local:account-one", TokenID: "token-long", SessionID: "session-long",
		IssuedAt: now, ExpiresAt: now.Add(9 * time.Hour),
	})
	if _, err := adapter.Verify(t.Context(), rawToken); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want %v", err, ErrTokenInvalid)
	}
}

func TestLocalAdapterUsesProductionShapedClaimsAndRevocation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	adapter := NewLocalAdapter(domain.FixedClock{Value: now})
	claims := Claims{
		Subject: "local-seller", TokenID: "local-token-id", SessionID: "local-session-id", ExpiresAt: now.Add(time.Hour),
	}
	if err := adapter.Register("local-secret-token", claims); err != nil {
		t.Fatal(err)
	}
	verified, err := adapter.Verify(t.Context(), "local-secret-token")
	if err != nil || verified != claims {
		t.Fatalf("verified = %#v, error = %v", verified, err)
	}
	if _, err := adapter.Verify(t.Context(), "wrong-token"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("wrong token error = %v", err)
	}
	adapter.Remove("local-secret-token")
	if _, err := adapter.Verify(t.Context(), "local-secret-token"); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("removed token error = %v", err)
	}
}

func TestLocalAdapterRejectsExpiredOrIncompleteClaims(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		claims Claims
	}{
		{name: "missing subject", claims: Claims{TokenID: "token", SessionID: "session", ExpiresAt: now.Add(time.Hour)}},
		{name: "missing token ID", claims: Claims{Subject: "subject", SessionID: "session", ExpiresAt: now.Add(time.Hour)}},
		{name: "missing session ID", claims: Claims{Subject: "subject", TokenID: "token", ExpiresAt: now.Add(time.Hour)}},
		{name: "expired", claims: Claims{Subject: "subject", TokenID: "token", SessionID: "session", ExpiresAt: now}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			adapter := NewLocalAdapter(domain.FixedClock{Value: now})
			if err := adapter.Register("local-secret-token", test.claims); err == nil {
				t.Fatal("Register() error = nil")
			}
		})
	}
}

func signLocalTokenForTest(t *testing.T, secret string, claims Claims) string {
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
