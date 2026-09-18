package agentpayverify

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	ExecutionCapabilityHeader = "X-AgentPay-Execution-Capability"
	ExecutionCapabilityType   = "agentpay-execution+jwt"
	minimumExecutionLifetime  = 30 * time.Second
	maximumExecutionLifetime  = 60 * time.Second
	maximumExecutionTokenSize = 16 * 1024
)

var (
	ErrInvalidExecutionCapability = errors.New("invalid AgentPay execution capability")
	ErrExecutionBinding           = errors.New("AgentPay execution capability binding mismatch")
	ErrExecutionKeyUnavailable    = errors.New("AgentPay execution verification key unavailable")
)

type ExecutionKeyResolver interface {
	ResolveExecutionKey(context.Context, string) (*ecdsa.PublicKey, error)
}

type ExecutionReplayStore interface {
	ClaimExecutionJTI(context.Context, string, time.Time) (bool, error)
}

type ExecutionConfig struct {
	Issuer      string
	SellerID    string
	RouteID     string
	KeyResolver ExecutionKeyResolver
	ReplayStore ExecutionReplayStore
	Clock       func() time.Time
}

type ExecutionClaims struct {
	Issuer          string
	Audience        string
	Subject         string
	SellerID        string
	RouteID         string
	TransactionID   string
	Method          string
	Path            string
	BodySHA256      string
	PaymentFinality string
	JWTID           string
	IssuedAt        time.Time
	ExpiresAt       time.Time
}

type ExecutionVerifier struct {
	config ExecutionConfig
}

func NewExecutionVerifier(config ExecutionConfig) (*ExecutionVerifier, error) {
	config.Issuer = strings.TrimRight(strings.TrimSpace(config.Issuer), "/")
	config.SellerID = strings.TrimSpace(config.SellerID)
	config.RouteID = strings.TrimSpace(config.RouteID)
	if config.Issuer == "" || config.SellerID == "" || config.RouteID == "" ||
		config.KeyResolver == nil || config.ReplayStore == nil {
		return nil, ErrInvalidExecutionCapability
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	return &ExecutionVerifier{config: config}, nil
}

func (verifier *ExecutionVerifier) Verify(
	ctx context.Context,
	method string,
	path string,
	body []byte,
	rawToken string,
	transactionHeader string,
) (ExecutionClaims, error) {
	if rawToken == "" || len(rawToken) > maximumExecutionTokenSize || strings.TrimSpace(transactionHeader) == "" {
		return ExecutionClaims{}, ErrInvalidExecutionCapability
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != "ES256" || token.Header["typ"] != ExecutionCapabilityType {
				return nil, ErrInvalidExecutionCapability
			}
			keyID, ok := token.Header["kid"].(string)
			if !ok || strings.TrimSpace(keyID) == "" {
				return nil, ErrInvalidExecutionCapability
			}
			key, resolveErr := verifier.config.KeyResolver.ResolveExecutionKey(ctx, keyID)
			if resolveErr != nil || key == nil {
				return nil, ErrExecutionKeyUnavailable
			}
			return key, nil
		},
		jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithIssuer(verifier.config.Issuer),
		jwt.WithAudience("urn:agentpay:seller:"+verifier.config.SellerID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(func() time.Time { return verifier.config.Clock().UTC() }),
	)
	if err != nil || token == nil || !token.Valid {
		if errors.Is(err, ErrExecutionKeyUnavailable) {
			return ExecutionClaims{}, ErrExecutionKeyUnavailable
		}
		return ExecutionClaims{}, ErrInvalidExecutionCapability
	}
	parsed, err := parseExecutionClaims(claims)
	if err != nil {
		return ExecutionClaims{}, err
	}
	if parsed.ExpiresAt.Sub(parsed.IssuedAt) < minimumExecutionLifetime ||
		parsed.ExpiresAt.Sub(parsed.IssuedAt) > maximumExecutionLifetime {
		return ExecutionClaims{}, ErrInvalidExecutionCapability
	}
	bodyDigest := sha256.Sum256(body)
	if parsed.Subject != transactionHeader || parsed.TransactionID != transactionHeader ||
		parsed.SellerID != verifier.config.SellerID || parsed.RouteID != verifier.config.RouteID ||
		parsed.Method != strings.ToUpper(method) || parsed.Path != path ||
		parsed.BodySHA256 != hex.EncodeToString(bodyDigest[:]) ||
		parsed.PaymentFinality != "finalized" {
		return ExecutionClaims{}, ErrExecutionBinding
	}
	claimed, err := verifier.config.ReplayStore.ClaimExecutionJTI(ctx, parsed.JWTID, parsed.ExpiresAt)
	if err != nil {
		return ExecutionClaims{}, err
	}
	if !claimed {
		return ExecutionClaims{}, ErrReplay
	}
	return parsed, nil
}

func parseExecutionClaims(claims jwt.MapClaims) (ExecutionClaims, error) {
	issuedAt, err := claims.GetIssuedAt()
	if err != nil || issuedAt == nil {
		return ExecutionClaims{}, ErrInvalidExecutionCapability
	}
	expiresAt, err := claims.GetExpirationTime()
	if err != nil || expiresAt == nil {
		return ExecutionClaims{}, ErrInvalidExecutionCapability
	}
	issuer, _ := claims["iss"].(string)
	audience, _ := claims["aud"].(string)
	subject, _ := claims["sub"].(string)
	parsed := ExecutionClaims{
		Issuer: issuer, Audience: audience, Subject: subject,
		SellerID: stringClaim(claims, "sellerId"), RouteID: stringClaim(claims, "routeId"),
		TransactionID: stringClaim(claims, "transactionId"), Method: stringClaim(claims, "method"),
		Path: stringClaim(claims, "path"), BodySHA256: stringClaim(claims, "bodySha256"),
		PaymentFinality: stringClaim(claims, "paymentFinality"), JWTID: stringClaim(claims, "jti"),
		IssuedAt: issuedAt.Time.UTC(), ExpiresAt: expiresAt.Time.UTC(),
	}
	if parsed.Issuer == "" || parsed.Audience == "" || parsed.Subject == "" ||
		parsed.SellerID == "" || parsed.RouteID == "" || parsed.TransactionID == "" ||
		parsed.Method == "" || parsed.Path == "" || len(parsed.BodySHA256) != 64 ||
		!strings.HasPrefix(parsed.JWTID, "xec_") {
		return ExecutionClaims{}, ErrInvalidExecutionCapability
	}
	return parsed, nil
}

func stringClaim(claims jwt.MapClaims, name string) string {
	value, _ := claims[name].(string)
	return value
}

type MemoryExecutionReplayStore struct {
	mutex sync.Mutex
	jtis  map[string]time.Time
}

func NewMemoryExecutionReplayStore() *MemoryExecutionReplayStore {
	return &MemoryExecutionReplayStore{jtis: make(map[string]time.Time)}
}

func (store *MemoryExecutionReplayStore) ClaimExecutionJTI(
	_ context.Context,
	jti string,
	expiresAt time.Time,
) (bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if _, exists := store.jtis[jti]; exists {
		return false, nil
	}
	store.jtis[jti] = expiresAt
	return true, nil
}
