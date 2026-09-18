package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/operations"
)

// QuotaCounterRepository stores atomic monthly counters for local use.
type QuotaCounterRepository struct {
	mutex  sync.Mutex
	counts map[string]uint64
	claims map[string]struct{}
}

// NewQuotaCounterRepository creates an empty quota repository.
func NewQuotaCounterRepository() *QuotaCounterRepository {
	return &QuotaCounterRepository{
		counts: make(map[string]uint64),
		claims: make(map[string]struct{}),
	}
}

// Increment consumes one monthly quota unit without exceeding its limit.
func (repository *QuotaCounterRepository) Increment(
	_ context.Context,
	request operations.CounterRequest,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	return repository.incrementLocked(request)
}

// IncrementUnique consumes one unit only for a previously unseen source.
func (repository *QuotaCounterRepository) IncrementUnique(
	_ context.Context,
	request operations.CounterRequest,
	source string,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	claimKey := quotaCounterKey(request) + "\x00" + source
	if _, exists := repository.claims[claimKey]; exists {
		return nil
	}
	if err := repository.incrementLocked(request); err != nil {
		return err
	}
	repository.claims[claimKey] = struct{}{}
	return nil
}

// incrementLocked advances one counter while the repository mutex is held.
func (repository *QuotaCounterRepository) incrementLocked(
	request operations.CounterRequest,
) error {
	key := quotaCounterKey(request)
	if repository.counts[key] >= request.Limit {
		return domain.ErrRateLimitExceeded
	}
	repository.counts[key]++
	return nil
}

// quotaCounterKey returns the in-memory seller, month, and quota identity.
func quotaCounterKey(request operations.CounterRequest) string {
	return request.SellerID.String() + "\x00" +
		request.PeriodStart.Time().UTC().Format("2006-01") + "\x00" +
		string(request.QuotaName)
}
