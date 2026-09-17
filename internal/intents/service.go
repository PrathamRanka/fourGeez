package intents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime"
	"regexp"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/gowebpki/jcs"
)

const (
	intentHashDomain     = "agentpay.intent.v1"
	intentSchemaVersion  = "1"
	maximumBuyerIDLength = 160
	maximumAssetLength   = 160
	maximumNetworkLength = 80
)

var (
	requestPathPattern = regexp.MustCompile(`^/[A-Za-z0-9/_-]+$`)
	sha256Pattern      = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// createPurchaseIntent constructs an immutable validated purchase proposal.
func createPurchaseIntent(params PurchaseIntentParams) (PurchaseIntent, error) {
	validationErrors := validatePurchaseIntentParams(params)
	if len(validationErrors) > 0 {
		return PurchaseIntent{}, validationErrors
	}

	status := PurchaseIntentStatusReady
	if params.RequiresApproval {
		status = PurchaseIntentStatusApprovalPending
	}

	intentHash, err := hashCanonicalValue(intentHashDomain, intentHashPayload{
		SchemaVersion:    intentSchemaVersion,
		IntentID:         params.IntentID.String(),
		SellerID:         params.SellerID.String(),
		RouteID:          params.RouteID.String(),
		BuyerID:          strings.TrimSpace(params.BuyerID),
		RequestMethod:    params.RequestMethod,
		RequestPath:      params.RequestPath,
		RequestBodyHash:  params.RequestBodyHash.String(),
		Amount:           params.Amount.String(),
		Asset:            strings.TrimSpace(params.Asset),
		Network:          strings.TrimSpace(params.Network),
		MaximumAmount:    params.MaximumAmount.String(),
		RequiresApproval: params.RequiresApproval,
		CreatedAt:        params.CreatedAt.String(),
		ExpiresAt:        params.ExpiresAt.String(),
	})
	if err != nil {
		return PurchaseIntent{}, err
	}

	return PurchaseIntent{
		intentID:         params.IntentID,
		sellerID:         params.SellerID,
		routeID:          params.RouteID,
		buyerID:          strings.TrimSpace(params.BuyerID),
		requestMethod:    params.RequestMethod,
		requestPath:      params.RequestPath,
		requestBodyHash:  params.RequestBodyHash,
		amount:           params.Amount,
		asset:            strings.TrimSpace(params.Asset),
		network:          strings.TrimSpace(params.Network),
		maximumAmount:    params.MaximumAmount,
		requiresApproval: params.RequiresApproval,
		intentHash:       intentHash,
		status:           status,
		createdAt:        params.CreatedAt,
		expiresAt:        params.ExpiresAt,
	}, nil
}

// parseSHA256Digest validates the canonical lowercase digest representation.
func parseSHA256Digest(raw string) (SHA256Digest, error) {
	if !sha256Pattern.MatchString(raw) {
		return "", domain.NewValidationError(
			"sha256",
			"format",
			"must contain 64 lowercase hexadecimal characters",
		)
	}
	return SHA256Digest(raw), nil
}

// hashRequestBody canonicalizes JSON before hashing request bytes.
func hashRequestBody(requestBody []byte, mediaType string) (SHA256Digest, error) {
	canonicalBody := requestBody
	if mediaType != "" {
		parsedMediaType, _, err := mime.ParseMediaType(mediaType)
		if err != nil {
			return "", domain.NewValidationError(
				"requestContentType",
				"format",
				"must be a valid media type",
			)
		}
		if parsedMediaType == "application/json" || strings.HasSuffix(parsedMediaType, "+json") {
			canonicalBody, err = jcs.Transform(requestBody)
			if err != nil {
				return "", domain.NewValidationError(
					"requestBody",
					"json",
					"must contain valid canonicalizable JSON",
				)
			}
		}
	}
	return hashBytes(canonicalBody), nil
}

// hashCanonicalValue hashes a JCS value with an unambiguous domain separator.
func hashCanonicalValue(domainSeparator string, value any) (SHA256Digest, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(encoded)
	if err != nil {
		return "", err
	}

	payload := make([]byte, 0, len(domainSeparator)+1+len(canonical))
	payload = append(payload, domainSeparator...)
	payload = append(payload, 0)
	payload = append(payload, canonical...)
	return hashBytes(payload), nil
}

// hashBytes returns the lowercase SHA-256 digest of value.
func hashBytes(value []byte) SHA256Digest {
	digest := sha256.Sum256(value)
	return SHA256Digest(hex.EncodeToString(digest[:]))
}

// validatePurchaseIntentParams validates every execution-relevant field.
func validatePurchaseIntentParams(params PurchaseIntentParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.IntentID.String(), domain.IntentIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("intentId", "format", "must be a purchase-intent identifier"))
	}
	if _, err := domain.ParseID(params.SellerID.String(), domain.SellerIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "format", "must be a seller identifier"))
	}
	if _, err := domain.ParseID(params.RouteID.String(), domain.RouteIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("routeId", "format", "must be a route identifier"))
	}

	buyerID := strings.TrimSpace(params.BuyerID)
	if buyerID == "" || len(buyerID) > maximumBuyerIDLength {
		validationErrors = append(validationErrors, domain.NewValidationError("buyerId", "length", "must contain 1-160 characters"))
	}
	if params.RequestMethod != RequestMethodGet && params.RequestMethod != RequestMethodPost {
		validationErrors = append(validationErrors, domain.NewValidationError("requestMethod", "supported", "must be GET or POST"))
	}
	if !requestPathPattern.MatchString(params.RequestPath) {
		validationErrors = append(validationErrors, domain.NewValidationError("requestPath", "format", "must be a literal absolute route path"))
	}
	if !sha256Pattern.MatchString(params.RequestBodyHash.String()) {
		validationErrors = append(validationErrors, domain.NewValidationError("requestBodyHash", "format", "must be a SHA-256 digest"))
	}
	if params.Amount.IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("amount", "positive", "must be greater than zero"))
	}
	if params.Amount.Compare(params.MaximumAmount) > 0 {
		validationErrors = append(validationErrors, domain.NewValidationError("maximumAmount", "limit", "must be greater than or equal to the seller's exact price"))
	}
	if value := strings.TrimSpace(params.Asset); value == "" || len(value) > maximumAssetLength {
		validationErrors = append(validationErrors, domain.NewValidationError("asset", "length", "must contain 1-160 characters"))
	}
	if value := strings.TrimSpace(params.Network); value == "" || len(value) > maximumNetworkLength {
		validationErrors = append(validationErrors, domain.NewValidationError("network", "length", "must contain 1-80 characters"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	if params.ExpiresAt.Time().IsZero() || !params.ExpiresAt.Time().After(params.CreatedAt.Time()) {
		validationErrors = append(validationErrors, domain.NewValidationError("expiresAt", "chronology", "must occur after creation"))
	}
	return validationErrors
}
