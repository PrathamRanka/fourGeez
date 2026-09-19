package awskms

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"math/big"
	"sort"
	"strings"
	"sync"

	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/evidence"
)

type Client interface {
	Sign(context.Context, *awskms.SignInput, ...func(*awskms.Options)) (*awskms.SignOutput, error)
	Verify(context.Context, *awskms.VerifyInput, ...func(*awskms.Options)) (*awskms.VerifyOutput, error)
	GetPublicKey(context.Context, *awskms.GetPublicKeyInput, ...func(*awskms.Options)) (*awskms.GetPublicKeyOutput, error)
}

type CapabilityKeyRing struct {
	client       Client
	currentKeyID string
	keyIDs       map[string]string
	mutex        sync.RWMutex
	publicKeys   map[string]*ecdsa.PublicKey
}

func NewCapabilityKeyRing(client Client, currentKMSKeyID string, keyIDs map[string]string) (*CapabilityKeyRing, error) {
	currentKMSKeyID = strings.TrimSpace(currentKMSKeyID)
	if client == nil || currentKMSKeyID == "" || len(keyIDs) == 0 {
		return nil, authorization.ErrAuthorizationUnavailable
	}
	logicalCurrent := ""
	keys := make(map[string]string, len(keyIDs))
	for logicalID, kmsKeyID := range keyIDs {
		logicalID = strings.TrimSpace(logicalID)
		kmsKeyID = strings.TrimSpace(kmsKeyID)
		if logicalID == "" || kmsKeyID == "" {
			return nil, authorization.ErrAuthorizationUnavailable
		}
		keys[logicalID] = kmsKeyID
		if kmsKeyID == currentKMSKeyID {
			logicalCurrent = logicalID
		}
	}
	if logicalCurrent == "" {
		return nil, authorization.ErrAuthorizationUnavailable
	}
	return &CapabilityKeyRing{client: client, currentKeyID: logicalCurrent, keyIDs: keys, publicKeys: make(map[string]*ecdsa.PublicKey)}, nil
}

func (ring *CapabilityKeyRing) CurrentKeyID(context.Context) (string, error) {
	return ring.currentKeyID, nil
}

func (ring *CapabilityKeyRing) Sign(ctx context.Context, keyID string, signingInput []byte) ([]byte, error) {
	if keyID != ring.currentKeyID {
		return nil, authorization.ErrAuthorizationUnavailable
	}
	kmsKeyID := ring.keyIDs[keyID]
	output, err := ring.client.Sign(ctx, &awskms.SignInput{
		KeyId: &kmsKeyID, Message: signingInput,
		MessageType: types.MessageTypeRaw, SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256,
	})
	if err != nil {
		return nil, authorization.ErrAuthorizationUnavailable
	}
	return derToRawES256(output.Signature)
}

