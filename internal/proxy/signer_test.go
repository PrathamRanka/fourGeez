package proxy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
)

// TestHMACSignerCreatesVerifiableSellerHeaders verifies canonical signatures.
func TestHMACSignerCreatesVerifiableSellerHeaders(t *testing.T) {
	t.Parallel()

	secret := []byte(strings.Repeat("s", 32))
	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	signer := NewHMACSigner(
		&staticSecretProvider{
			secrets: map[string][]byte{
				"secret/seller": secret,
			},
		},
		clock,
	)
	input := validSigningInput(t)
	first, err := signer.Sign(t.Context(), "secret/seller", input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := signer.Sign(t.Context(), "secret/seller", input)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.Signature == "" {
		t.Fatalf("signatures = %#v and %#v", first, second)
	}
	if !VerifyHMACSignature(secret, input, first) {
		t.Fatal("VerifyHMACSignature() rejected valid headers")
	}

	tampered := input
	tampered.Body = []byte(`{"city":"Seattle"}`)
	if VerifyHMACSignature(secret, tampered, first) {
		t.Fatal("VerifyHMACSignature() accepted a modified body")
	}
}

// TestHMACSignerRejectsMissingOrWeakSecrets verifies fail-closed secret loading.
func TestHMACSignerRejectsMissingOrWeakSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider SecretProvider
	}{
		{
			name:     "missing",
			provider: &staticSecretProvider{err: errors.New("not found")},
		},
		{
			name: "weak",
			provider: &staticSecretProvider{
				secrets: map[string][]byte{"secret/seller": []byte("short")},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			signer := NewHMACSigner(
				test.provider,
				domain.FixedClock{Value: time.Now()},
			)
			if _, err := signer.Sign(
				t.Context(),
				"secret/seller",
				validSigningInput(t),
			); !errors.Is(err, ErrSigningUnavailable) {
				t.Fatalf("Sign() error = %v", err)
			}
		})
	}
}

// validSigningInput returns one complete seller signature input.
func validSigningInput(t *testing.T) SigningInput {
	t.Helper()

	transactionID, err := domain.ParseID(
		"txn_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.TransactionIDPrefix,
	)
	if err != nil {
		t.Fatal(err)
	}
	return SigningInput{
		TransactionID: transactionID,
		Method:        catalog.RouteMethodPost,
		Path:          "/research",
		Body:          []byte(`{"topic":"payments"}`),
	}
}

type staticSecretProvider struct {
	secrets map[string][]byte
	err     error
}

// GetSecret returns a copied test secret for its reference.
func (provider *staticSecretProvider) GetSecret(
	_ context.Context,
	reference string,
) ([]byte, error) {
	if provider.err != nil {
		return nil, provider.err
	}
	secret, exists := provider.secrets[reference]
	if !exists {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), secret...), nil
}
