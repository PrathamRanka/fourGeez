package billing

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const currentPlanVersion uint64 = 1

var planCatalogV1 = map[PlanID]PlanDefinition{
	PlanStarter: {
		PlanID:      PlanStarter,
		PlanVersion: currentPlanVersion,
		DisplayName: "Starter",
		Limits: PlanLimits{
			APIRequestsPerMonth:       10_000,
			MCPOperationsPerMonth:     1_000,
			PublishedRoutes:           5,
			WebhookSubscriptions:      1,
			WebhookDeliveriesPerMonth: 1_000,
			AnalyticsWindowDays:       7,
			EvidenceRetentionDays:     30,
		},
		Features: PlanFeatures{
			Webhooks: true,
		},
	},
	PlanGrowth: {
		PlanID:      PlanGrowth,
		PlanVersion: currentPlanVersion,
		DisplayName: "Growth",
		Limits: PlanLimits{
			APIRequestsPerMonth:       100_000,
			MCPOperationsPerMonth:     10_000,
			PublishedRoutes:           50,
			WebhookSubscriptions:      5,
			WebhookDeliveriesPerMonth: 25_000,
			AnalyticsWindowDays:       90,
			EvidenceRetentionDays:     180,
		},
		Features: PlanFeatures{
			Approvals:         true,
			Webhooks:          true,
			AdvancedAnalytics: true,
		},
	},
	PlanScale: {
		PlanID:      PlanScale,
		PlanVersion: currentPlanVersion,
		DisplayName: "Scale",
		Limits: PlanLimits{
			APIRequestsPerMonth:       1_000_000,
			MCPOperationsPerMonth:     100_000,
			PublishedRoutes:           500,
			WebhookSubscriptions:      25,
			WebhookDeliveriesPerMonth: 250_000,
			AnalyticsWindowDays:       365,
			EvidenceRetentionDays:     3_650,
		},
		Features: PlanFeatures{
			Approvals:         true,
			Webhooks:          true,
			AdvancedAnalytics: true,
			PrioritySupport:   true,
		},
	},
}

// Service owns seller plans independently from buyer settlement.
type Service struct {
	repository Repository
	authorizer SellerAuthorizer
	clock      domain.Clock
}

// NewService creates the seller billing-plan service.
func NewService(
	repository Repository,
	authorizer SellerAuthorizer,
	clock domain.Clock,
) *Service {
	return &Service{
		repository: repository,
		authorizer: authorizer,
		clock:      clock,
	}
}

// V1PlanCatalog returns deterministic isolated plan definitions.
func V1PlanCatalog() []PlanDefinition {
	plans := make([]PlanDefinition, 0, len(planCatalogV1))
	for _, plan := range planCatalogV1 {
		plans = append(plans, plan)
	}
	sort.Slice(plans, func(leftIndex int, rightIndex int) bool {
		order := map[PlanID]int{PlanStarter: 0, PlanGrowth: 1, PlanScale: 2}
		return order[plans[leftIndex].PlanID] < order[plans[rightIndex].PlanID]
	})
	return plans
}

// GetSellerPlan authorizes and initializes the seller's default assignment.
func (service *Service) GetSellerPlan(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
) (SellerPlanResponse, error) {
	if err := service.authorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return SellerPlanResponse{}, ErrSellerPlanNotFound
	}
	assignment, err := service.repository.Get(ctx, sellerID)
	if err == nil {
		return sellerPlanResponse(assignment)
	}
	if !errors.Is(err, ErrSellerPlanNotFound) &&
		!errors.Is(err, persistence.ErrNotFound) {
		return SellerPlanResponse{}, err
	}
	now := service.clock.Now().UTC()
	periodStart, periodEnd := utcMonthBoundaries(now)
	assignment, err = NewSellerPlan(SellerPlanParams{
		SellerID:           sellerID,
		PlanID:             PlanStarter,
		PlanVersion:        currentPlanVersion,
		Status:             SellerPlanStatusActive,
		BillingPeriodStart: domain.NewTimestamp(periodStart),
		BillingPeriodEnd:   domain.NewTimestamp(periodEnd),
		AssignedAt:         domain.NewTimestamp(now),
	})
	if err != nil {
		return SellerPlanResponse{}, err
	}
	assignment, _, err = service.repository.CreateIfAbsent(ctx, assignment)
	if err != nil {
		return SellerPlanResponse{}, err
	}
	return sellerPlanResponse(assignment)
}

