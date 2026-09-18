package agentpayverify

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestExecutionVerifierValidatesBindingsAndConsumesJTIOnce(t *testing.T) {
	t.Parallel()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)
	resolver := &staticExecutionKeyResolver{key: &privateKey.PublicKey}
	verifier, err := NewExecutionVerifier(ExecutionConfig{
		Issuer: "https://api.agentpay.test", SellerID: "sel_test", RouteID: "rte_test",
		ReplayStore: NewMemoryExecutionReplayStore(), KeyResolver: resolver,
		Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"topic":"payments"}`)
	token := signedExecutionToken(t, privateKey, now, body, nil)
	claims, err := verifier.Verify(
		t.Context(), "POST", "/research", body, token, "txn_test",
	)
	if err != nil {
		t.Fatal(err)
	}
	if claims.TransactionID != "txn_test" || claims.JWTID != "xec_test" {
		t.Fatalf("claims = %#v", claims)
	}
	if _, err := verifier.Verify(t.Context(), "POST", "/research", body, token, "txn_test"); !errors.Is(err, ErrReplay) {
		t.Fatalf("replay error = %v", err)
	}
}

func TestExecutionVerifierRejectsEveryMismatchedBinding(t *testing.T) {
	t.Parallel()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"topic":"payments"}`)
	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
		txn    string
		mutate func(jwt.MapClaims)
	}{
		{name: "issuer", method: "POST", path: "/research", body: body, txn: "txn_test", mutate: func(claims jwt.MapClaims) { claims["iss"] = "https://evil.example" }},
		{name: "audience", method: "POST", path: "/research", body: body, txn: "txn_test", mutate: func(claims jwt.MapClaims) { claims["aud"] = "urn:agentpay:seller:sel_other" }},
		{name: "seller", method: "POST", path: "/research", body: body, txn: "txn_test", mutate: func(claims jwt.MapClaims) { claims["sellerId"] = "sel_other" }},
		{name: "route", method: "POST", path: "/research", body: body, txn: "txn_test", mutate: func(claims jwt.MapClaims) { claims["routeId"] = "rte_other" }},
		{name: "method", method: "GET", path: "/research", body: body, txn: "txn_test"},
		{name: "path", method: "POST", path: "/other", body: body, txn: "txn_test"},
		{name: "body", method: "POST", path: "/research", body: []byte(`{"topic":"changed"}`), txn: "txn_test"},
		{name: "transaction header", method: "POST", path: "/research", body: body, txn: "txn_other"},
		{name: "finality", method: "POST", path: "/research", body: body, txn: "txn_test", mutate: func(claims jwt.MapClaims) { claims["paymentFinality"] = "confirmed" }},
		{name: "lifetime", method: "POST", path: "/research", body: body, txn: "txn_test", mutate: func(claims jwt.MapClaims) { claims["exp"] = now.Add(61 * time.Second).Unix() }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier, createErr := NewExecutionVerifier(ExecutionConfig{
				Issuer: "https://api.agentpay.test", SellerID: "sel_test", RouteID: "rte_test",
				ReplayStore: NewMemoryExecutionReplayStore(),
				KeyResolver: &staticExecutionKeyResolver{key: &privateKey.PublicKey},
				Clock:       func() time.Time { return now },
			})
			if createErr != nil {
				t.Fatal(createErr)
			}
			token := signedExecutionToken(t, privateKey, now, body, test.mutate)
			if _, verifyErr := verifier.Verify(t.Context(), test.method, test.path, test.body, token, test.txn); verifyErr == nil {
				t.Fatal("Verify() accepted mismatched execution capability")
			}
		})
	}
}

func signedExecutionToken(
	t *testing.T,
	privateKey *ecdsa.PrivateKey,
	now time.Time,
	body []byte,
	mutate func(jwt.MapClaims),
) string {
	t.Helper()
	digest := sha256.Sum256(body)
	claims := jwt.MapClaims{
		"iss": "https://api.agentpay.test", "aud": "urn:agentpay:seller:sel_test",
		"sub": "txn_test", "sellerId": "sel_test", "routeId": "rte_test",
		"transactionId": "txn_test", "method": "POST", "path": "/research",
		"bodySha256": hex.EncodeToString(digest[:]), "paymentFinality": "finalized",
		"jti": "xec_test", "iat": now.Unix(), "exp": now.Add(45 * time.Second).Unix(),
	}
	if mutate != nil {
		mutate(claims)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["typ"] = ExecutionCapabilityType
	token.Header["kid"] = "key-1"
	raw, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type staticExecutionKeyResolver struct {
	key *ecdsa.PublicKey
	err error
}

func (resolver *staticExecutionKeyResolver) ResolveExecutionKey(context.Context, string) (*ecdsa.PublicKey, error) {
	return resolver.key, resolver.err
}
