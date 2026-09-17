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

const intentHashDomain = "agentpay.intent.v1"

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// SHA256Digest is a lowercase SHA-256 digest encoded as hexadecimal.
type SHA256Digest string

// ParseSHA256Digest validates a digest received at a system boundary.
func ParseSHA256Digest(raw string) (SHA256Digest, error) {
	if !sha256Pattern.MatchString(raw) {
		return "", domain.NewValidationError("sha256", "format", "must contain 64 lowercase hexadecimal characters")
	}
	return SHA256Digest(raw), nil
}

// String returns the lowercase hexadecimal digest.
func (digest SHA256Digest) String() string {
	return string(digest)
}

// MarshalJSON serializes the digest as a string.
func (digest SHA256Digest) MarshalJSON() ([]byte, error) {
	return json.Marshal(digest.String())
}

// UnmarshalJSON validates a digest from a JSON string.
func (digest *SHA256Digest) UnmarshalJSON(encoded []byte) error {
	var raw string
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return domain.NewValidationError("sha256", "format", "must be a JSON string containing a SHA-256 digest")
	}
	parsed, err := ParseSHA256Digest(raw)
	if err != nil {
		return err
	}
	*digest = parsed
	return nil
}

// HashRequestBody hashes JSON canonically and all other media types byte-for-byte.
func HashRequestBody(requestBody []byte, mediaType string) (SHA256Digest, error) {
	canonicalBody := requestBody
	if mediaType != "" {
		parsedMediaType, _, err := mime.ParseMediaType(mediaType)
		if err != nil {
			return "", domain.NewValidationError("requestContentType", "format", "must be a valid media type")
		}
		if parsedMediaType == "application/json" || strings.HasSuffix(parsedMediaType, "+json") {
			canonicalBody, err = jcs.Transform(requestBody)
			if err != nil {
				return "", domain.NewValidationError("requestBody", "json", "must contain valid canonicalizable JSON")
			}
		}
	}
	return hashBytes(canonicalBody), nil
}

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

func hashBytes(value []byte) SHA256Digest {
	digest := sha256.Sum256(value)
	return SHA256Digest(hex.EncodeToString(digest[:]))
}
