package settlement

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

// TestEVMPersonalSignOwnershipVerifierRecoversExpectedSigner verifies EIP-191 proofs.
func TestEVMPersonalSignOwnershipVerifierRecoversExpectedSigner(t *testing.T) {
	t.Parallel()

	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	challenge := "AgentPay ownership test"
	signature, err := crypto.Sign(accounts.TextHash([]byte(challenge)), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	signature[crypto.RecoveryIDOffset] += 27
	destination := PaymentDestination{
		Network: "eip155:84532",
		Address: crypto.PubkeyToAddress(privateKey.PublicKey).Hex(),
	}

	valid, err := NewEVMPersonalSignOwnershipVerifier().VerifyOwnership(
		context.Background(),
		destination,
		challenge,
		"0x"+hex.EncodeToString(signature),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("valid personal-sign proof was rejected")
	}
}

// TestEVMPersonalSignOwnershipVerifierRejectsMalformedProof verifies safe failures.
func TestEVMPersonalSignOwnershipVerifierRejectsMalformedProof(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		network   string
		address   string
		signature string
	}{
		{
			name:      "unsupported network",
			network:   "solana:devnet",
			address:   "11111111111111111111111111111111",
			signature: "0x00",
		},
		{
			name:      "invalid address",
			network:   "eip155:84532",
			address:   "not-an-address",
			signature: "0x00",
		},
		{
			name:      "invalid signature length",
			network:   "eip155:84532",
			address:   "0x1111111111111111111111111111111111111111",
			signature: "0x00",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewEVMPersonalSignOwnershipVerifier().VerifyOwnership(
				context.Background(),
				PaymentDestination{Network: test.network, Address: test.address},
				"challenge",
				test.signature,
			)
			if err == nil {
				t.Fatal("VerifyOwnership() error = nil")
			}
		})
	}
}
