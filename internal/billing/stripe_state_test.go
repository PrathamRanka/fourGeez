package billing

import (
	"context"
	"testing"
	"time"
)

func TestStripeCurrentStateReaderBuildsCompleteCandidateFromFetchedState(t *testing.T) {
	t.Parallel()

	client := &stripeSubscriptionClient{snapshot: StripeSubscriptionSnapshot{
		SellerID: mustBillingID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"), CustomerID: "cus_123",
		SubscriptionID: "sub_current", PriceID: "price_growth", Status: StripeSubscriptionStatusActive,
		CurrentPeriodStart: entitlementTime(2026, time.September, 1, 0), CurrentPeriodEnd: entitlementTime(2026, time.October, 1, 0), PaidThrough: entitlementTime(2026, time.October, 1, 0),
		CancelAtPeriodEnd: true, LatestInvoicePaid: true,
	}}
	reader := NewStripeCurrentStateReader(client, map[string]PlanID{"price_growth": PlanGrowth})
	candidate, err := reader.CurrentEntitlement(t.Context(), SubscriptionProviderEvent{EventID: "evt_old", SubscriptionID: "sub_old"})
	if err != nil {
		t.Fatal(err)
	}
	if client.requestedSubscriptionID != "sub_old" || candidate.SellerID != client.snapshot.SellerID || candidate.PlanID != PlanGrowth || candidate.AccessEndsAt != client.snapshot.PaidThrough || !candidate.CancelAtPeriodEnd {
		t.Fatalf("candidate = %#v", candidate)
	}
}

type stripeSubscriptionClient struct {
	snapshot                StripeSubscriptionSnapshot
	requestedSubscriptionID string
}

func (client *stripeSubscriptionClient) FetchSubscription(_ context.Context, subscriptionID string) (StripeSubscriptionSnapshot, error) {
	client.requestedSubscriptionID = subscriptionID
	return client.snapshot, nil
}
