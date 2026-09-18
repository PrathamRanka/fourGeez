package domain

import "errors"

var (
	// ErrPermissionDenied reports a valid caller blocked by policy or entitlement.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrRateLimitExceeded reports an exhausted time-window quota.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)
