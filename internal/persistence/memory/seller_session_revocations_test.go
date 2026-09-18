package memory

import (
	"testing"
	"time"
)

func TestSellerSessionRevocationRepositoryExpiresAndNeverShortensRevocation(t *testing.T) {
	t.Parallel()

	repository := NewSellerSessionRevocationRepository()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	if err := repository.Revoke(t.Context(), "digest", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repository.Revoke(t.Context(), "digest", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	revoked, err := repository.IsRevoked(t.Context(), "digest", now.Add(30*time.Minute))
	if err != nil || !revoked {
		t.Fatalf("revoked = %t, error = %v", revoked, err)
	}
	revoked, err = repository.IsRevoked(t.Context(), "digest", now.Add(time.Hour))
	if err != nil || revoked {
		t.Fatalf("expired revoked = %t, error = %v", revoked, err)
	}
}