// AssignPlan changes one seller assignment through the trusted billing boundary.
func (service *Service) AssignPlan(
	ctx context.Context,
	sellerID domain.ID,
	planID PlanID,
	expectedVersion uint64,
) (SellerPlanResponse, error) {
	planID = normalizePlanID(planID)
	plan, exists := planCatalogV1[planID]
	if !exists {
		return SellerPlanResponse{}, ErrPlanNotFound
	}
	assignment, err := service.repository.Get(ctx, sellerID)
	if err != nil {
		return SellerPlanResponse{}, err
	}
	if err := assignment.ChangePlan(
		plan.PlanID,
		plan.PlanVersion,
		domain.NewTimestamp(service.clock.Now()),
	); err != nil {
		return SellerPlanResponse{}, err
	}
	if err := service.repository.Update(ctx, assignment, expectedVersion); err != nil {
		return SellerPlanResponse{}, err
	}
	return sellerPlanResponse(assignment)
}

// NewSellerPlan validates and creates one active assignment.
func NewSellerPlan(params SellerPlanParams) (SellerPlan, error) {
	var validationErrors domain.ValidationErrors
	if params.SellerID.Prefix() != domain.SellerIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "prefix", "must use the seller prefix"))
	}
	plan, exists := planCatalogV1[params.PlanID]
	if !exists || plan.PlanVersion != params.PlanVersion {
		validationErrors = append(validationErrors, domain.NewValidationError("plan", "version", "must reference a supported plan version"))
	}
	if params.Status != SellerPlanStatusActive && params.Status != SellerPlanStatusSuspended {
		validationErrors = append(validationErrors, domain.NewValidationError("status", "supported", "must be active or suspended"))
	}
	if params.BillingPeriodStart.Time().IsZero() ||
		params.BillingPeriodEnd.Time().IsZero() ||
		!params.BillingPeriodStart.Before(params.BillingPeriodEnd) {
		validationErrors = append(validationErrors, domain.NewValidationError("billingPeriod", "range", "must have ordered UTC boundaries"))
	}
	if params.AssignedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("assignedAt", "required", "is required"))
	}
	if len(validationErrors) > 0 {
		return SellerPlan{}, validationErrors
	}
	return SellerPlan{
		sellerID:           params.SellerID,
		planID:             params.PlanID,
		planVersion:        params.PlanVersion,
		status:             params.Status,
		billingPeriodStart: params.BillingPeriodStart,
		billingPeriodEnd:   params.BillingPeriodEnd,
		assignedAt:         params.AssignedAt,
		updatedAt:          params.AssignedAt,
		version:            1,
	}, nil
}

// ChangePlan records a supported plan version without touching settlement state.
func (assignment *SellerPlan) ChangePlan(
	planID PlanID,
	planVersion uint64,
	changedAt domain.Timestamp,
) error {
	plan, exists := planCatalogV1[planID]
	if !exists || plan.PlanVersion != planVersion {
		return ErrPlanNotFound
	}
	if changedAt.Before(assignment.updatedAt) {
		return domain.NewValidationError("updatedAt", "chronology", "cannot move backward")
	}
	assignment.planID = planID
	assignment.planVersion = planVersion
	assignment.assignedAt = changedAt
	assignment.updatedAt = changedAt
	assignment.version++
	return nil
}

// sellerPlanResponse resolves the immutable definition for one assignment.
func sellerPlanResponse(assignment SellerPlan) (SellerPlanResponse, error) {
	plan, exists := planCatalogV1[assignment.PlanID()]
	if !exists || plan.PlanVersion != assignment.PlanVersion() {
		return SellerPlanResponse{}, ErrPlanNotFound
	}
	return SellerPlanResponse{
		Assignment: sellerPlanView(assignment),
		Plan:       plan,
	}, nil
}

// sellerPlanView returns the public assignment representation.
func sellerPlanView(assignment SellerPlan) SellerPlanView {
	return SellerPlanView{
		SellerID:           assignment.SellerID(),
		PlanID:             assignment.PlanID(),
		PlanVersion:        assignment.PlanVersion(),
		Status:             assignment.Status(),
		BillingPeriodStart: assignment.BillingPeriodStart(),
		BillingPeriodEnd:   assignment.BillingPeriodEnd(),
		AssignedAt:         assignment.AssignedAt(),
		UpdatedAt:          assignment.UpdatedAt(),
		Version:            assignment.Version(),
	}
}

// utcMonthBoundaries returns inclusive and exclusive UTC calendar boundaries.
func utcMonthBoundaries(now time.Time) (time.Time, time.Time) {
	utc := now.UTC()
	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

// normalizePlanID trims a plan identifier supplied at a system boundary.
func normalizePlanID(planID PlanID) PlanID {
	return PlanID(strings.TrimSpace(string(planID)))
}
