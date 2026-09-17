package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

const requestIDByteLength = 16

// ErrIdempotencyConflict reports reuse of a key with a different request.
var ErrIdempotencyConflict = errors.New("idempotency key was reused with a different request")

// StaticAuthenticator validates configured local-development credentials.
type StaticAuthenticator struct {
	sellerToken []byte
	agentKey    []byte
}

// NewStaticAuthenticator creates a constant-time local authenticator.
func NewStaticAuthenticator(sellerToken, agentKey string) *StaticAuthenticator {
	return &StaticAuthenticator{
		sellerToken: []byte(sellerToken),
		agentKey:    []byte(agentKey),
	}
}

// AuthenticateSeller validates a configured seller bearer token.
func (authenticator *StaticAuthenticator) AuthenticateSeller(
	_ context.Context,
	token string,
) (Principal, bool) {
	if !secureEqual(authenticator.sellerToken, []byte(token)) {
		return Principal{}, false
	}
	return Principal{
		Kind:    PrincipalSeller,
		Subject: "local-seller",
	}, true
}

// AuthenticateAgent validates a configured agent API key.
func (authenticator *StaticAuthenticator) AuthenticateAgent(
	_ context.Context,
	key string,
) (Principal, bool) {
	if !secureEqual(authenticator.agentKey, []byte(key)) {
		return Principal{}, false
	}
	return Principal{
		Kind:    PrincipalAgent,
		Subject: "local-agent",
	}, true
}

// secureEqual compares credentials without content-dependent timing.
func secureEqual(expected, actual []byte) bool {
	if len(expected) == 0 || len(actual) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare(expected, actual) == 1
}

// newRequestID creates an opaque correlation identifier.
func newRequestID() string {
	randomBytes := make([]byte, requestIDByteLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "request-unavailable"
	}
	return "req_" + hex.EncodeToString(randomBytes)
}

// incomingRequestID accepts a bounded printable request identifier.
func incomingRequestID(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if len(trimmedValue) < 1 || len(trimmedValue) > 128 {
		return ""
	}
	for _, character := range trimmedValue {
		if character < 0x21 || character > 0x7e {
			return ""
		}
	}
	return trimmedValue
}

// CheckIdempotency validates a mutation key and returns a stored replay when present.
func CheckIdempotency(
	ctx context.Context,
	store domain.IdempotencyStore,
	scope string,
	rawKey string,
	requestBody []byte,
) (IdempotencyDecision, error) {
	key, err := domain.ParseIdempotencyKey(rawKey)
	if err != nil {
		return IdempotencyDecision{}, err
	}

	digest := sha256.Sum256(requestBody)
	requestHash := hex.EncodeToString(digest[:])
	record, found, err := store.Load(ctx, scope, key)
	if err != nil {
		return IdempotencyDecision{}, err
	}
	if found && record.RequestHash != requestHash {
		return IdempotencyDecision{}, ErrIdempotencyConflict
	}
	if found {
		return IdempotencyDecision{
			Key:         key,
			RequestHash: requestHash,
			Replay:      true,
			Status:      record.ResponseStatus,
			Body:        record.ResponseBody,
		}, nil
	}

	return IdempotencyDecision{
		Key:         key,
		RequestHash: requestHash,
	}, nil
}

// SaveIdempotency stores a completed mutation response for later replay.
func SaveIdempotency(
	ctx context.Context,
	store domain.IdempotencyStore,
	scope string,
	decision IdempotencyDecision,
	status int,
	body []byte,
	createdAt time.Time,
) error {
	_, err := store.SaveIfAbsent(ctx, domain.IdempotencyRecord{
		Scope:          scope,
		Key:            decision.Key,
		RequestHash:    decision.RequestHash,
		ResponseStatus: status,
		ResponseBody:   append([]byte(nil), body...),
		CreatedAt:      domain.NewTimestamp(createdAt),
		ExpiresAt:      domain.NewTimestamp(createdAt.Add(24 * time.Hour)),
	})
	return err
}
