package main

import (
	"context"
	"errors"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const localEnvironment = "local"

type onboardingSellerReader interface {
	GetSeller(context.Context, domain.ID) (catalog.Seller, error)
}

type localOnboardingEntitlementRepository struct {
	repository billing.Repository
	sellers    onboardingSellerReader
	clock      domain.Clock
}

// configureOnboardingEntitlementRepository gives local and AWS development
// sellers a no-charge Starter entitlement without weakening demo/production.
func configureOnboardingEntitlementRepository(
	environment string,
	repository billing.Repository,
	sellers onboardingSellerReader,
	clock domain.Clock,
) billing.Repository {
	if environment != localEnvironment && environment != "dev" {
		return repository
	}
	return &localOnboardingEntitlementRepository{
		repository: repository,
		sellers:    sellers,
		clock:      clock,
	}
}

func (repository *localOnboardingEntitlementRepository) Get(
	ctx context.Context,
	sellerID domain.ID,
) (billing.SellerEntitlement, error) {
	entitlement, err := repository.repository.Get(ctx, sellerID)
	if err == nil {
		return entitlement, nil
	}
	if !errors.Is(err, persistence.ErrNotFound) &&
		!errors.Is(err, billing.ErrSellerEntitlementNotFound) {
		return billing.SellerEntitlement{}, err
	}
	if _, err := repository.sellers.GetSeller(ctx, sellerID); err != nil {
		return billing.SellerEntitlement{}, err
	}

	now := repository.clock.Now().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)
	entitlement, reconciliation, err := billing.ReconcileSellerEntitlement(
		nil,
		billing.EntitlementCandidate{
			SellerID:           sellerID,
			PlanID:             billing.PlanStarter,
			PlanVersion:        1,
			Status:             billing.EntitlementStatusActive,
			BillingPeriodStart: domain.NewTimestamp(periodStart),
			BillingPeriodEnd:   domain.NewTimestamp(periodEnd),
			AccessEndsAt:       domain.NewTimestamp(periodEnd),
			Source:             billing.EntitlementSourceLocal,
			Provider:           billing.EntitlementProviderLocal,
		},
		domain.NewTimestamp(now),
	)
	if err != nil {
		return billing.SellerEntitlement{}, err
	}
	if err := repository.repository.Apply(ctx, entitlement, reconciliation, 0); err != nil {
		if errors.Is(err, persistence.ErrConditionFailed) {
			return repository.repository.Get(ctx, sellerID)
		}
		return billing.SellerEntitlement{}, err
	}
	return entitlement, nil
}

func (repository *localOnboardingEntitlementRepository) Apply(
	ctx context.Context,
	entitlement billing.SellerEntitlement,
	reconciliation billing.EntitlementReconciliation,
	expectedVersion uint64,
) error {
	return repository.repository.Apply(ctx, entitlement, reconciliation, expectedVersion)
}
