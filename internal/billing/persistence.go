package billing

import "github.com/fourgeez/agentpay/internal/domain"

type SellerEntitlementSnapshot struct {
	SellerID                   domain.ID               `json:"sellerId"`
	PlanID                     PlanID                  `json:"planId"`
	PlanVersion                uint64                  `json:"planVersion"`
	Status                     EntitlementStatus       `json:"status"`
	BillingPeriodStart         domain.Timestamp        `json:"billingPeriodStart"`
	BillingPeriodEnd           domain.Timestamp        `json:"billingPeriodEnd"`
	AccessEndsAt               domain.Timestamp        `json:"accessEndsAt"`
	GraceEndsAt                *domain.Timestamp       `json:"graceEndsAt"`
	CancelAtPeriodEnd          bool                    `json:"cancelAtPeriodEnd"`
	EntitlementEpoch           uint64                  `json:"entitlementEpoch"`
	Source                     EntitlementSource       `json:"source"`
	SourceRevision             string                  `json:"sourceRevision"`
	StatusReason               EntitlementStatusReason `json:"statusReason,omitempty"`
	Provider                   EntitlementProvider     `json:"provider"`
	ProviderCustomerID         string                  `json:"providerCustomerId"`
	ProviderSubscriptionID     string                  `json:"providerSubscriptionId"`
	ProviderPriceID            string                  `json:"providerPriceId"`
	LastProviderEventID        string                  `json:"lastProviderEventId,omitempty"`
	LastReconciledAt           *domain.Timestamp       `json:"lastReconciledAt"`
	CredentialRotationRequired bool                    `json:"credentialRotationRequired"`
	AssignedAt                 domain.Timestamp        `json:"assignedAt"`
	UpdatedAt                  domain.Timestamp        `json:"updatedAt"`
	Version                    uint64                  `json:"version"`
}

type SellerPlanSnapshot = SellerEntitlementSnapshot

func (entitlement SellerEntitlement) Snapshot() SellerEntitlementSnapshot {
	return SellerEntitlementSnapshot{
		SellerID:                   entitlement.sellerID,
		PlanID:                     entitlement.planID,
		PlanVersion:                entitlement.planVersion,
		Status:                     entitlement.status,
		BillingPeriodStart:         entitlement.billingPeriodStart,
		BillingPeriodEnd:           entitlement.billingPeriodEnd,
		AccessEndsAt:               entitlement.accessEndsAt,
		GraceEndsAt:                copyTimestamp(entitlement.graceEndsAt),
		CancelAtPeriodEnd:          entitlement.cancelAtPeriodEnd,
		EntitlementEpoch:           entitlement.entitlementEpoch,
		Source:                     entitlement.source,
		SourceRevision:             entitlement.sourceRevision,
		StatusReason:               entitlement.statusReason,
		Provider:                   entitlement.provider,
		ProviderCustomerID:         entitlement.providerCustomerID,
		ProviderSubscriptionID:     entitlement.providerSubscriptionID,
		ProviderPriceID:            entitlement.providerPriceID,
		LastProviderEventID:        entitlement.lastProviderEventID,
		LastReconciledAt:           copyTimestamp(entitlement.lastReconciledAt),
		CredentialRotationRequired: entitlement.credentialRotationRequired,
		AssignedAt:                 entitlement.assignedAt,
		UpdatedAt:                  entitlement.updatedAt,
		Version:                    entitlement.version,
	}
}

func RestoreSellerEntitlement(snapshot SellerEntitlementSnapshot) (SellerEntitlement, error) {
	return NewSellerEntitlement(SellerEntitlementParams(snapshot))
}

func RestoreSellerPlan(snapshot SellerPlanSnapshot) (SellerPlan, error) {
	return RestoreSellerEntitlement(snapshot)
}
