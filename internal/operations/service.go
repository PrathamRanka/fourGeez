package operations

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

// Service enforces plan status and operational limits.
type Service struct {
	repository   Repository
	planResolver PlanResolver
	clock        domain.Clock
}

// NewService creates the quota service.
func NewService(repository Repository, planResolver PlanResolver, clock domain.Clock) *Service {
	return &Service{repository: repository, planResolver: planResolver, clock: clock}
}

// ConsumeAPIRequest consumes one seller API unit.
func (service *Service) ConsumeAPIRequest(ctx context.Context, sellerID domain.ID) error {
	return service.consume(ctx, sellerID, QuotaAPIRequest, "")
}

// ConsumeMCPOperation consumes one MCP unit.
func (service *Service) ConsumeMCPOperation(ctx context.Context, sellerID domain.ID) error {
	return service.consume(ctx, sellerID, QuotaMCPOperation, "")
}

// ConsumeWebhookDelivery consumes one retry-safe delivery unit.
func (service *Service) ConsumeWebhookDelivery(ctx context.Context, sellerID domain.ID, sourceID string) error {
	if strings.TrimSpace(sourceID) == "" {
		return domain.NewValidationError("sourceId", "required", "is required")
	}
	return service.consume(ctx, sellerID, QuotaWebhookDelivery, sourceID)
}

// AllowPublishedRoute checks current published-route capacity.
func (service *Service) AllowPublishedRoute(ctx context.Context, sellerID domain.ID, count uint64) error {
	plan, err := service.activePlan(ctx, sellerID)
	if err != nil {
		return err
	}
	if count >= plan.Plan.Limits.PublishedRoutes {
		return domain.ErrPermissionDenied
	}
	return nil
}

// AllowWebhookSubscription checks webhook entitlement and capacity.
func (service *Service) AllowWebhookSubscription(ctx context.Context, sellerID domain.ID, count uint64) error {
	plan, err := service.activePlan(ctx, sellerID)
	if err != nil {
		return err
	}
	if !plan.Plan.Features.Webhooks || count >= plan.Plan.Limits.WebhookSubscriptions {
		return domain.ErrPermissionDenied
	}
	return nil
}

// consume atomically increments one UTC-month quota.
func (service *Service) consume(ctx context.Context, sellerID domain.ID, quotaName QuotaName, sourceID string) error {
	plan, err := service.activePlan(ctx, sellerID)
	if err != nil {
		return err
	}
	limit := plan.Plan.Limits.APIRequestsPerMonth
	if quotaName == QuotaMCPOperation {
		limit = plan.Plan.Limits.MCPOperationsPerMonth
	} else if quotaName == QuotaWebhookDelivery {
		limit = plan.Plan.Limits.WebhookDeliveriesPerMonth
	}
	now := service.clock.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	request := CounterRequest{
		SellerID:    sellerID,
		QuotaName:   quotaName,
		PeriodStart: domain.NewTimestamp(start),
		PeriodEnd:   domain.NewTimestamp(start.AddDate(0, 1, 0)),
		Limit:       limit,
		UpdatedAt:   domain.NewTimestamp(now),
	}
	if sourceID == "" {
		err = service.repository.Increment(ctx, request)
	} else {
		err = service.repository.IncrementUnique(ctx, request, sourceID)
	}
	if errors.Is(err, domain.ErrRateLimitExceeded) {
		return domain.ErrRateLimitExceeded
	}
	return err
}

// activePlan enforces the authoritative exclusive entitlement boundary.
func (service *Service) activePlan(ctx context.Context, sellerID domain.ID) (billing.SellerPlanResponse, error) {
	plan, err := service.planResolver.ResolveSellerPlan(ctx, sellerID)
	if err != nil {
		return billing.SellerPlanResponse{}, err
	}
	if plan.Assignment.Status != billing.EntitlementStatusActive ||
		!domain.NewTimestamp(service.clock.Now()).Before(plan.Assignment.AccessEndsAt) ||
		plan.Assignment.CredentialRotationRequired {
		return billing.SellerPlanResponse{}, domain.ErrPermissionDenied
	}
	return plan, nil
}
