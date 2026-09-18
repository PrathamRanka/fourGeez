package billing

import (
	"context"
	"errors"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

const EntitlementGraceDuration = 72 * time.Hour

var (
	ErrPlanNotFound              = errors.New("seller plan was not found")
	ErrSellerEntitlementNotFound = errors.New("seller entitlement was not found")
	ErrSellerEntitlementConflict = errors.New("seller entitlement conflict")
	ErrSellerPlanNotFound        = ErrSellerEntitlementNotFound
	ErrSellerPlanConflict        = ErrSellerEntitlementConflict
)

type PlanID string

const (
	PlanStarter PlanID = "starter"
	PlanGrowth  PlanID = "growth"
	PlanScale   PlanID = "scale"
)

type EntitlementStatus string

const (
	EntitlementStatusActive    EntitlementStatus = "active"
	EntitlementStatusGrace     EntitlementStatus = "grace"
	EntitlementStatusSuspended EntitlementStatus = "suspended"
	EntitlementStatusCancelled EntitlementStatus = "cancelled"
	EntitlementStatusClosed    EntitlementStatus = "closed"

	SellerPlanStatusActive    = EntitlementStatusActive
	SellerPlanStatusSuspended = EntitlementStatusSuspended
)

type SellerPlanStatus = EntitlementStatus

type EntitlementSource string

const (
	EntitlementSourceBillingProvider EntitlementSource = "billing_provider"
	EntitlementSourceOperator        EntitlementSource = "operator"
	EntitlementSourceLocal           EntitlementSource = "local"
)

type EntitlementProvider string

const (
	EntitlementProviderStripe   EntitlementProvider = "stripe"
	EntitlementProviderOperator EntitlementProvider = "operator"
	EntitlementProviderLocal    EntitlementProvider = "local"
)

type EntitlementStatusReason string

const (
	EntitlementStatusReasonPaymentFailed         EntitlementStatusReason = "payment_failed"
	EntitlementStatusReasonPaymentActionRequired EntitlementStatusReason = "payment_action_required"
	EntitlementStatusReasonCancellationRequested EntitlementStatusReason = "cancellation_requested"
	EntitlementStatusReasonCancelled             EntitlementStatusReason = "cancelled"
	EntitlementStatusReasonAdministrative        EntitlementStatusReason = "administrative"
	EntitlementStatusReasonFraudQuarantine       EntitlementStatusReason = "fraud_quarantine"
	EntitlementStatusReasonAccountClosed         EntitlementStatusReason = "account_closed"
	EntitlementStatusReasonProviderIncomplete    EntitlementStatusReason = "provider_incomplete"
)

type NetworkAccess string

const (
	NetworkAccessEnabled NetworkAccess = "enabled"
	NetworkAccessBlocked NetworkAccess = "blocked"
)

type DashboardAccess string

const (
	DashboardAccessFull             DashboardAccess = "full"
	DashboardAccessRecoveryReadOnly DashboardAccess = "recovery_read_only"
	DashboardAccessClosed           DashboardAccess = "closed"
)

type PlanLimits struct {
	APIRequestsPerMonth       uint64 `json:"apiRequestsPerMonth"`
	MCPOperationsPerMonth     uint64 `json:"mcpOperationsPerMonth"`
	PublishedRoutes           uint64 `json:"publishedRoutes"`
	WebhookSubscriptions      uint64 `json:"webhookSubscriptions"`
	WebhookDeliveriesPerMonth uint64 `json:"webhookDeliveriesPerMonth"`
	AnalyticsWindowDays       uint64 `json:"analyticsWindowDays"`
	EvidenceRetentionDays     uint64 `json:"evidenceRetentionDays"`
}

type PlanFeatures struct {
	Approvals         bool `json:"approvals"`
	Webhooks          bool `json:"webhooks"`
	AdvancedAnalytics bool `json:"advancedAnalytics"`
	PrioritySupport   bool `json:"prioritySupport"`
}

type PlanDefinition struct {
	PlanID      PlanID       `json:"planId"`
	PlanVersion uint64       `json:"planVersion"`
	DisplayName string       `json:"displayName"`
	Limits      PlanLimits   `json:"limits"`
	Features    PlanFeatures `json:"features"`
}

type SellerEntitlementParams struct {
	SellerID                   domain.ID
	PlanID                     PlanID
	PlanVersion                uint64
	Status                     EntitlementStatus
	BillingPeriodStart         domain.Timestamp
	BillingPeriodEnd           domain.Timestamp
	AccessEndsAt               domain.Timestamp
	GraceEndsAt                *domain.Timestamp
	CancelAtPeriodEnd          bool
	EntitlementEpoch           uint64
	Source                     EntitlementSource
	SourceRevision             string
	StatusReason               EntitlementStatusReason
	Provider                   EntitlementProvider
	ProviderCustomerID         string
	ProviderSubscriptionID     string
	ProviderPriceID            string
	LastProviderEventID        string
	LastReconciledAt           *domain.Timestamp
	CredentialRotationRequired bool
	AssignedAt                 domain.Timestamp
	UpdatedAt                  domain.Timestamp
	Version                    uint64
}

type EntitlementCandidate struct {
	SellerID               domain.ID
	PlanID                 PlanID
	PlanVersion            uint64
	Status                 EntitlementStatus
	BillingPeriodStart     domain.Timestamp
	BillingPeriodEnd       domain.Timestamp
	AccessEndsAt           domain.Timestamp
	CancelAtPeriodEnd      bool
	Source                 EntitlementSource
	StatusReason           EntitlementStatusReason
	Provider               EntitlementProvider
	ProviderCustomerID     string
	ProviderSubscriptionID string
	ProviderPriceID        string
	LastProviderEventID    string
}

type SellerEntitlement struct {
	sellerID                   domain.ID
	planID                     PlanID
	planVersion                uint64
	status                     EntitlementStatus
	billingPeriodStart         domain.Timestamp
	billingPeriodEnd           domain.Timestamp
	accessEndsAt               domain.Timestamp
	graceEndsAt                *domain.Timestamp
	cancelAtPeriodEnd          bool
	entitlementEpoch           uint64
	source                     EntitlementSource
	sourceRevision             string
	statusReason               EntitlementStatusReason
	provider                   EntitlementProvider
	providerCustomerID         string
	providerSubscriptionID     string
	providerPriceID            string
	lastProviderEventID        string
	lastReconciledAt           *domain.Timestamp
	credentialRotationRequired bool
	assignedAt                 domain.Timestamp
	updatedAt                  domain.Timestamp
	version                    uint64
}

type SellerPlan = SellerEntitlement
type SellerPlanParams = SellerEntitlementParams

type SellerPlanResponse struct {
	Assignment SellerEntitlementView `json:"assignment"`
	Plan       PlanDefinition        `json:"plan"`
}

type SellerEntitlementView struct {
	SellerID                   domain.ID                `json:"sellerId"`
	PlanID                     PlanID                   `json:"planId"`
	PlanVersion                uint64                   `json:"planVersion"`
	Status                     EntitlementStatus        `json:"status"`
	BillingPeriodStart         domain.Timestamp         `json:"billingPeriodStart"`
	BillingPeriodEnd           domain.Timestamp         `json:"billingPeriodEnd"`
	AccessEndsAt               domain.Timestamp         `json:"accessEndsAt"`
	GraceEndsAt                *domain.Timestamp        `json:"graceEndsAt"`
	CancelAtPeriodEnd          bool                     `json:"cancelAtPeriodEnd"`
	EntitlementEpoch           uint64                   `json:"entitlementEpoch"`
	Source                     EntitlementSource        `json:"source"`
	SourceRevision             string                   `json:"sourceRevision"`
	StatusReason               *EntitlementStatusReason `json:"statusReason"`
	CredentialRotationRequired bool                     `json:"credentialRotationRequired"`
	NetworkAccess              NetworkAccess            `json:"networkAccess"`
	DashboardAccess            DashboardAccess          `json:"dashboardAccess"`
	Provider                   EntitlementProvider      `json:"provider"`
	LastReconciledAt           *domain.Timestamp        `json:"lastReconciledAt"`
	AssignedAt                 domain.Timestamp         `json:"assignedAt"`
	UpdatedAt                  domain.Timestamp         `json:"updatedAt"`
	Version                    uint64                   `json:"version"`
}

type SellerPlanView = SellerEntitlementView

type EntitlementReconciliation struct {
	SellerID           domain.ID         `json:"sellerId"`
	SourceRevision     string            `json:"sourceRevision"`
	Source             EntitlementSource `json:"source"`
	TriggeringEventIDs []string          `json:"triggeringEventIds"`
	SnapshotHash       string            `json:"snapshotHash"`
	ReconciledAt       domain.Timestamp  `json:"reconciledAt"`
	AppliedVersion     uint64            `json:"appliedVersion"`
}

type Repository interface {
	Get(context.Context, domain.ID) (SellerEntitlement, error)
	Apply(context.Context, SellerEntitlement, EntitlementReconciliation, uint64) error
}

type SellerAuthorizer interface {
	AuthorizeSeller(context.Context, string, domain.ID) error
}

type EntitlementInvalidator interface {
	InvalidateSellerEntitlement(context.Context, domain.ID) error
}

func (entitlement SellerEntitlement) SellerID() domain.ID       { return entitlement.sellerID }
func (entitlement SellerEntitlement) PlanID() PlanID            { return entitlement.planID }
func (entitlement SellerEntitlement) PlanVersion() uint64       { return entitlement.planVersion }
func (entitlement SellerEntitlement) Status() EntitlementStatus { return entitlement.status }
func (entitlement SellerEntitlement) BillingPeriodStart() domain.Timestamp {
	return entitlement.billingPeriodStart
}
func (entitlement SellerEntitlement) BillingPeriodEnd() domain.Timestamp {
	return entitlement.billingPeriodEnd
}
func (entitlement SellerEntitlement) AccessEndsAt() domain.Timestamp { return entitlement.accessEndsAt }
func (entitlement SellerEntitlement) GraceEndsAt() *domain.Timestamp {
	return copyTimestamp(entitlement.graceEndsAt)
}
func (entitlement SellerEntitlement) CancelAtPeriodEnd() bool   { return entitlement.cancelAtPeriodEnd }
func (entitlement SellerEntitlement) EntitlementEpoch() uint64  { return entitlement.entitlementEpoch }
func (entitlement SellerEntitlement) Source() EntitlementSource { return entitlement.source }
func (entitlement SellerEntitlement) SourceRevision() string    { return entitlement.sourceRevision }
func (entitlement SellerEntitlement) StatusReason() EntitlementStatusReason {
	return entitlement.statusReason
}
func (entitlement SellerEntitlement) Provider() EntitlementProvider { return entitlement.provider }
func (entitlement SellerEntitlement) ProviderCustomerID() string {
	return entitlement.providerCustomerID
}
func (entitlement SellerEntitlement) ProviderSubscriptionID() string {
	return entitlement.providerSubscriptionID
}
func (entitlement SellerEntitlement) ProviderPriceID() string { return entitlement.providerPriceID }
func (entitlement SellerEntitlement) LastProviderEventID() string {
	return entitlement.lastProviderEventID
}
func (entitlement SellerEntitlement) LastReconciledAt() *domain.Timestamp {
	return copyTimestamp(entitlement.lastReconciledAt)
}
func (entitlement SellerEntitlement) CredentialRotationRequired() bool {
	return entitlement.credentialRotationRequired
}
func (entitlement SellerEntitlement) AssignedAt() domain.Timestamp { return entitlement.assignedAt }
func (entitlement SellerEntitlement) UpdatedAt() domain.Timestamp  { return entitlement.updatedAt }
func (entitlement SellerEntitlement) Version() uint64              { return entitlement.version }

func (entitlement SellerEntitlement) AllowsNetworkAccess(now domain.Timestamp) bool {
	return entitlement.status == EntitlementStatusActive && now.Before(entitlement.accessEndsAt)
}

func copyTimestamp(timestamp *domain.Timestamp) *domain.Timestamp {
	if timestamp == nil {
		return nil
	}
	copy := *timestamp
	return &copy
}
