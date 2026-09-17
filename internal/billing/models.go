package billing

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
)

var (
	// ErrPlanNotFound reports an unknown versioned plan identifier.
	ErrPlanNotFound = errors.New("seller plan was not found")
	// ErrSellerPlanNotFound reports a missing or inaccessible assignment.
	ErrSellerPlanNotFound = errors.New("seller plan assignment was not found")
	// ErrSellerPlanConflict reports a stale assignment update.
	ErrSellerPlanConflict = errors.New("seller plan assignment conflict")
)

// PlanID identifies one product tier in the versioned catalog.
type PlanID string

const (
	PlanStarter PlanID = "starter"
	PlanGrowth  PlanID = "growth"
	PlanScale   PlanID = "scale"
)

// SellerPlanStatus identifies whether plan entitlements are active.
type SellerPlanStatus string

const (
	SellerPlanStatusActive    SellerPlanStatus = "active"
	SellerPlanStatusSuspended SellerPlanStatus = "suspended"
)

// PlanLimits contains operational quotas that OPS-001 will enforce.
type PlanLimits struct {
	APIRequestsPerMonth       uint64 `json:"apiRequestsPerMonth"`
	MCPOperationsPerMonth     uint64 `json:"mcpOperationsPerMonth"`
	PublishedRoutes           uint64 `json:"publishedRoutes"`
	WebhookSubscriptions      uint64 `json:"webhookSubscriptions"`
	WebhookDeliveriesPerMonth uint64 `json:"webhookDeliveriesPerMonth"`
	AnalyticsWindowDays       uint64 `json:"analyticsWindowDays"`
	EvidenceRetentionDays     uint64 `json:"evidenceRetentionDays"`
}

// PlanFeatures contains explicit seller product entitlements.
type PlanFeatures struct {
	Approvals         bool `json:"approvals"`
	Webhooks          bool `json:"webhooks"`
	AdvancedAnalytics bool `json:"advancedAnalytics"`
	PrioritySupport   bool `json:"prioritySupport"`
}

// PlanDefinition is one immutable versioned seller plan.
type PlanDefinition struct {
	PlanID      PlanID       `json:"planId"`
	PlanVersion uint64       `json:"planVersion"`
	DisplayName string       `json:"displayName"`
	Limits      PlanLimits   `json:"limits"`
	Features    PlanFeatures `json:"features"`
}

// SellerPlanParams contains values required for a seller assignment.
type SellerPlanParams struct {
	SellerID           domain.ID
	PlanID             PlanID
	PlanVersion        uint64
	Status             SellerPlanStatus
	BillingPeriodStart domain.Timestamp
	BillingPeriodEnd   domain.Timestamp
	AssignedAt         domain.Timestamp
}

// SellerPlan stores one seller's current versioned plan assignment.
type SellerPlan struct {
	sellerID           domain.ID
	planID             PlanID
	planVersion        uint64
	status             SellerPlanStatus
	billingPeriodStart domain.Timestamp
	billingPeriodEnd   domain.Timestamp
	assignedAt         domain.Timestamp
	updatedAt          domain.Timestamp
	version            uint64
}

// SellerPlanResponse combines an assignment with its immutable plan definition.
type SellerPlanResponse struct {
	Assignment SellerPlanView `json:"assignment"`
	Plan       PlanDefinition `json:"plan"`
}

// SellerPlanView is the public assignment representation.
type SellerPlanView struct {
	SellerID           domain.ID        `json:"sellerId"`
	PlanID             PlanID           `json:"planId"`
	PlanVersion        uint64           `json:"planVersion"`
	Status             SellerPlanStatus `json:"status"`
	BillingPeriodStart domain.Timestamp `json:"billingPeriodStart"`
	BillingPeriodEnd   domain.Timestamp `json:"billingPeriodEnd"`
	AssignedAt         domain.Timestamp `json:"assignedAt"`
	UpdatedAt          domain.Timestamp `json:"updatedAt"`
	Version            uint64           `json:"version"`
}

// Repository persists one plan assignment per seller.
type Repository interface {
	Get(context.Context, domain.ID) (SellerPlan, error)
	CreateIfAbsent(context.Context, SellerPlan) (SellerPlan, bool, error)
	Update(context.Context, SellerPlan, uint64) error
}

// SellerAuthorizer verifies seller ownership for plan reads.
type SellerAuthorizer interface {
	AuthorizeSeller(context.Context, string, domain.ID) error
}

// SellerID returns the assigned seller identifier.
func (assignment SellerPlan) SellerID() domain.ID {
	return assignment.sellerID
}

// PlanID returns the assigned plan identifier.
func (assignment SellerPlan) PlanID() PlanID {
	return assignment.planID
}

// PlanVersion returns the immutable catalog version used by this assignment.
func (assignment SellerPlan) PlanVersion() uint64 {
	return assignment.planVersion
}

// Status returns whether plan entitlements are active.
func (assignment SellerPlan) Status() SellerPlanStatus {
	return assignment.status
}

// BillingPeriodStart returns the inclusive UTC period boundary.
func (assignment SellerPlan) BillingPeriodStart() domain.Timestamp {
	return assignment.billingPeriodStart
}

// BillingPeriodEnd returns the exclusive UTC period boundary.
func (assignment SellerPlan) BillingPeriodEnd() domain.Timestamp {
	return assignment.billingPeriodEnd
}

// AssignedAt returns when the current plan was assigned.
func (assignment SellerPlan) AssignedAt() domain.Timestamp {
	return assignment.assignedAt
}

// UpdatedAt returns the latest assignment mutation time.
func (assignment SellerPlan) UpdatedAt() domain.Timestamp {
	return assignment.updatedAt
}

// Version returns the optimistic concurrency version.
func (assignment SellerPlan) Version() uint64 {
	return assignment.version
}
