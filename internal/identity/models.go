package identity

import (
	"context"
	"errors"
	"time"
)

const maximumIdentityValueLength = 256

var (
	ErrIdentityUnavailable = errors.New("identity service is unavailable")
	ErrTokenExpired        = errors.New("seller token has expired")
	ErrTokenInvalid        = errors.New("seller token is invalid")
	ErrTokenRevoked        = errors.New("seller session is revoked")
	ErrUnauthenticated     = errors.New("seller authentication is required")
)

// Claims are the normalized production and local seller-token claims.
type Claims struct {
	Subject   string
	Email     string
	Username  string
	TokenID   string
	SessionID string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Principal is the authenticated seller identity used by the HTTP boundary.
type Principal struct {
	Subject   string
	Email     string
	Username  string
	TokenID   string
	SessionID string
	ExpiresAt time.Time
}

// TokenVerifier verifies an access token at the identity-provider boundary.
type TokenVerifier interface {
	Verify(context.Context, string) (Claims, error)
}

// RevocationRepository stores hashed seller-session revocations.
type RevocationRepository interface {
	IsRevoked(context.Context, string, time.Time) (bool, error)
	Revoke(context.Context, string, time.Time) error
}
