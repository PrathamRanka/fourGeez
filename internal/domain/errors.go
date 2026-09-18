package domain

import "errors"

var (
	// ErrPermissionDenied reports a valid caller blocked by policy or entitlement.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrRateLimitExceeded reports an exhausted time-window quota.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	// ErrCommerceUnavailable reports that a currently published offer cannot
	// authorize a new purchase because an authoritative prerequisite changed.
	ErrCommerceUnavailable = errors.New("commerce is unavailable")
)
