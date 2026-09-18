package operations

import (
	"context"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

// QuotaName identifies one operational limit.
type QuotaName string

const (
	QuotaAPIRequest      QuotaName = "api_request"
	QuotaMCPOperation    QuotaName = "mcp_operation"
	QuotaWebhookDelivery QuotaName = "webhook_delivery"
)

// CounterRequest describes one atomic monthly quota increment.
type CounterRequest struct {
	SellerID    domain.ID
	QuotaName   QuotaName
	PeriodStart domain.Timestamp
	PeriodEnd   domain.Timestamp
	Limit       uint64
	UpdatedAt   domain.Timestamp
}

// Repository persists atomic monthly quota usage.
type Repository interface {
	Increment(context.Context, CounterRequest) error
	IncrementUnique(context.Context, CounterRequest, string) error
}

// PlanResolver returns the seller's assigned plan.
type PlanResolver interface {
	ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error)
}

// Enforcer is the quota boundary consumed by application packages.
type Enforcer interface {
	ConsumeAPIRequest(context.Context, domain.ID) error
	ConsumeMCPOperation(context.Context, domain.ID) error
	ConsumeWebhookDelivery(context.Context, domain.ID, string) error
	AllowPublishedRoute(context.Context, domain.ID, uint64) error
	AllowWebhookSubscription(context.Context, domain.ID, uint64) error
}

// NoopEnforcer explicitly disables quotas in isolated tests.
type NoopEnforcer struct{}

// ConsumeAPIRequest permits one test request.
func (NoopEnforcer) ConsumeAPIRequest(context.Context, domain.ID) error {
	return nil
}

// ConsumeMCPOperation permits one test operation.
func (NoopEnforcer) ConsumeMCPOperation(context.Context, domain.ID) error {
	return nil
}

// ConsumeWebhookDelivery permits one test delivery.
func (NoopEnforcer) ConsumeWebhookDelivery(context.Context, domain.ID, string) error {
	return nil
}

// AllowPublishedRoute permits one test publication.
func (NoopEnforcer) AllowPublishedRoute(context.Context, domain.ID, uint64) error {
	return nil
}

// AllowWebhookSubscription permits one test subscription.
func (NoopEnforcer) AllowWebhookSubscription(context.Context, domain.ID, uint64) error {
	return nil
}
