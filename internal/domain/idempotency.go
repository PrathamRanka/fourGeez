package domain

import (
	"context"
	"regexp"
)

const (
	minimumIdempotencyKeyLength = 8
	maximumIdempotencyKeyLength = 128
)

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

// IdempotencyKey is a validated caller-supplied mutation key.
type IdempotencyKey string

// ParseIdempotencyKey validates the public Idempotency-Key contract.
func ParseIdempotencyKey(raw string) (IdempotencyKey, error) {
	if len(raw) < minimumIdempotencyKeyLength || len(raw) > maximumIdempotencyKeyLength || !idempotencyKeyPattern.MatchString(raw) {
		return "", NewValidationError("idempotencyKey", "format", "must be 8-128 characters using letters, digits, dot, underscore, colon, or hyphen")
	}
	return IdempotencyKey(raw), nil
}

// IdempotencyRecord stores the original result for a caller, operation scope,
// key, and request hash combination.
type IdempotencyRecord struct {
	Scope          string
	Key            IdempotencyKey
	RequestHash    string
	ResponseStatus int
	ResponseBody   []byte
	CreatedAt      Timestamp
	ExpiresAt      Timestamp
}

// IdempotencyStore is the persistence boundary for mutation replay protection.
type IdempotencyStore interface {
	Load(ctx context.Context, scope string, key IdempotencyKey) (record IdempotencyRecord, found bool, err error)
	SaveIfAbsent(ctx context.Context, record IdempotencyRecord) (created bool, err error)
}
