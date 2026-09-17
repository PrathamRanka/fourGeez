package billing

import "github.com/fourgeez/agentpay/internal/domain"

// UsageMeterEventSnapshot is the persisted immutable usage representation.
type UsageMeterEventSnapshot struct {
	MeterEventID        domain.ID        `json:"meterEventId"`
	SellerID            domain.ID        `json:"sellerId"`
	MeterName           MeterName        `json:"meterName"`
	Quantity            uint64           `json:"quantity"`
	SourceTransactionID domain.ID        `json:"sourceTransactionId"`
	PlanID              PlanID           `json:"planId"`
	PlanVersion         uint64           `json:"planVersion"`
	OccurredAt          domain.Timestamp `json:"occurredAt"`
}

// Snapshot returns the complete immutable usage representation.
func (event UsageMeterEvent) Snapshot() UsageMeterEventSnapshot {
	return UsageMeterEventSnapshot{
		MeterEventID:        event.meterEventID,
		SellerID:            event.sellerID,
		MeterName:           event.meterName,
		Quantity:            event.quantity,
		SourceTransactionID: event.sourceTransactionID,
		PlanID:              event.planID,
		PlanVersion:         event.planVersion,
		OccurredAt:          event.occurredAt,
	}
}

// RestoreUsageMeterEvent validates persisted immutable usage.
func RestoreUsageMeterEvent(snapshot UsageMeterEventSnapshot) (UsageMeterEvent, error) {
	return NewUsageMeterEvent(UsageMeterEventParams(snapshot))
}

// usageMeterEventView returns the public immutable usage representation.
func usageMeterEventView(event UsageMeterEvent) UsageMeterEventView {
	snapshot := event.Snapshot()
	return UsageMeterEventView(snapshot)
}
