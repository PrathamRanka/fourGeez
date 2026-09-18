package memory

import (
	"context"
	"sync"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

type exchangeLimitWindow struct {
	startedAt time.Time
	count     uint64
}

type ProjectKeyExchangeRateLimiter struct {
	mutex       sync.Mutex
	clock       domain.Clock
	maximum     uint64
	window      time.Duration
	credentials map[domain.ID]exchangeLimitWindow
}

func NewProjectKeyExchangeRateLimiter(clock domain.Clock, maximum uint64, window time.Duration) *ProjectKeyExchangeRateLimiter {
	return &ProjectKeyExchangeRateLimiter{clock: clock, maximum: maximum, window: window, credentials: make(map[domain.ID]exchangeLimitWindow)}
}

func (limiter *ProjectKeyExchangeRateLimiter) AllowProjectKeyExchange(_ context.Context, credentialID domain.ID) error {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	now := limiter.clock.Now().UTC()
	window := limiter.credentials[credentialID]
	if window.startedAt.IsZero() || !now.Before(window.startedAt.Add(limiter.window)) {
		window = exchangeLimitWindow{startedAt: now}
	}
	if limiter.maximum == 0 || window.count >= limiter.maximum {
		return domain.ErrRateLimitExceeded
	}
	window.count++
	limiter.credentials[credentialID] = window
	return nil
}
