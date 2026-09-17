package agents

import (
	"errors"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

var (
	// ErrBudgetExceeded reports a purchase ceiling above the buyer budget.
	ErrBudgetExceeded = errors.New("buyer budget exceeded")
	// ErrMaximumPriceExceeded reports a purchase ceiling above per-route policy.
	ErrMaximumPriceExceeded = errors.New("maximum price exceeded")
	// ErrToolCallLimitExceeded reports excessive model-requested operations.
	ErrToolCallLimitExceeded = errors.New("tool call limit exceeded")
)

// NewLimits validates all agent limits from explicit application configuration.
func NewLimits(
	budget string,
	maximumPrice string,
	invocationTimeout time.Duration,
	maximumToolCalls int,
) (Limits, error) {
	parsedBudget, err := domain.ParseAmount(budget)
	if err != nil || parsedBudget.IsZero() {
		return Limits{}, errors.New("buyer budget must be positive atomic units")
	}
	parsedMaximumPrice, err := domain.ParseAmount(maximumPrice)
	if err != nil || parsedMaximumPrice.IsZero() {
		return Limits{}, errors.New("maximum price must be positive atomic units")
	}
	if invocationTimeout <= 0 {
		return Limits{}, errors.New("model invocation timeout must be positive")
	}
	if maximumToolCalls <= 0 {
		return Limits{}, errors.New("maximum tool calls must be positive")
	}

	return Limits{
		budget:            parsedBudget,
		maximumPrice:      parsedMaximumPrice,
		invocationTimeout: invocationTimeout,
		maximumToolCalls:  maximumToolCalls,
	}, nil
}

// validateAmount rejects any proposed or authoritative amount above policy.
func (limits Limits) validateAmount(amount domain.Amount) error {
	if amount.Compare(limits.budget) > 0 {
		return ErrBudgetExceeded
	}
	if amount.Compare(limits.maximumPrice) > 0 {
		return ErrMaximumPriceExceeded
	}

	return nil
}
