package identity

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

func TestCognitoVerifierValidatesAccessTokenContract(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	server := newJWKSServer(t, "key-1", &privateKey.PublicKey)
	t.Cleanup(server.Close)

	verifier, err := NewCognitoVerifier(CognitoConfig{
		Issuer: server.URL, ClientID: "seller-client",
		HTTPClient: &http.Client{Timeout: time.Second},
		Clock:      domain.FixedClock{Value: now},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		claims    jwt.MapClaims
		algorithm jwt.SigningMethod
		wantErr   bool
	}{
		{name: "valid access token", claims: validCognitoClaims(server.URL, now), algorithm: jwt.SigningMethodRS256},
		{name: "wrong client", claims: mergeClaims(validCognitoClaims(server.URL, now), jwt.MapClaims{"client_id": "other-client"}), algorithm: jwt.SigningMethodRS256, wantErr: true},
		{name: "ID token rejected", claims: mergeClaims(validCognitoClaims(server.URL, now), jwt.MapClaims{"token_use": "id"}), algorithm: jwt.SigningMethodRS256, wantErr: true},
		{name: "expired", claims: mergeClaims(validCognitoClaims(server.URL, now), jwt.MapClaims{"exp": now.Add(-time.Minute).Unix()}), algorithm: jwt.SigningMethodRS256, wantErr: true},
		{name: "algorithm confusion rejected", claims: validCognitoClaims(server.URL, now), algorithm: jwt.SigningMethodHS256, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := jwt.NewWithClaims(test.algorithm, test.claims)
			token.Header["kid"] = "key-1"
			var signed string
			var signingErr error
			if test.algorithm == jwt.SigningMethodRS256 {
				signed, signingErr = token.SignedString(privateKey)
			} else {
				signed, signingErr = token.SignedString([]byte("not-an-rsa-key"))
			}
			if signingErr != nil {
				t.Fatal(signingErr)
			}

			claims, verifyErr := verifier.Verify(t.Context(), signed)
			if (verifyErr != nil) != test.wantErr {
				t.Fatalf("Verify() error = %v, want error %t", verifyErr, test.wantErr)
			}
			if !test.wantErr && (claims.Subject != "seller-subject" || claims.SessionID != "origin-jti") {
				t.Fatalf("claims = %#v", claims)
			}
		})
	}
}

func validCognitoClaims(issuer string, now time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"iss": issuer, "sub": "seller-subject", "client_id": "seller-client", "token_use": "access",
		"jti": "token-jti", "origin_jti": "origin-jti", "iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(time.Hour).Unix(), "email": "seller@example.com", "username": "seller@example.com",
	}
}

func mergeClaims(base jwt.MapClaims, override jwt.MapClaims) jwt.MapClaims {
	merged := make(jwt.MapClaims, len(base)+len(override))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func newJWKSServer(t *testing.T, keyID string, publicKey *rsa.PublicKey) *httptest.Server {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"keys": []map[string]string{{
			"kid": keyID, "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/.well-known/jwks.json" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write(payload)
	}))
}
