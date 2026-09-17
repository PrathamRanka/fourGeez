package billing

import "github.com/fourgeez/agentpay/internal/domain"

// SellerPlanSnapshot is the complete persisted assignment representation.
type SellerPlanSnapshot struct {
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

// Snapshot returns the complete plan assignment persistence representation.
func (assignment SellerPlan) Snapshot() SellerPlanSnapshot {
	return SellerPlanSnapshot{
		SellerID:           assignment.sellerID,
		PlanID:             assignment.planID,
		PlanVersion:        assignment.planVersion,
		Status:             assignment.status,
		BillingPeriodStart: assignment.billingPeriodStart,
		BillingPeriodEnd:   assignment.billingPeriodEnd,
		AssignedAt:         assignment.assignedAt,
		UpdatedAt:          assignment.updatedAt,
		Version:            assignment.version,
	}
}

// RestoreSellerPlan recreates an assignment from trusted persistence.
func RestoreSellerPlan(snapshot SellerPlanSnapshot) (SellerPlan, error) {
	assignment, err := NewSellerPlan(SellerPlanParams{
		SellerID:           snapshot.SellerID,
		PlanID:             snapshot.PlanID,
		PlanVersion:        snapshot.PlanVersion,
		Status:             snapshot.Status,
		BillingPeriodStart: snapshot.BillingPeriodStart,
		BillingPeriodEnd:   snapshot.BillingPeriodEnd,
		AssignedAt:         snapshot.AssignedAt,
	})
	if err != nil {
		return SellerPlan{}, err
	}
	if snapshot.Version == 0 || snapshot.UpdatedAt.Time().IsZero() ||
		snapshot.UpdatedAt.Before(snapshot.AssignedAt) {
		return SellerPlan{}, domain.NewValidationError(
			"sellerPlan",
			"persistence",
			"stored seller plan metadata is invalid",
		)
	}
	assignment.updatedAt = snapshot.UpdatedAt
	assignment.version = snapshot.Version
	return assignment, nil
}
