package identity

import (
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

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
