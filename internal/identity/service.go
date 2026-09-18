package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

// Service validates seller identity and applies immediate session revocation.
type Service struct {
	verifier    TokenVerifier
	revocations RevocationRepository
	clock       domain.Clock
}

// NewService creates the seller identity service.
func NewService(verifier TokenVerifier, revocations RevocationRepository, clock domain.Clock) *Service {
	return &Service{verifier: verifier, revocations: revocations, clock: clock}
}

// Authenticate verifies one bearer token and rejects revoked token families.
func (service *Service) Authenticate(ctx context.Context, rawToken string) (Principal, error) {
	if service == nil || service.verifier == nil || service.revocations == nil || service.clock == nil {
		return Principal{}, ErrIdentityUnavailable
	}
	if strings.TrimSpace(rawToken) == "" {
		return Principal{}, ErrUnauthenticated
	}
	claims, err := service.verifier.Verify(ctx, rawToken)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			return Principal{}, ErrTokenExpired
		}
		if errors.Is(err, ErrIdentityUnavailable) {
			return Principal{}, ErrIdentityUnavailable
		}
		if errors.Is(err, ErrTokenRevoked) {
			return Principal{}, ErrTokenRevoked
		}
		return Principal{}, ErrUnauthenticated
	}
	if err := validateClaims(claims, service.clock.Now()); err != nil {
		return Principal{}, err
	}
	revoked, err := service.revocations.IsRevoked(ctx, sessionDigest(claims.SessionID), service.clock.Now())
	if err != nil {
		return Principal{}, ErrIdentityUnavailable
	}
	if revoked {
		return Principal{}, ErrTokenRevoked
	}
	return Principal{
		Subject: claims.Subject, Email: claims.Email, Username: claims.Username,
		TokenID: claims.TokenID, SessionID: claims.SessionID, ExpiresAt: claims.ExpiresAt,
	}, nil
}

// Revoke invalidates the complete token family represented by the current session.
func (service *Service) Revoke(ctx context.Context, principal Principal) error {
	if service == nil || service.revocations == nil || service.clock == nil {
		return ErrIdentityUnavailable
	}
	if strings.TrimSpace(principal.SessionID) == "" || !principal.ExpiresAt.After(service.clock.Now()) {
		return ErrTokenInvalid
	}
	if err := service.revocations.Revoke(ctx, sessionDigest(principal.SessionID), principal.ExpiresAt); err != nil {
		return ErrIdentityUnavailable
	}
	return nil
}

func validateClaims(claims Claims, now time.Time) error {
	if !validIdentityValue(claims.Subject) || !validIdentityValue(claims.TokenID) || !validIdentityValue(claims.SessionID) {
		return ErrTokenInvalid
	}
	if !claims.ExpiresAt.After(now) {
		return ErrTokenExpired
	}
	if !claims.IssuedAt.IsZero() && claims.IssuedAt.After(now.Add(time.Minute)) {
		return ErrTokenInvalid
	}
	return nil
}

func validIdentityValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && trimmed == value && len(value) <= maximumIdentityValueLength
}

func sessionDigest(sessionID string) string {
	digest := sha256.Sum256([]byte("agentpay.seller-session.v1\x00" + sessionID))
	return hex.EncodeToString(digest[:])
}
