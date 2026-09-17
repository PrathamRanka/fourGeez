package notifications

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/gowebpki/jcs"
)

const webhookSignatureDomain = "agentpay.webhook.v1"

// HMACEventSigner signs canonical webhook bodies with referenced seller secrets.
type HMACEventSigner struct {
	secretProvider SecretProvider
	clock          domain.Clock
}

// NewHMACEventSigner creates the outbound webhook signer.
func NewHMACEventSigner(
	secretProvider SecretProvider,
	clock domain.Clock,
) *HMACEventSigner {
	return &HMACEventSigner{
		secretProvider: secretProvider,
		clock:          clock,
	}
}

// Sign canonicalizes one event and returns replay-safe signature headers.
func (signer *HMACEventSigner) Sign(
	ctx context.Context,
	secretRef string,
	event WebhookEvent,
) (SignedEvent, error) {
	secret, err := signer.secretProvider.GetSecret(ctx, secretRef)
	if err != nil || len(secret) < minimumWebhookSecretLength {
		return SignedEvent{}, ErrSigningUnavailable
	}
	canonicalBody, err := CanonicalEventBody(event)
	if err != nil {
		return SignedEvent{}, err
	}
	timestamp := domain.NewTimestamp(signer.clock.Now()).String()
	headers := SignatureHeaders{
		EventID:   event.EventID.String(),
		Timestamp: timestamp,
		Signature: calculateWebhookHMAC(secret, canonicalBody, event.EventID.String(), timestamp),
	}
	return SignedEvent{Body: canonicalBody, Headers: headers}, nil
}

// CanonicalEventBody validates and canonicalizes one public webhook event.
func CanonicalEventBody(event WebhookEvent) ([]byte, error) {
	if event.SchemaVersion != "1" ||
		event.EventID.Prefix() != domain.EvidenceIDPrefix ||
		event.SellerID.Prefix() != domain.SellerIDPrefix ||
		!validEventType(event.EventType) ||
		event.OccurredAt.Time().IsZero() ||
		event.Payload == nil {
		return nil, domain.NewValidationError(
			"event",
			"contract",
			"must satisfy the webhook event contract",
		)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return jcs.Transform(encoded)
}

// VerifyWebhookSignature verifies canonical webhook bytes in constant time.
func VerifyWebhookSignature(
	secret []byte,
	body []byte,
	headers SignatureHeaders,
) bool {
	if len(secret) < minimumWebhookSecretLength ||
		strings.TrimSpace(headers.EventID) == "" ||
		strings.TrimSpace(headers.Timestamp) == "" {
		return false
	}
	expected, err := base64.StdEncoding.DecodeString(
		calculateWebhookHMAC(secret, body, headers.EventID, headers.Timestamp),
	)
	if err != nil {
		return false
	}
	supplied, err := base64.StdEncoding.DecodeString(headers.Signature)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, supplied)
}

// calculateWebhookHMAC signs the domain, event identity, timestamp, and body digest.
func calculateWebhookHMAC(
	secret []byte,
	body []byte,
	eventID string,
	timestamp string,
) string {
	bodyDigest := sha256.Sum256(body)
	message := webhookSignatureDomain + "\n" + eventID + "\n" + timestamp + "\n" +
		hex.EncodeToString(bodyDigest[:])
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
