package settlement

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	OwnershipChallengeLifetime = 10 * time.Minute
	ownershipNonceBytes        = 32
	maximumChallengeLength     = 2048
	maximumSignatureLength     = 4096
)

// SecureOwnershipNonceGenerator creates URL-safe nonce values from cryptographic randomness.
type SecureOwnershipNonceGenerator struct {
	reader io.Reader
}

// NewSecureOwnershipNonceGenerator uses crypto/rand when no reader is supplied.
func NewSecureOwnershipNonceGenerator(reader io.Reader) *SecureOwnershipNonceGenerator {
	if reader == nil {
		reader = cryptorand.Reader
	}
	return &SecureOwnershipNonceGenerator{reader: reader}
}

// NewNonce returns 256 bits of URL-safe challenge entropy.
func (generator *SecureOwnershipNonceGenerator) NewNonce() (string, error) {
	randomBytes := make([]byte, ownershipNonceBytes)
	if _, err := io.ReadFull(generator.reader, randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

// buildOwnershipChallenge binds a nonce to the complete public destination identity.
func buildOwnershipChallenge(
	destination PaymentDestination,
	nonce string,
	expiresAt domain.Timestamp,
) string {
	return fmt.Sprintf(
		"AgentPay payment destination ownership\nVersion: 1\nSeller: %s\nDestination: %s\nAsset: %s\nNetwork: %s\nAddress: %s\nNonce: %s\nExpires At: %s",
		destination.SellerID.String(),
		destination.DestinationID.String(),
		destination.Asset,
		destination.Network,
		destination.Address,
		nonce,
		expiresAt.String(),
	)
}

// ownershipChallengeHash returns the lowercase SHA-256 digest stored at rest.
func ownershipChallengeHash(challenge string) string {
	digest := sha256.Sum256([]byte(challenge))
	return hex.EncodeToString(digest[:])
}

// ownershipChallengeMatches compares challenge digests without data-dependent timing.
func ownershipChallengeMatches(challenge string, expectedHash string) bool {
	expectedDigest, err := hex.DecodeString(expectedHash)
	if err != nil || len(expectedDigest) != sha256.Size {
		return false
	}
	actualDigest := sha256.Sum256([]byte(challenge))
	return subtle.ConstantTimeCompare(expectedDigest, actualDigest[:]) == 1
}

// validateOwnershipProofInput bounds untrusted proof material before cryptographic work.
func validateOwnershipProofInput(request VerifyOwnershipRequest) error {
	validationErrors := make(domain.ValidationErrors, 0)
	if strings.TrimSpace(request.Challenge) == "" {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("challenge", "required", "is required"),
		)
	} else if len(request.Challenge) > maximumChallengeLength {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("challenge", "length", "exceeds the maximum length"),
		)
	}
	if strings.TrimSpace(request.Signature) == "" {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("signature", "required", "is required"),
		)
	} else if len(request.Signature) > maximumSignatureLength {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError("signature", "length", "exceeds the maximum length"),
		)
	}
	if len(validationErrors) > 0 {
		return validationErrors
	}
	return nil
}
