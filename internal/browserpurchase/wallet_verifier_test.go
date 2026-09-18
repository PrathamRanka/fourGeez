package browserpurchase

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestEVMPersonalSignVerifierRecoversExpectedWallet(t *testing.T) {
	t.Parallel()
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	message := "AgentPay browser purchase recovery test"
	signature, err := crypto.Sign(accounts.TextHash([]byte(message)), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	signature[crypto.RecoveryIDOffset] += 27

	verified, err := NewEVMPersonalSignVerifier().Verify(t.Context(), "eip155:84532", address, message, hexutil.Encode(signature))
	if err != nil || !verified {
		t.Fatalf("Verify() = %v, %v", verified, err)
	}
}

func TestEVMPersonalSignVerifierRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		network string
		address string
		proof   string
	}{
		{name: "unsupported network", network: "solana:devnet", address: "wallet", proof: "proof"},
		{name: "invalid address", network: "eip155:84532", address: "0x123", proof: "0x00"},
		{name: "invalid proof", network: "eip155:84532", address: "0x1111111111111111111111111111111111111111", proof: "0x00"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			verified, err := NewEVMPersonalSignVerifier().Verify(t.Context(), test.network, test.address, "message", test.proof)
			if err != nil || verified {
				t.Fatalf("Verify() = %v, %v", verified, err)
			}
		})
	}
}