func (ring *CapabilityKeyRing) VerificationKey(ctx context.Context, keyID string) (*ecdsa.PublicKey, error) {
	ring.mutex.RLock()
	publicKey := ring.publicKeys[keyID]
	ring.mutex.RUnlock()
	if publicKey != nil {
		return publicKey, nil
	}
	kmsKeyID, found := ring.keyIDs[keyID]
	if !found {
		return nil, authorization.ErrInvalidAccessToken
	}
	output, err := ring.client.GetPublicKey(ctx, &awskms.GetPublicKeyInput{KeyId: &kmsKeyID})
	if err != nil || output.KeySpec != types.KeySpecEccNistP256 || output.KeyUsage != types.KeyUsageTypeSignVerify {
		return nil, authorization.ErrAuthorizationUnavailable
	}
	parsed, err := x509.ParsePKIXPublicKey(output.PublicKey)
	if err != nil {
		return nil, authorization.ErrAuthorizationUnavailable
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve == nil || publicKey.Curve.Params().Name != "P-256" {
		return nil, authorization.ErrAuthorizationUnavailable
	}
	ring.mutex.Lock()
	ring.publicKeys[keyID] = publicKey
	ring.mutex.Unlock()
	return publicKey, nil
}

func (ring *CapabilityKeyRing) JWKS(ctx context.Context) (authorization.JSONWebKeySet, error) {
	keyIDs := make([]string, 0, len(ring.keyIDs))
	for keyID := range ring.keyIDs {
		keyIDs = append(keyIDs, keyID)
	}
	sort.Strings(keyIDs)
	keys := make([]authorization.JSONWebKey, 0, len(keyIDs))
	for _, keyID := range keyIDs {
		publicKey, err := ring.VerificationKey(ctx, keyID)
		if err != nil {
			return authorization.JSONWebKeySet{}, err
		}
		x := make([]byte, 32)
		y := make([]byte, 32)
		publicKey.X.FillBytes(x)
		publicKey.Y.FillBytes(y)
		keys = append(keys, authorization.JSONWebKey{
			KeyType: "EC", Use: "sig", KeyOps: []string{"verify"}, Algorithm: "ES256",
			KeyID: keyID, Curve: "P-256", X: base64.RawURLEncoding.EncodeToString(x), Y: base64.RawURLEncoding.EncodeToString(y),
		})
	}
	return authorization.JSONWebKeySet{Keys: keys}, nil
}

type EvidenceSigner struct {
	client Client
	keyID  string
}

func NewEvidenceSigner(client Client, keyID string) (*EvidenceSigner, error) {
	keyID = strings.TrimSpace(keyID)
	if client == nil || keyID == "" {
		return nil, errors.New("KMS evidence signer requires client and key ID")
	}
	return &EvidenceSigner{client: client, keyID: keyID}, nil
}

func (signer *EvidenceSigner) Sign(ctx context.Context, digest []byte) (evidence.Signature, error) {
	if len(digest) != sha256.Size {
		return evidence.Signature{}, errors.New("evidence digest must be SHA-256")
	}
	output, err := signer.client.Sign(ctx, &awskms.SignInput{
		KeyId: &signer.keyID, Message: digest,
		MessageType: types.MessageTypeDigest, SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256,
	})
	if err != nil {
		return evidence.Signature{}, err
	}
	return evidence.Signature{KeyID: signer.keyID, Value: base64.StdEncoding.EncodeToString(output.Signature)}, nil
}

func (signer *EvidenceSigner) Verify(ctx context.Context, keyID string, digest []byte, encodedSignature string) (bool, error) {
	if keyID != signer.keyID || len(digest) != sha256.Size {
		return false, nil
	}
	signature, err := base64.StdEncoding.DecodeString(encodedSignature)
	if err != nil {
		return false, nil
	}
	output, err := signer.client.Verify(ctx, &awskms.VerifyInput{
		KeyId: &signer.keyID, Message: digest, Signature: signature,
		MessageType: types.MessageTypeDigest, SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256,
	})
	if err != nil {
		return false, err
	}
	return output.SignatureValid, nil
}

func derToRawES256(encoded []byte) ([]byte, error) {
	var signature struct{ R, S *big.Int }
	remainder, err := asn1.Unmarshal(encoded, &signature)
	if err != nil || len(remainder) != 0 || signature.R == nil || signature.S == nil || signature.R.Sign() <= 0 || signature.S.Sign() <= 0 || signature.R.BitLen() > 256 || signature.S.BitLen() > 256 {
		return nil, authorization.ErrAuthorizationUnavailable
	}
	raw := make([]byte, 64)
	signature.R.FillBytes(raw[:32])
	signature.S.FillBytes(raw[32:])
	return raw, nil
}

var _ authorization.CapabilitySigner = (*CapabilityKeyRing)(nil)
var _ authorization.CapabilityKeyProvider = (*CapabilityKeyRing)(nil)
var _ evidence.Signer = (*EvidenceSigner)(nil)
