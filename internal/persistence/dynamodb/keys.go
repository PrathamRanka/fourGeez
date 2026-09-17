package dynamodb

import "fmt"

const profileSortKey = "PROFILE"

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

// paymentDestinationSortKey returns the seller payment-destination sort key.
func paymentDestinationSortKey(destinationID string) string {
	return "DESTINATION#" + destinationID
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
