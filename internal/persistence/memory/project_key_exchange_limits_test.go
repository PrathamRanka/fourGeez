package memory

import (
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestProjectKeyExchangeRateLimiterEnforcesFixedWindow(t *testing.T) {
	t.Parallel()
	clock := &mutableMemoryClock{now: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)}
	limiter := NewProjectKeyExchangeRateLimiter(clock, 2, time.Minute)
	credentialID := mustMemoryID(t, "key_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.CredentialIDPrefix)
	if err := limiter.AllowProjectKeyExchange(t.Context(), credentialID); err != nil {
		t.Fatal(err)
	}
	if err := limiter.AllowProjectKeyExchange(t.Context(), credentialID); err != nil {
		t.Fatal(err)
	}
	if err := limiter.AllowProjectKeyExchange(t.Context(), credentialID); !errors.Is(err, domain.ErrRateLimitExceeded) {
		t.Fatalf("third attempt error = %v", err)
	}
	clock.now = clock.now.Add(time.Minute)
	if err := limiter.AllowProjectKeyExchange(t.Context(), credentialID); err != nil {
		t.Fatalf("next window error = %v", err)
	}
}

type mutableMemoryClock struct{ now time.Time }

func (clock *mutableMemoryClock) Now() time.Time { return clock.now }
