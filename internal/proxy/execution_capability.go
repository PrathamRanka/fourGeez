package proxy

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const (
	ExecutionCapabilityType      = "agentpay-execution+jwt"
	executionCapabilityAlgorithm = "ES256"
	minimumExecutionLifetime     = 30 * time.Second
	maximumExecutionLifetime     = 60 * time.Second
	maximumExecutionTokenBytes   = 16 * 1024
)

var ErrExecutionCapabilityUnavailable = errors.New("execution capability is unavailable")

type ExecutionCapabilityConfig struct {
	Issuer   string
	Lifetime time.Duration
}

type ExecutionCapabilityKeySigner interface {
	CurrentKeyID(context.Context) (string, error)
	Sign(context.Context, string, []byte) ([]byte, error)
}

type ES256ExecutionCapabilitySigner struct {
	config     ExecutionCapabilityConfig
	keySigner  ExecutionCapabilityKeySigner
	clock      domain.Clock
	randomness io.Reader
}

type executionCapabilityHeader struct {
	Type      string `json:"typ"`
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type executionCapabilityClaims struct {
	Issuer          string                       `json:"iss"`
	Audience        string                       `json:"aud"`
	Subject         string                       `json:"sub"`
	SellerID        string                       `json:"sellerId"`
	RouteID         string                       `json:"routeId"`
	TransactionID   string                       `json:"transactionId"`
	Method          string                       `json:"method"`
	Path            string                       `json:"path"`
	BodySHA256      string                       `json:"bodySha256"`
	PaymentFinality transactions.PaymentFinality `json:"paymentFinality"`
	JWTID           string                       `json:"jti"`
	IssuedAt        int64                        `json:"iat"`
	ExpiresAt       int64                        `json:"exp"`
}

func NewES256ExecutionCapabilitySigner(
	config ExecutionCapabilityConfig,
	keySigner ExecutionCapabilityKeySigner,
	clock domain.Clock,
	randomness io.Reader,
) (*ES256ExecutionCapabilitySigner, error) {
	config.Issuer = strings.TrimRight(strings.TrimSpace(config.Issuer), "/")
	if config.Issuer == "" || config.Lifetime < minimumExecutionLifetime ||
		config.Lifetime > maximumExecutionLifetime || keySigner == nil {
		return nil, ErrExecutionCapabilityUnavailable
	}
	if clock == nil {
		clock = domain.SystemClock{}
	}
	if randomness == nil {
		randomness = cryptorand.Reader
	}
	return &ES256ExecutionCapabilitySigner{
		config: config, keySigner: keySigner, clock: clock, randomness: randomness,
	}, nil
}

func (signer *ES256ExecutionCapabilitySigner) Sign(
	ctx context.Context,
	_ string,
	input SigningInput,
) (SignatureHeaders, error) {
	if input.PaymentFinality != transactions.PaymentFinalityFinalized ||
		strings.TrimSpace(input.Path) == "" {
		return SignatureHeaders{}, ErrExecutionCapabilityUnavailable
	}
	keyID, err := signer.keySigner.CurrentKeyID(ctx)
	if err != nil || strings.TrimSpace(keyID) == "" {
		return SignatureHeaders{}, ErrExecutionCapabilityUnavailable
	}
	identifierBytes := make([]byte, 16)
	if _, err := io.ReadFull(signer.randomness, identifierBytes); err != nil {
		return SignatureHeaders{}, ErrExecutionCapabilityUnavailable
	}
	now := signer.clock.Now().UTC()
	bodyDigest := sha256.Sum256(input.Body)
	claims := executionCapabilityClaims{
		Issuer:          signer.config.Issuer,
		Audience:        "urn:agentpay:seller:" + input.SellerID.String(),
		Subject:         input.TransactionID.String(),
		SellerID:        input.SellerID.String(),
		RouteID:         input.RouteID.String(),
		TransactionID:   input.TransactionID.String(),
		Method:          strings.ToUpper(string(input.Method)),
		Path:            input.Path,
		BodySHA256:      hex.EncodeToString(bodyDigest[:]),
		PaymentFinality: transactions.PaymentFinalityFinalized,
		JWTID:           "xec_" + base64.RawURLEncoding.EncodeToString(identifierBytes),
		IssuedAt:        now.Unix(),
		ExpiresAt:       now.Add(signer.config.Lifetime).Unix(),
	}
	headerBytes, err := json.Marshal(executionCapabilityHeader{
		Type: ExecutionCapabilityType, Algorithm: executionCapabilityAlgorithm, KeyID: keyID,
	})
	if err != nil {
		return SignatureHeaders{}, err
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return SignatureHeaders{}, err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerBytes) + "." +
		base64.RawURLEncoding.EncodeToString(claimsBytes)
	signature, err := signer.keySigner.Sign(ctx, keyID, []byte(signingInput))
	if err != nil || len(signature) != 64 {
		return SignatureHeaders{}, ErrExecutionCapabilityUnavailable
	}
	token := signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
	if len(token) > maximumExecutionTokenBytes {
		return SignatureHeaders{}, ErrExecutionCapabilityUnavailable
	}
	return SignatureHeaders{
		ExecutionCapability: token,
		Transaction:         input.TransactionID.String(),
	}, nil
}

var _ RequestSigner = (*ES256ExecutionCapabilitySigner)(nil)
