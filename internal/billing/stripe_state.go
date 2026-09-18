package billing

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/domain"
)

var (
	ErrStripeSubscriptionUnavailable = errors.New("Stripe subscription is unavailable")
	ErrStripePriceUnmapped           = errors.New("Stripe price is not mapped to an AgentPay plan")
)

type StripeSubscriptionStatus string

const (
	StripeSubscriptionStatusActive            StripeSubscriptionStatus = "active"
	StripeSubscriptionStatusPastDue           StripeSubscriptionStatus = "past_due"
	StripeSubscriptionStatusUnpaid            StripeSubscriptionStatus = "unpaid"
	StripeSubscriptionStatusCanceled          StripeSubscriptionStatus = "canceled"
	StripeSubscriptionStatusPaused            StripeSubscriptionStatus = "paused"
	StripeSubscriptionStatusIncomplete        StripeSubscriptionStatus = "incomplete"
	StripeSubscriptionStatusIncompleteExpired StripeSubscriptionStatus = "incomplete_expired"
	StripeSubscriptionStatusTrialing          StripeSubscriptionStatus = "trialing"
)

type StripeSubscriptionSnapshot struct {
	SellerID           domain.ID
	CustomerID         string
	SubscriptionID     string
	PriceID            string
	Status             StripeSubscriptionStatus
	CurrentPeriodStart domain.Timestamp
	CurrentPeriodEnd   domain.Timestamp
	PaidThrough        domain.Timestamp
	LatestInvoicePaid  bool
	CancelAtPeriodEnd  bool
}

type StripeSubscriptionClient interface {
	FetchSubscription(context.Context, string) (StripeSubscriptionSnapshot, error)
}

type StripeCurrentStateReader struct {
	client     StripeSubscriptionClient
	pricePlans map[string]PlanID
}

func NewStripeCurrentStateReader(client StripeSubscriptionClient, pricePlans map[string]PlanID) *StripeCurrentStateReader {
	isolatedPricePlans := make(map[string]PlanID, len(pricePlans))
	for priceID, planID := range pricePlans {
		isolatedPricePlans[priceID] = planID
	}
	return &StripeCurrentStateReader{client: client, pricePlans: isolatedPricePlans}
}

func (reader *StripeCurrentStateReader) CurrentEntitlement(
	ctx context.Context,
	event SubscriptionProviderEvent,
) (EntitlementCandidate, error) {
	if event.SubscriptionID == "" {
		return EntitlementCandidate{}, ErrStripeSubscriptionUnavailable
	}
	subscription, err := reader.client.FetchSubscription(ctx, event.SubscriptionID)
	if err != nil {
		return EntitlementCandidate{}, err
	}
	planID, exists := reader.pricePlans[subscription.PriceID]
	if !exists {
		return EntitlementCandidate{}, ErrStripePriceUnmapped
	}
	status, reason := stripeEntitlementStatus(subscription)
	return EntitlementCandidate{
		SellerID: subscription.SellerID, PlanID: planID, PlanVersion: currentPlanVersion,
		Status: status, BillingPeriodStart: subscription.CurrentPeriodStart,
		BillingPeriodEnd: subscription.CurrentPeriodEnd, AccessEndsAt: subscription.PaidThrough,
		CancelAtPeriodEnd: subscription.CancelAtPeriodEnd, Source: EntitlementSourceBillingProvider,
		StatusReason: reason, Provider: EntitlementProviderStripe,
		ProviderCustomerID: subscription.CustomerID, ProviderSubscriptionID: subscription.SubscriptionID,
		ProviderPriceID: subscription.PriceID, LastProviderEventID: event.EventID,
	}, nil
}

func stripeEntitlementStatus(subscription StripeSubscriptionSnapshot) (EntitlementStatus, EntitlementStatusReason) {
	switch subscription.Status {
	case StripeSubscriptionStatusActive:
		if subscription.LatestInvoicePaid {
			return EntitlementStatusActive, ""
		}
		return EntitlementStatusActive, EntitlementStatusReasonPaymentFailed
	case StripeSubscriptionStatusPastDue:
		return EntitlementStatusActive, EntitlementStatusReasonPaymentFailed
	case StripeSubscriptionStatusUnpaid:
		return EntitlementStatusSuspended, EntitlementStatusReasonPaymentFailed
	case StripeSubscriptionStatusCanceled:
		return EntitlementStatusCancelled, EntitlementStatusReasonCancelled
	case StripeSubscriptionStatusPaused:
		return EntitlementStatusSuspended, EntitlementStatusReasonAdministrative
	case StripeSubscriptionStatusIncomplete, StripeSubscriptionStatusIncompleteExpired, StripeSubscriptionStatusTrialing:
		return EntitlementStatusSuspended, EntitlementStatusReasonProviderIncomplete
	default:
		return EntitlementStatusSuspended, EntitlementStatusReasonProviderIncomplete
	}
}
