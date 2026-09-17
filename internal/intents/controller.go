package intents

// NewPurchaseIntent validates, hashes, and freezes a purchase proposal.
func NewPurchaseIntent(params PurchaseIntentParams) (PurchaseIntent, error) {
	return createPurchaseIntent(params)
}

// ParseSHA256Digest validates a digest received at a system boundary.
func ParseSHA256Digest(raw string) (SHA256Digest, error) {
	return parseSHA256Digest(raw)
}

// HashRequestBody hashes JSON canonically and other media types byte-for-byte.
func HashRequestBody(requestBody []byte, mediaType string) (SHA256Digest, error) {
	return hashRequestBody(requestBody, mediaType)
}
