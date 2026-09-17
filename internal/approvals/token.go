package approvals

import (
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/intents"
)

const (
	approvalTokenVersion       = "1"
	minimumApprovalSecretBytes = 32
	invitationTokenBytes       = 32
)

var (
	ErrApprovalTokenInvalid = errors.New("approval token is invalid")
	ErrApprovalTokenExpired = errors.New("approval token has expired")
)

// TokenGenerator creates unpredictable invitation tokens.
type TokenGenerator interface {
	NewToken() (string, error)
}

// SecureTokenGenerator creates URL-safe random tokens.
type SecureTokenGenerator struct {
	reader io.Reader
}

// NewSecureTokenGenerator creates a token generator backed by crypto/rand when
// reader is nil.
func NewSecureTokenGenerator(reader io.Reader) *SecureTokenGenerator {
	if reader == nil {
		reader = cryptorand.Reader
	}
	return &SecureTokenGenerator{reader: reader}
}

// NewToken creates a 256-bit URL-safe invitation token.
func (generator *SecureTokenGenerator) NewToken() (string, error) {
	randomBytes := make([]byte, invitationTokenBytes)
	if _, err := io.ReadFull(generator.reader, randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

// ApprovalTokenClaims are the signed values bound to an approved session.
type ApprovalTokenClaims struct {
	Version    string               `json:"version"`
	SessionID  domain.ID            `json:"sessionId"`
	IntentID   domain.ID            `json:"intentId"`
	IntentHash intents.SHA256Digest `json:"intentHash"`
	ExpiresAt  domain.Timestamp     `json:"expiresAt"`
}

// ApprovalTokenSigner issues and verifies internal approval tokens.
type ApprovalTokenSigner struct {
	secret []byte
}

// NewApprovalTokenSigner copies and validates an HMAC secret.
func NewApprovalTokenSigner(secret []byte) (*ApprovalTokenSigner, error) {
	if len(secret) < minimumApprovalSecretBytes {
		return nil, domain.NewValidationError("approvalTokenSecret", "length", "must contain at least 32 bytes")
	}
	secretCopy := append([]byte(nil), secret...)
	return &ApprovalTokenSigner{secret: secretCopy}, nil
}

// Issue returns the raw token once and its safe storage hash.
func (signer *ApprovalTokenSigner) Issue(sessionID, intentID domain.ID, intentHash intents.SHA256Digest, expiresAt domain.Timestamp) (string, string, error) {
	claims := ApprovalTokenClaims{
		Version:    approvalTokenVersion,
		SessionID:  sessionID,
		IntentID:   intentID,
		IntentHash: intentHash,
		ExpiresAt:  expiresAt,
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", "", err
	}
	payloadSegment := base64.RawURLEncoding.EncodeToString(payload)
	signatureSegment := base64.RawURLEncoding.EncodeToString(signer.sign([]byte(payloadSegment)))
	token := payloadSegment + "." + signatureSegment
	return token, hashToken(token), nil
}

// Verify validates signature, binding, version, and expiration.
func (signer *ApprovalTokenSigner) Verify(token string, expectedSessionID, expectedIntentID domain.ID, expectedIntentHash intents.SHA256Digest, now domain.Timestamp) (ApprovalTokenClaims, error) {
	segments := strings.Split(token, ".")
	if len(segments) != 2 {
		return ApprovalTokenClaims{}, ErrApprovalTokenInvalid
	}

	suppliedSignature, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil || !hmac.Equal(suppliedSignature, signer.sign([]byte(segments[0]))) {
		return ApprovalTokenClaims{}, ErrApprovalTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(segments[0])
	if err != nil {
		return ApprovalTokenClaims{}, ErrApprovalTokenInvalid
	}

	var claims ApprovalTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ApprovalTokenClaims{}, ErrApprovalTokenInvalid
	}
	if claims.Version != approvalTokenVersion ||
		claims.SessionID != expectedSessionID ||
		claims.IntentID != expectedIntentID ||
		claims.IntentHash != expectedIntentHash {
		return ApprovalTokenClaims{}, ErrApprovalTokenInvalid
	}
	if !now.Time().Before(claims.ExpiresAt.Time()) {
		return ApprovalTokenClaims{}, ErrApprovalTokenExpired
	}
	return claims, nil
}

func (signer *ApprovalTokenSigner) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, signer.secret)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

func hashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
