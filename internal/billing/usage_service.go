package billing

import (
	"context"
	"sort"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const (
	invoiceExportSchemaVersion = "1"
	maximumInvoiceWindow       = 31 * 24 * time.Hour
	maximumInvoiceEvents       = 1000
)

// UsageService records successful usage and exports non-monetary invoice facts.
type UsageService struct {
	repository        UsageRepository
	planResolver      PlanResolver
	transactionReader TransactionReader
	authorizer        SellerAuthorizer
	idGenerator       domain.IDGenerator
	clock             domain.Clock
}

// NewUsageService creates the immutable usage-meter service.
func NewUsageService(
	repository UsageRepository,
	planResolver PlanResolver,
	transactionReader TransactionReader,
	authorizer SellerAuthorizer,
	idGenerator domain.IDGenerator,
	clock domain.Clock,
) *UsageService {
	return &UsageService{
		repository:        repository,
		planResolver:      planResolver,
		transactionReader: transactionReader,
		authorizer:        authorizer,
		idGenerator:       idGenerator,
		clock:             clock,
	}
}

// RecordSuccessfulTransaction records one billable unit exactly once.
func (service *UsageService) RecordSuccessfulTransaction(
	ctx context.Context,
	transactionID domain.ID,
) (UsageMeterEvent, error) {
	transaction, err := service.transactionReader.Get(ctx, transactionID)
	if err != nil {
		return UsageMeterEvent{}, err
	}
	if transaction.Status() != transactions.StatusFulfilled ||
		transaction.PaymentFinality() != transactions.PaymentFinalityFinalized {
		return UsageMeterEvent{}, ErrUsageSourceInvalid
	}
	plan, err := service.planResolver.ResolveSellerPlan(ctx, transaction.SellerID())
	if err != nil {
		return UsageMeterEvent{}, err
	}
	meterEventID, err := service.idGenerator.New(domain.UsageMeterEventIDPrefix)
	if err != nil {
		return UsageMeterEvent{}, err
	}
	event, err := NewUsageMeterEvent(UsageMeterEventParams{
		MeterEventID:        meterEventID,
		SellerID:            transaction.SellerID(),
		MeterName:           MeterSuccessfulTransaction,
		Quantity:            1,
		SourceTransactionID: transaction.TransactionID(),
		PlanID:              plan.Assignment.PlanID,
		PlanVersion:         plan.Assignment.PlanVersion,
		OccurredAt:          transaction.UpdatedAt(),
	})
	if err != nil {
		return UsageMeterEvent{}, err
	}
	stored, _, err := service.repository.CreateIfAbsent(ctx, event)
	return stored, err
}

// RecordSuccessfulTransactionUsage adapts usage metering to fulfillment hooks.
func (service *UsageService) RecordSuccessfulTransactionUsage(
	ctx context.Context,
	transactionID domain.ID,
) error {
	_, err := service.RecordSuccessfulTransaction(ctx, transactionID)
	return err
}

// ExportInvoice returns bounded usage grouped by meter and plan version.
func (service *UsageService) ExportInvoice(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	from domain.Timestamp,
	to domain.Timestamp,
) (InvoiceExport, error) {
	if err := service.authorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return InvoiceExport{}, ErrSellerPlanNotFound
	}
	if from.Time().IsZero() || to.Time().IsZero() ||
		!from.Before(to) || to.Time().Sub(from.Time()) > maximumInvoiceWindow {
		return InvoiceExport{}, ErrUsageWindowInvalid
	}
	events, err := service.repository.ListBySellerWindow(
		ctx,
		sellerID,
		from,
		to,
		maximumInvoiceEvents+1,
	)
	if err != nil {
		return InvoiceExport{}, err
	}
	if len(events) > maximumInvoiceEvents {
		return InvoiceExport{}, ErrUsageExportTooLarge
	}
	lineItems := aggregateInvoiceLineItems(events)
	return InvoiceExport{
		SchemaVersion: invoiceExportSchemaVersion,
		SellerID:      sellerID,
		PeriodStart:   from,
		PeriodEnd:     to,
		GeneratedAt:   domain.NewTimestamp(service.clock.Now()),
		LineItems:     lineItems,
	}, nil
}

// NewUsageMeterEvent validates one immutable usage fact.
func NewUsageMeterEvent(params UsageMeterEventParams) (UsageMeterEvent, error) {
	var validationErrors domain.ValidationErrors
	if params.MeterEventID.Prefix() != domain.UsageMeterEventIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("meterEventId", "prefix", "must use the usage meter prefix"))
	}
	if params.SellerID.Prefix() != domain.SellerIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "prefix", "must use the seller prefix"))
	}
	if params.MeterName != MeterSuccessfulTransaction {
		validationErrors = append(validationErrors, domain.NewValidationError("meterName", "supported", "must use a supported meter"))
	}
	if params.Quantity != 1 {
		validationErrors = append(validationErrors, domain.NewValidationError("quantity", "exact", "successful transaction quantity must equal one"))
	}
	if params.SourceTransactionID.Prefix() != domain.TransactionIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("sourceTransactionId", "prefix", "must use the transaction prefix"))
	}
	plan, exists := planCatalogV1[params.PlanID]
	if !exists || plan.PlanVersion != params.PlanVersion {
		validationErrors = append(validationErrors, domain.NewValidationError("plan", "version", "must reference a supported plan version"))
	}
	if params.OccurredAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("occurredAt", "required", "is required"))
	}
	if len(validationErrors) > 0 {
		return UsageMeterEvent{}, validationErrors
	}
	return UsageMeterEvent{
		meterEventID:        params.MeterEventID,
		sellerID:            params.SellerID,
		meterName:           params.MeterName,
		quantity:            params.Quantity,
		sourceTransactionID: params.SourceTransactionID,
		planID:              params.PlanID,
		planVersion:         params.PlanVersion,
		occurredAt:          params.OccurredAt,
	}, nil
}

// aggregateInvoiceLineItems deterministically groups immutable usage facts.
func aggregateInvoiceLineItems(events []UsageMeterEvent) []InvoiceLineItem {
	type lineItemKey struct {
		meterName   MeterName
		planID      PlanID
		planVersion uint64
	}
	totals := make(map[lineItemKey]uint64)
	for _, event := range events {
		key := lineItemKey{
			meterName:   event.MeterName(),
			planID:      event.PlanID(),
			planVersion: event.PlanVersion(),
		}
		totals[key] += event.Quantity()
	}
	lineItems := make([]InvoiceLineItem, 0, len(totals))
	for key, quantity := range totals {
		lineItems = append(lineItems, InvoiceLineItem{
			MeterName:   key.meterName,
			PlanID:      key.planID,
			PlanVersion: key.planVersion,
			Quantity:    quantity,
		})
	}
	sort.Slice(lineItems, func(leftIndex int, rightIndex int) bool {
		left := lineItems[leftIndex]
		right := lineItems[rightIndex]
		if left.PlanID != right.PlanID {
			return left.PlanID < right.PlanID
		}
		if left.PlanVersion != right.PlanVersion {
			return left.PlanVersion < right.PlanVersion
		}
		return left.MeterName < right.MeterName
	})
	return lineItems
}
