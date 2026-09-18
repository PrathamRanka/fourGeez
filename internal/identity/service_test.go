package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestServiceAuthenticatesValidatedUnrevokedSellerClaims(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		claims     Claims
		verifyErr  error
		revoked    bool
		revokeErr  error
		wantErr    error
		wantLookup bool
	}{
		{
			name: "valid claims",
			claims: Claims{
				Subject:   "cognito-subject",
				TokenID:   "token-id",
				SessionID: "session-id",
				ExpiresAt: now.Add(time.Hour),
			},
			wantLookup: true,
		},
		{name: "invalid signature", verifyErr: ErrTokenInvalid, wantErr: ErrUnauthenticated},
		{
			name:    "expired",
			claims:  Claims{Subject: "subject", TokenID: "token", SessionID: "session", ExpiresAt: now},
			wantErr: ErrTokenExpired,
		},
		{
			name:    "missing subject",
			claims:  Claims{TokenID: "token", SessionID: "session", ExpiresAt: now.Add(time.Hour)},
			wantErr: ErrTokenInvalid,
		},
		{
			name:       "revoked session",
			claims:     Claims{Subject: "subject", TokenID: "token", SessionID: "session", ExpiresAt: now.Add(time.Hour)},
			revoked:    true,
			wantErr:    ErrTokenRevoked,
			wantLookup: true,
		},
		{
			name:       "revocation dependency failure",
			claims:     Claims{Subject: "subject", TokenID: "token", SessionID: "session", ExpiresAt: now.Add(time.Hour)},
			revokeErr:  errors.New("database unavailable"),
			wantErr:    ErrIdentityUnavailable,
			wantLookup: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &recordingRevocationRepository{revoked: test.revoked, err: test.revokeErr}
			service := NewService(
				stubTokenVerifier{claims: test.claims, err: test.verifyErr},
				repository,
				domain.FixedClock{Value: now},
			)

			principal, err := service.Authenticate(t.Context(), "secret-token")
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && (principal.Subject != test.claims.Subject || principal.SessionID != test.claims.SessionID) {
				t.Fatalf("principal = %#v", principal)
			}
			if (repository.lookupDigest != "") != test.wantLookup {
				t.Fatalf("revocation lookup = %q, want lookup %t", repository.lookupDigest, test.wantLookup)
			}
			if repository.lookupDigest == "session-id" {
				t.Fatal("raw session identifier was passed to persistence")
			}
		})
	}
}

func TestServiceRevokesCurrentSessionUntilTokenExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	repository := &recordingRevocationRepository{}
	service := NewService(stubTokenVerifier{}, repository, domain.FixedClock{Value: now})
	principal := Principal{Subject: "seller-subject", SessionID: "session-id", ExpiresAt: now.Add(time.Hour)}

	if err := service.Revoke(t.Context(), principal); err != nil {
		t.Fatal(err)
	}
	if repository.revokedDigest == "" || repository.revokedDigest == principal.SessionID {
		t.Fatalf("revoked digest = %q", repository.revokedDigest)
	}
	if !repository.expiresAt.Equal(principal.ExpiresAt) {
		t.Fatalf("expiry = %s, want %s", repository.expiresAt, principal.ExpiresAt)
	}
}

type stubTokenVerifier struct {
	claims Claims
	err    error
}

func (verifier stubTokenVerifier) Verify(context.Context, string) (Claims, error) {
	return verifier.claims, verifier.err
}

type recordingRevocationRepository struct {
	revoked       bool
	err           error
	lookupDigest  string
	revokedDigest string
	expiresAt     time.Time
}

func (repository *recordingRevocationRepository) IsRevoked(_ context.Context, digest string, _ time.Time) (bool, error) {
	repository.lookupDigest = digest
	return repository.revoked, repository.err
}

func (repository *recordingRevocationRepository) Revoke(_ context.Context, digest string, expiresAt time.Time) error {
	repository.revokedDigest = digest
	repository.expiresAt = expiresAt
	return repository.err
}
