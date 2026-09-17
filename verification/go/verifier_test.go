package agentpayverify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestMiddlewareVerifiesSignatureFreshnessAndReplay exercises the HTTP boundary.
func TestMiddlewareVerifiesSignatureFreshnessAndReplay(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)
	secret := []byte("0123456789abcdef0123456789abcdef")
	verifier, err := NewVerifier(Config{
		Secret:      secret,
		MaximumAge:  5 * time.Minute,
		ReplayStore: NewMemoryReplayStore(),
		Clock:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := verifier.Middleware(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	testCases := []struct {
		name          string
		body          string
		signedBody    string
		timestamp     time.Time
		transactionID string
		wantStatus    int
	}{
		{name: "valid", body: "hello", signedBody: "hello", timestamp: now, transactionID: "txn_valid", wantStatus: http.StatusNoContent},
		{name: "fractional timestamp", body: "hello", signedBody: "hello", timestamp: now.Add(123456 * time.Microsecond), transactionID: "txn_fractional", wantStatus: http.StatusNoContent},
		{name: "modified body", body: "changed", signedBody: "hello", timestamp: now, transactionID: "txn_changed", wantStatus: http.StatusUnauthorized},
		{name: "stale", body: "hello", signedBody: "hello", timestamp: now.Add(-6 * time.Minute), transactionID: "txn_stale", wantStatus: http.StatusUnauthorized},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := signedRequest(t, secret, testCase.timestamp, testCase.transactionID, testCase.body, testCase.signedBody)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, testCase.wantStatus)
			}
			if response.Code != http.StatusNoContent &&
				response.Header().Get("Content-Type") != "application/json" {
				t.Fatalf(
					"content type = %q, want application/json",
					response.Header().Get("Content-Type"),
				)
			}
		})
	}

	replay := signedRequest(t, secret, now, "txn_valid", "hello", "hello")
	replayResponse := httptest.NewRecorder()
	handler.ServeHTTP(replayResponse, replay)
	if replayResponse.Code != http.StatusConflict {
		t.Fatalf("replay status = %d, want %d", replayResponse.Code, http.StatusConflict)
	}
	if replayResponse.Header().Get("Content-Type") != "application/json" {
		t.Fatalf(
			"replay content type = %q, want application/json",
			replayResponse.Header().Get("Content-Type"),
		)
	}
}

// TestNewVerifierRejectsShortSecret verifies minimum key strength.
func TestNewVerifierRejectsShortSecret(t *testing.T) {
	t.Parallel()

	if _, err := NewVerifier(Config{Secret: []byte("short")}); err == nil {
		t.Fatal("NewVerifier() accepted a short secret")
	}
}

// signedRequest creates one independently signed seller request fixture.
func signedRequest(
	t *testing.T,
	secret []byte,
	timestamp time.Time,
	transactionID string,
	body string,
	signedBody string,
) *http.Request {
	t.Helper()

	request := httptest.NewRequest(
		http.MethodPost,
		"https://seller.example/fulfill?ignored=true",
		bytes.NewBufferString(body),
	)
	bodyDigest := sha256.Sum256([]byte(signedBody))
	canonical := "agentpay.seller-request.v1\n" +
		timestamp.UTC().Format(time.RFC3339Nano) + "\n" +
		http.MethodPost + "\n" +
		"/fulfill\n" +
		hex.EncodeToString(bodyDigest[:]) + "\n" +
		transactionID
	signature := hmac.New(sha256.New, secret)
	_, _ = signature.Write([]byte(canonical))
	request.Header.Set(SignatureHeader, base64.StdEncoding.EncodeToString(signature.Sum(nil)))
	request.Header.Set(TimestampHeader, timestamp.UTC().Format(time.RFC3339Nano))
	request.Header.Set(TransactionIDHeader, transactionID)
	return request.WithContext(context.Background())
}
