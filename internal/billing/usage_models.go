package billing

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/transactions"
)

var (
	// ErrUsageSourceInvalid reports a transaction that is not billable usage.
	ErrUsageSourceInvalid = errors.New("usage source transaction is not successful")
	// ErrUsageWindowInvalid reports an unsupported invoice export window.
	ErrUsageWindowInvalid = errors.New("usage export window is invalid")
	// ErrUsageExportTooLarge reports a window above the bounded export size.
	ErrUsageExportTooLarge = errors.New("usage export exceeds the maximum event count")
)

// MeterName identifies one immutable AgentPay usage dimension.
type MeterName string

const MeterSuccessfulTransaction MeterName = "successful_transaction"

// UsageMeterEventParams contains values required for one immutable usage fact.
type UsageMeterEventParams struct {
	MeterEventID        domain.ID
	SellerID            domain.ID
	MeterName           MeterName
	Quantity            uint64
	SourceTransactionID domain.ID
	PlanID              PlanID
	PlanVersion         uint64
	OccurredAt          domain.Timestamp
}

// UsageMeterEvent is one immutable, idempotent billable usage fact.
type UsageMeterEvent struct {
	meterEventID        domain.ID
	sellerID            domain.ID
	meterName           MeterName
	quantity            uint64
	sourceTransactionID domain.ID
	planID              PlanID
	planVersion         uint64
	occurredAt          domain.Timestamp
}

// UsageMeterEventView is the public usage-event representation.
type UsageMeterEventView struct {
	MeterEventID        domain.ID        `json:"meterEventId"`
	SellerID            domain.ID        `json:"sellerId"`
	MeterName           MeterName        `json:"meterName"`
	Quantity            uint64           `json:"quantity"`
	SourceTransactionID domain.ID        `json:"sourceTransactionId"`
	PlanID              PlanID           `json:"planId"`
	PlanVersion         uint64           `json:"planVersion"`
	OccurredAt          domain.Timestamp `json:"occurredAt"`
}

// InvoiceLineItem groups usage under one meter and plan version.
type InvoiceLineItem struct {
	MeterName   MeterName `json:"meterName"`
	PlanID      PlanID    `json:"planId"`
	PlanVersion uint64    `json:"planVersion"`
	Quantity    uint64    `json:"quantity"`
}

// InvoiceExport is a non-monetary usage statement for a future billing adapter.
type InvoiceExport struct {
	SchemaVersion string            `json:"schemaVersion"`
	SellerID      domain.ID         `json:"sellerId"`
	PeriodStart   domain.Timestamp  `json:"periodStart"`
	PeriodEnd     domain.Timestamp  `json:"periodEnd"`
	GeneratedAt   domain.Timestamp  `json:"generatedAt"`
	LineItems     []InvoiceLineItem `json:"lineItems"`
}

// UsageRepository stores immutable usage and bounded seller-window queries.
type UsageRepository interface {
	CreateIfAbsent(context.Context, UsageMeterEvent) (UsageMeterEvent, bool, error)
	ListBySellerWindow(context.Context, domain.ID, domain.Timestamp, domain.Timestamp, int) ([]UsageMeterEvent, error)
}

// PlanResolver resolves the plan version attached to usage facts.
type PlanResolver interface {
	ResolveSellerPlan(context.Context, domain.ID) (SellerPlanResponse, error)
}

// TransactionReader loads authoritative transaction outcomes.
type TransactionReader interface {
	Get(context.Context, domain.ID) (transactions.Transaction, error)
}

// MeterEventID returns the immutable usage event identifier.
func (event UsageMeterEvent) MeterEventID() domain.ID {
	return event.meterEventID
}

// SellerID returns the billed seller identifier.
func (event UsageMeterEvent) SellerID() domain.ID {
	return event.sellerID
}

// MeterName returns the usage dimension.
func (event UsageMeterEvent) MeterName() MeterName {
	return event.meterName
}

// Quantity returns the immutable usage quantity.
func (event UsageMeterEvent) Quantity() uint64 {
	return event.quantity
}

// SourceTransactionID returns the successful source transaction.
func (event UsageMeterEvent) SourceTransactionID() domain.ID {
	return event.sourceTransactionID
}

// PlanID returns the plan attached when usage occurred.
func (event UsageMeterEvent) PlanID() PlanID {
	return event.planID
}

// PlanVersion returns the plan version attached when usage occurred.
func (event UsageMeterEvent) PlanVersion() uint64 {
	return event.planVersion
}

// OccurredAt returns the authoritative transaction completion time.
func (event UsageMeterEvent) OccurredAt() domain.Timestamp {
	return event.occurredAt
}
