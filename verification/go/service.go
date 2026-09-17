package agentpayverify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

const signatureDomain = "agentpay.seller-request.v1"

// Verifier validates AgentPay seller-request signatures and replay state.
type Verifier struct {
	secret           []byte
	maximumAge       time.Duration
	maximumBodyBytes int64
	replayStore      ReplayStore
	clock            func() time.Time
}

// NewVerifier validates configuration and creates a request verifier.
func NewVerifier(config Config) (*Verifier, error) {
	if len(config.Secret) < minimumSecretBytes {
		return nil, errors.New("AgentPay verification secret must contain at least 32 bytes")
	}
	if config.MaximumAge <= 0 {
		config.MaximumAge = defaultMaximumAge
	}
	if config.MaximumBodyBytes <= 0 {
		config.MaximumBodyBytes = defaultMaximumBody
	}
	if config.ReplayStore == nil {
		return nil, errors.New("AgentPay replay store is required")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	return &Verifier{
		secret:           append([]byte(nil), config.Secret...),
		maximumAge:       config.MaximumAge,
		maximumBodyBytes: config.MaximumBodyBytes,
		replayStore:      config.ReplayStore,
		clock:            config.Clock,
	}, nil
}

// Verify validates one exact request and atomically claims its transaction.
func (verifier *Verifier) Verify(
	ctx context.Context,
	method string,
	path string,
	body []byte,
	signatureValue string,
	timestampValue string,
	transactionID string,
) error {
	if strings.TrimSpace(transactionID) == "" {
		return ErrInvalidSignature
	}
	timestamp, err := time.Parse(time.RFC3339, timestampValue)
	if err != nil {
		return ErrInvalidSignature
	}
	now := verifier.clock().UTC()
	requestAge := now.Sub(timestamp.UTC())
	if requestAge < -verifier.maximumAge || requestAge > verifier.maximumAge {
		return ErrStaleRequest
	}
	providedSignature, err := base64.StdEncoding.DecodeString(signatureValue)
	if err != nil {
		return ErrInvalidSignature
	}
	bodyDigest := sha256.Sum256(body)
	canonical := strings.Join([]string{
		signatureDomain,
		timestampValue,
		strings.ToUpper(method),
		path,
		hex.EncodeToString(bodyDigest[:]),
		transactionID,
	}, "\n")
	expectedSignature := hmac.New(sha256.New, verifier.secret)
	_, _ = expectedSignature.Write([]byte(canonical))
	if !hmac.Equal(providedSignature, expectedSignature.Sum(nil)) {
		return ErrInvalidSignature
	}
	claimed, err := verifier.replayStore.Claim(ctx, transactionID, timestamp.UTC())
	if err != nil {
		return err
	}
	if !claimed {
		return ErrReplay
	}
	return nil
}

// MemoryReplayStore provides process-local atomic replay protection for tests.
type MemoryReplayStore struct {
	mutex        sync.Mutex
	transactions map[string]time.Time
}

// NewMemoryReplayStore creates an empty process-local replay store.
func NewMemoryReplayStore() *MemoryReplayStore {
	return &MemoryReplayStore{transactions: make(map[string]time.Time)}
}

// Claim records a transaction only when it has not already been accepted.
func (store *MemoryReplayStore) Claim(
	_ context.Context,
	transactionID string,
	timestamp time.Time,
) (bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if _, exists := store.transactions[transactionID]; exists {
		return false, nil
	}
	store.transactions[transactionID] = timestamp
	return true, nil
}
