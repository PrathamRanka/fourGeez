package settlement

import (
	"context"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

var ErrOwnershipNetworkUnsupported = errors.New("ownership verification network is unsupported")

// EVMPersonalSignOwnershipVerifier validates EIP-191 personal-sign proofs for EVM destinations.
type EVMPersonalSignOwnershipVerifier struct{}

// NewEVMPersonalSignOwnershipVerifier creates the EVM proof adapter.
func NewEVMPersonalSignOwnershipVerifier() *EVMPersonalSignOwnershipVerifier {
	return &EVMPersonalSignOwnershipVerifier{}
}

// VerifyOwnership recovers the EOA signer from an EIP-191 personal-sign proof.
func (*EVMPersonalSignOwnershipVerifier) VerifyOwnership(
	_ context.Context,
	destination PaymentDestination,
	challenge string,
	signature string,
) (bool, error) {
	if !strings.HasPrefix(destination.Network, "eip155:") {
		return false, ErrOwnershipNetworkUnsupported
	}
	if !common.IsHexAddress(destination.Address) {
		return false, ErrOwnershipProofInvalid
	}
	signatureBytes, err := hexutil.Decode(signature)
	if err != nil {
		return false, ErrOwnershipProofInvalid
	}
	if len(signatureBytes) != crypto.SignatureLength {
		return false, ErrOwnershipProofInvalid
	}
	normalizedSignature := append([]byte(nil), signatureBytes...)
	if normalizedSignature[crypto.RecoveryIDOffset] >= 27 {
		normalizedSignature[crypto.RecoveryIDOffset] -= 27
	}
	publicKey, err := crypto.SigToPub(
		accounts.TextHash([]byte(challenge)),
		normalizedSignature,
	)
	if err != nil {
		return false, ErrOwnershipProofInvalid
	}
	recoveredAddress := crypto.PubkeyToAddress(*publicKey)
	return recoveredAddress == common.HexToAddress(destination.Address), nil
}
