package awskms

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"math/big"
	"testing"

	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

func TestCapabilityKeyRingUsesLogicalKeyIDsAndRawES256Signatures(t *testing.T) {
	t.Parallel()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeKMSClient{privateKey: privateKey, publicKey: publicDER}
	ring, err := NewCapabilityKeyRing(client, "kms-key-v1", map[string]string{"v1": "kms-key-v1"})
	if err != nil {
		t.Fatal(err)
	}

	keyID, err := ring.CurrentKeyID(t.Context())
	if err != nil || keyID != "v1" {
		t.Fatalf("CurrentKeyID() = %q, %v", keyID, err)
	}
	signature, err := ring.Sign(t.Context(), keyID, []byte("header.payload"))
	if err != nil || len(signature) != 64 {
		t.Fatalf("Sign() length = %d, error = %v", len(signature), err)
	}
	publicKey, err := ring.VerificationKey(t.Context(), keyID)
	if err != nil || publicKey.X.Cmp(privateKey.X) != 0 || publicKey.Y.Cmp(privateKey.Y) != 0 {
		t.Fatalf("VerificationKey() = %#v, %v", publicKey, err)
	}
	jwks, err := ring.JWKS(t.Context())
	if err != nil || len(jwks.Keys) != 1 || jwks.Keys[0].KeyID != "v1" {
		t.Fatalf("JWKS() = %#v, %v", jwks, err)
	}
	if client.signInput == nil || client.signInput.MessageType != types.MessageTypeRaw {
		t.Fatalf("KMS Sign input = %#v", client.signInput)
	}
}

func TestEvidenceSignerUsesDigestModeAndKMSVerification(t *testing.T) {
	t.Parallel()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeKMSClient{privateKey: privateKey}
	signer, err := NewEvidenceSigner(client, "evidence-key")
	if err != nil {
		t.Fatal(err)
	}
	digest := make([]byte, 32)
	signature, err := signer.Sign(t.Context(), digest)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := signer.Verify(t.Context(), signature.KeyID, digest, signature.Value)
	if err != nil || !valid {
		t.Fatalf("Verify() = %v, %v", valid, err)
	}
	if client.signInput.MessageType != types.MessageTypeDigest || client.verifyInput.MessageType != types.MessageTypeDigest {
		t.Fatalf("message types = %q and %q", client.signInput.MessageType, client.verifyInput.MessageType)
	}
}

type fakeKMSClient struct {
	privateKey  *ecdsa.PrivateKey
	publicKey   []byte
	signInput   *awskms.SignInput
	verifyInput *awskms.VerifyInput
}

func (client *fakeKMSClient) Sign(_ context.Context, input *awskms.SignInput, _ ...func(*awskms.Options)) (*awskms.SignOutput, error) {
	client.signInput = input
	digest := input.Message
	if input.MessageType == types.MessageTypeRaw {
		hashed := sha256Digest(input.Message)
		digest = hashed
	}
	r, s, err := ecdsa.Sign(cryptorand.Reader, client.privateKey, digest)
	if err != nil {
		return nil, err
	}
	encoded, err := asn1.Marshal(struct{ R, S *big.Int }{r, s})
	return &awskms.SignOutput{Signature: encoded}, err
}

func (client *fakeKMSClient) Verify(_ context.Context, input *awskms.VerifyInput, _ ...func(*awskms.Options)) (*awskms.VerifyOutput, error) {
	client.verifyInput = input
	var signature struct{ R, S *big.Int }
	_, err := asn1.Unmarshal(input.Signature, &signature)
	if err != nil {
		return nil, err
	}
	return &awskms.VerifyOutput{SignatureValid: ecdsa.Verify(&client.privateKey.PublicKey, input.Message, signature.R, signature.S)}, nil
}

func (client *fakeKMSClient) GetPublicKey(_ context.Context, _ *awskms.GetPublicKeyInput, _ ...func(*awskms.Options)) (*awskms.GetPublicKeyOutput, error) {
	return &awskms.GetPublicKeyOutput{PublicKey: client.publicKey, KeySpec: types.KeySpecEccNistP256, KeyUsage: types.KeyUsageTypeSignVerify}, nil
}

func sha256Digest(value []byte) []byte {
	digest := sha256.Sum256(value)
	return digest[:]
}
