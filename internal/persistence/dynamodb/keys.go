package dynamodb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const profileSortKey = "PROFILE"

const sellerPlanSortKey = "BILLING_PLAN"

// sellerPartitionKey returns the documented seller partition key.
func sellerPartitionKey(sellerID string) string {
	return "SELLER#" + sellerID
}

// routeSortKey returns the documented paid-route sort key.
func routeSortKey(routeID string) string {
	return "ROUTE#" + routeID
}

// credentialSortKey returns the documented integration-credential sort key.
func credentialSortKey(credentialID string) string {
	return "CREDENTIAL#" + credentialID
}

// webhookSubscriptionSortKey returns the seller webhook subscription key.
func webhookSubscriptionSortKey(subscriptionID string) string {
	return "WEBHOOK#" + subscriptionID
}

// webhookDeliverySortKey returns the seller webhook delivery key.
func webhookDeliverySortKey(deliveryID string) string {
	return "WEBHOOK_DELIVERY#" + deliveryID
}

// webhookEventClaimSortKey returns the unique subscription-event delivery key.
func webhookEventClaimSortKey(subscriptionID string, eventID string) string {
	return "WEBHOOK_EVENT#" + subscriptionID + "#" + eventID
}

// paymentDestinationSortKey returns the seller payment-destination sort key.
func paymentDestinationSortKey(destinationID string) string {
	return "DESTINATION#" + destinationID
}

// activePaymentDestinationSortKey returns the uniqueness key for an asset and network pair.
func activePaymentDestinationSortKey(asset string, network string) string {
	digest := sha256.Sum256([]byte(asset + "\x00" + network))
	return "DESTINATION_ACTIVE#" + hex.EncodeToString(digest[:])
}

// intentPartitionKey returns the documented purchase-intent partition key.
func intentPartitionKey(intentID string) string {
	return "INTENT#" + intentID
}

// approvalPartitionKey returns the documented approval-session partition key.
func approvalPartitionKey(sessionID string) string {
	return "APPROVAL#" + sessionID
}

// transactionPartitionKey returns the documented transaction partition key.
func transactionPartitionKey(transactionID string) string {
	return "TXN#" + transactionID
}

// eventSortKey returns the zero-padded evidence event sort key.
func eventSortKey(sequence uint64) string {
	return fmt.Sprintf("EVENT#%06d", sequence)
}

// disputePartitionKey returns the documented dispute partition key.
func disputePartitionKey(disputeID string) string {
	return "DISPUTE#" + disputeID
}

// idempotencyPartitionKey returns the documented idempotency partition key.
func idempotencyPartitionKey(scope string) string {
	return "IDEMPOTENCY#" + scope
}

// paymentPartitionKey returns the payment-identifier uniqueness key.
func paymentPartitionKey(paymentIdentifier string) string {
	return "PAYMENT#" + paymentIdentifier
}

// slugPartitionKey returns the storefront-slug uniqueness key.
func slugPartitionKey(slug string) string {
	return "SLUG#" + slug
}
