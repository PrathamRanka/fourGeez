package browserpurchase

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// EVMPersonalSignVerifier verifies EIP-191 personal_sign recovery proofs.
type EVMPersonalSignVerifier struct{}

func NewEVMPersonalSignVerifier() *EVMPersonalSignVerifier {
	return &EVMPersonalSignVerifier{}
}

func (*EVMPersonalSignVerifier) Verify(_ context.Context, network, address, message, proof string) (bool, error) {
	if !strings.HasPrefix(network, "eip155:") || !common.IsHexAddress(address) {
		return false, nil
	}
	signature, err := hexutil.Decode(proof)
	if err != nil || len(signature) != crypto.SignatureLength {
		return false, nil
	}
	normalizedSignature := append([]byte(nil), signature...)
	if normalizedSignature[crypto.RecoveryIDOffset] >= 27 {
		normalizedSignature[crypto.RecoveryIDOffset] -= 27
	}
	publicKey, err := crypto.SigToPub(accounts.TextHash([]byte(message)), normalizedSignature)
	if err != nil {
		return false, nil
	}
	return crypto.PubkeyToAddress(*publicKey) == common.HexToAddress(address), nil
}

var _ WalletProofVerifier = (*EVMPersonalSignVerifier)(nil)
