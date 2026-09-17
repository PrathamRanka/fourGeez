package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

const requestIDByteLength = 16

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
