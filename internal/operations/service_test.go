package operations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestServiceEnforcesMonthlyAndResourceQuotas verifies all quota classes.
func TestServiceEnforcesMonthlyAndResourceQuotas(t *testing.T) {
	sellerID := operationsID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	plan := billing.SellerPlanResponse{
		Assignment: billing.SellerPlanView{SellerID: sellerID, Status: billing.SellerPlanStatusActive},
		Plan: billing.PlanDefinition{
			Limits: billing.PlanLimits{
				APIRequestsPerMonth:       1,
				MCPOperationsPerMonth:     1,
				PublishedRoutes:           1,
				WebhookSubscriptions:      1,
				WebhookDeliveriesPerMonth: 1,
			},
			Features: billing.PlanFeatures{Webhooks: true},
		},
	}
	service := NewService(
		newQuotaRepository(),
		quotaPlanResolver{response: plan},
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)},
	)

	if err := service.ConsumeAPIRequest(t.Context(), sellerID); err != nil {
		t.Fatal(err)
	}
	if err := service.ConsumeAPIRequest(t.Context(), sellerID); !errors.Is(err, domain.ErrRateLimitExceeded) {
		t.Fatalf("API quota error = %v", err)
	}
	for range 2 {
		if err := service.ConsumeWebhookDelivery(t.Context(), sellerID, "whk_1:evt_1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.ConsumeWebhookDelivery(t.Context(), sellerID, "whk_2:evt_2"); !errors.Is(err, domain.ErrRateLimitExceeded) {
		t.Fatalf("webhook quota error = %v", err)
	}
	if err := service.AllowPublishedRoute(t.Context(), sellerID, 1); !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("route quota error = %v", err)
	}
	if err := service.AllowWebhookSubscription(t.Context(), sellerID, 1); !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("subscription quota error = %v", err)
	}
}

// TestServiceDeniesSuspendedSeller verifies deterministic permission denial.
func TestServiceDeniesSuspendedSeller(t *testing.T) {
	sellerID := operationsID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	service := NewService(
		newQuotaRepository(),
		quotaPlanResolver{response: billing.SellerPlanResponse{
			Assignment: billing.SellerPlanView{SellerID: sellerID, Status: billing.SellerPlanStatusSuspended},
		}},
		domain.FixedClock{Value: time.Now().UTC()},
	)
	if err := service.ConsumeMCPOperation(t.Context(), sellerID); !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("suspended plan error = %v", err)
	}
}

// operationsID parses one canonical test identifier.
func operationsID(t *testing.T, raw string, prefix domain.IDPrefix) domain.ID {
	t.Helper()
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}

type quotaPlanResolver struct{ response billing.SellerPlanResponse }

// ResolveSellerPlan returns the configured plan.
func (resolver quotaPlanResolver) ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error) {
	return resolver.response, nil
}

type quotaRepository struct {
	counts map[string]uint64
	claims map[string]struct{}
}

// newQuotaRepository creates an enforcing in-test repository.
func newQuotaRepository() *quotaRepository {
	return &quotaRepository{counts: map[string]uint64{}, claims: map[string]struct{}{}}
}

// Increment consumes one unit below the limit.
func (repository *quotaRepository) Increment(_ context.Context, request CounterRequest) error {
	key := request.SellerID.String() + string(request.QuotaName)
	if repository.counts[key] >= request.Limit {
		return domain.ErrRateLimitExceeded
	}
	repository.counts[key]++
	return nil
}

// IncrementUnique consumes one unit once per source.
func (repository *quotaRepository) IncrementUnique(ctx context.Context, request CounterRequest, source string) error {
	key := request.SellerID.String() + string(request.QuotaName) + source
	if _, exists := repository.claims[key]; exists {
		return nil
	}
	if err := repository.Increment(ctx, request); err != nil {
		return err
	}
	repository.claims[key] = struct{}{}
	return nil
}
