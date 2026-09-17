package agentpayverify

import (
	"context"
	"errors"
	"time"
)

const (
	SignatureHeader     = "X-AgentPay-Signature"
	TimestampHeader     = "X-AgentPay-Timestamp"
	TransactionIDHeader = "X-AgentPay-Transaction-Id"
	minimumSecretBytes  = 32
	defaultMaximumAge   = 5 * time.Minute
	defaultMaximumBody  = 1 << 20
)

var (
	ErrInvalidSignature = errors.New("invalid AgentPay signature")
	ErrStaleRequest     = errors.New("stale AgentPay request")
	ErrReplay           = errors.New("AgentPay transaction replay")
)

// ReplayStore atomically claims transaction identifiers after verification.
type ReplayStore interface {
	Claim(context.Context, string, time.Time) (bool, error)
}

// Config contains verifier dependencies and security limits.
type Config struct {
	Secret           []byte
	MaximumAge       time.Duration
	MaximumBodyBytes int64
	ReplayStore      ReplayStore
	Clock            func() time.Time
}
