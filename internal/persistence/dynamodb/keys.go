package dynamodb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// usageMeterSortKey returns the chronological immutable usage key.
func usageMeterSortKey(occurredAt time.Time, meterEventID string) string {
	return "METER#" + occurredAt.UTC().Format(time.RFC3339Nano) + "#" + meterEventID
}

// usageMeterSourceSortKey returns the unique meter-source claim key.
func usageMeterSourceSortKey(meterName string, sourceID string) string {
	return "METER_SOURCE#" + meterName + "#" + sourceID
}

// quotaCounterSortKey returns one seller UTC-month quota key.
func quotaCounterSortKey(period string, quotaName string) string {
	return "QUOTA#" + period + "#" + quotaName
}

// quotaClaimSortKey hashes one retry-safe source into a bounded claim key.
func quotaClaimSortKey(period string, quotaName string, source string) string {
	digest := sha256.Sum256([]byte(source))
	return "QUOTA_CLAIM#" + period + "#" + quotaName + "#" + hex.EncodeToString(digest[:])
}

// auditEventSortKey returns the chronological immutable audit key.
func auditEventSortKey(occurredAt time.Time, auditEventID string) string {
	return "AUDIT#" + occurredAt.UTC().Format(time.RFC3339Nano) + "#" + auditEventID
}

const profileSortKey = "PROFILE"

const sellerPlanSortKey = "BILLING_PLAN"

func publicationEventPartitionKey(eventID string) string {
	return "PUBLICATION_EVENT#" + eventID
}

const publicationCompletionSortKey = "COMPLETION"

func subscriptionReconciliationSortKey(sourceRevision string) string {
	return "SUBSCRIPTION_RECONCILIATION#" + sourceRevision
}

func launchEntitlementOperationSortKey(operationID string) string {
	return "LAUNCH_ENTITLEMENT_OPERATION#" + operationID
}

const providerEventInboxSortKey = "INBOX"

func providerEventPartitionKey(eventID string) string {
	return "STRIPE_EVENT#" + eventID
}

// sellerPartitionKey returns the documented seller partition key.
func sellerPartitionKey(sellerID string) string {
	return "SELLER#" + sellerID
}

func ownerSubjectPartitionKey(ownerSubject string) string {
	digest := sha256.Sum256([]byte("agentpay.seller-owner.v1\x00" + ownerSubject))
	return "SELLER_OWNER#" + hex.EncodeToString(digest[:])
}

const ownerSubjectSellerSortKey = "SELLER"

func sellerSessionPartitionKey(sessionDigest string) string {
	return "SELLER_SESSION#" + sessionDigest
}

const sellerSessionRevocationSortKey = "REVOCATION"

// routeSortKey returns the documented paid-route sort key.
func routeSortKey(routeID string) string {
	return "ROUTE#" + routeID
}

// productSlugClaimSortKey reserves one public product slug within a seller.
func productSlugClaimSortKey(productSlug string) string {
	return "PRODUCT_SLUG#" + productSlug
}

// credentialSortKey returns the documented integration-credential sort key.
func credentialSortKey(credentialID string) string {
	return "CREDENTIAL#" + credentialID
}

func credentialPartitionKey(credentialID string) string {
	return "CREDENTIAL#" + credentialID
}

const credentialLookupSortKey = "LOOKUP"

func projectKeyExchangeRateLimitSortKey(windowStart string) string {
	return "RATE_LIMIT#PROJECT_KEY_EXCHANGE#" + windowStart
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

func purchaseSessionPartitionKey(purchaseSessionID string) string {
	return "PURCHASE_SESSION#" + purchaseSessionID
}

func browserGrantPartitionKey(grantHash string) string {
	return "BROWSER_GRANT#" + grantHash
}

func browserGrantPurchaseSortKey(purchaseSessionID string) string {
	return "PURCHASE#" + purchaseSessionID
}

func browserRecoverySortKey(challengeID string) string {
	return "RECOVERY#" + challengeID
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

func mcpConfirmationPartitionKey(grantID string) string {
	return "MCP_CONFIRMATION#" + grantID
}

func mcpConfirmationBindingSortKey(bindingHash string) string {
	return "MCP_CONFIRMATION_BINDING#" + bindingHash
}

// paymentPartitionKey returns the payment-identifier uniqueness key.
func paymentPartitionKey(paymentIdentifier string) string {
	return "PAYMENT#" + paymentIdentifier
}

// slugPartitionKey returns the storefront-slug uniqueness key.
func slugPartitionKey(slug string) string {
	return "SLUG#" + slug
}
