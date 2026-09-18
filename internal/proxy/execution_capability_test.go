package proxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

func TestExecutionCapabilitySignerBindsFinalizedSellerRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)
	clock := domain.FixedClock{Value: now}
	keys, err := authorization.NewLocalES256KeyRing(clock)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewES256ExecutionCapabilitySigner(
		ExecutionCapabilityConfig{
			Issuer:   "https://api.agentpay.test",
			Lifetime: 45 * time.Second,
		},
		keys,
		clock,
		strings.NewReader(strings.Repeat("j", 64)),
	)
	if err != nil {
		t.Fatal(err)
	}
	request := validExecutionRequest(t, verifiedTransaction(t))
	headers, err := signer.Sign(t.Context(), "", SigningInput{
		TransactionID:   request.Transaction.TransactionID(),
		SellerID:        request.Seller.SellerID,
		RouteID:         request.Route.RouteID,
		Method:          request.Method,
		Path:            request.Path,
		Body:            request.Body,
		PaymentFinality: request.Transaction.PaymentFinality(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if headers.ExecutionCapability == "" || headers.Transaction != request.Transaction.TransactionID().String() {
		t.Fatalf("headers = %#v", headers)
	}

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(
		headers.ExecutionCapability,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != "ES256" || token.Header["typ"] != ExecutionCapabilityType {
				return nil, errors.New("unexpected protected header")
			}
			keyID, _ := token.Header["kid"].(string)
			return keys.VerificationKey(context.Background(), keyID)
		},
		jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithIssuer("https://api.agentpay.test"),
		jwt.WithAudience("urn:agentpay:seller:"+request.Seller.SellerID.String()),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil || !parsed.Valid {
		t.Fatalf("ParseWithClaims() = (%v, %v)", parsed, err)
	}
	bodyDigest := sha256.Sum256(request.Body)
	want := map[string]any{
		"sub":             request.Transaction.TransactionID().String(),
		"sellerId":        request.Seller.SellerID.String(),
		"routeId":         request.Route.RouteID.String(),
		"transactionId":   request.Transaction.TransactionID().String(),
		"method":          string(request.Method),
		"path":            request.Path,
		"bodySha256":      hex.EncodeToString(bodyDigest[:]),
		"paymentFinality": "finalized",
	}
	for name, value := range want {
		if claims[name] != value {
			t.Fatalf("claim %s = %#v, want %#v", name, claims[name], value)
		}
	}
	issuedAt, _ := claims["iat"].(float64)
	expiresAt, _ := claims["exp"].(float64)
	if expiresAt-issuedAt != 45 || !strings.HasPrefix(claims["jti"].(string), "xec_") {
		t.Fatalf("temporal claims = %#v", claims)
	}
}

func TestExecutionCapabilitySignerRejectsUnsafeLifetime(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)}
	keys, err := authorization.NewLocalES256KeyRing(clock)
	if err != nil {
		t.Fatal(err)
	}
	for _, lifetime := range []time.Duration{29 * time.Second, 61 * time.Second} {
		if _, err := NewES256ExecutionCapabilitySigner(
			ExecutionCapabilityConfig{Issuer: "https://api.agentpay.test", Lifetime: lifetime},
			keys,
			clock,
			strings.NewReader(strings.Repeat("j", 64)),
		); err == nil {
			t.Fatalf("lifetime %s was accepted", lifetime)
		}
	}
}

var _ interface {
	VerificationKey(context.Context, string) (*ecdsa.PublicKey, error)
} = (*authorization.LocalES256KeyRing)(nil)
