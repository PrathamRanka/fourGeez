package awskms

import (
	"bytes"
	"context"
	"testing"

	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
)

func TestEnvelopeProtectorUsesBoundEncryptionContext(t *testing.T) {
	t.Parallel()
	client := &fakeEnvelopeClient{}
	protector, err := NewEnvelopeProtector(client, "application-key")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := protector.Seal(t.Context(), []byte("rotation-response"))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := protector.Open(t.Context(), sealed)
	if err != nil || !bytes.Equal(opened, []byte("rotation-response")) {
		t.Fatalf("Open() = %q, %v", opened, err)
	}
	if client.encryptInput.EncryptionContext[envelopePurposeKey] != envelopePurposeValue || client.decryptInput.EncryptionContext[envelopePurposeKey] != envelopePurposeValue {
		t.Fatalf("encryption contexts = %#v / %#v", client.encryptInput.EncryptionContext, client.decryptInput.EncryptionContext)
	}
}

type fakeEnvelopeClient struct {
	encryptInput *awskms.EncryptInput
	decryptInput *awskms.DecryptInput
}

func (client *fakeEnvelopeClient) Encrypt(_ context.Context, input *awskms.EncryptInput, _ ...func(*awskms.Options)) (*awskms.EncryptOutput, error) {
	client.encryptInput = input
	return &awskms.EncryptOutput{CiphertextBlob: append([]byte("kms:"), input.Plaintext...)}, nil
}

func (client *fakeEnvelopeClient) Decrypt(_ context.Context, input *awskms.DecryptInput, _ ...func(*awskms.Options)) (*awskms.DecryptOutput, error) {
	client.decryptInput = input
	return &awskms.DecryptOutput{Plaintext: bytes.TrimPrefix(input.CiphertextBlob, []byte("kms:"))}, nil
}
