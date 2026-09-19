package awskms

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/fourgeez/agentpay/internal/integrations"
)

const (
	envelopePurposeKey   = "agentpay-purpose"
	envelopePurposeValue = "credential-rotation-replay"
)

type EnvelopeClient interface {
	Encrypt(context.Context, *awskms.EncryptInput, ...func(*awskms.Options)) (*awskms.EncryptOutput, error)
	Decrypt(context.Context, *awskms.DecryptInput, ...func(*awskms.Options)) (*awskms.DecryptOutput, error)
}

type EnvelopeProtector struct {
	client EnvelopeClient
	keyID  string
}

func NewEnvelopeProtector(client EnvelopeClient, keyID string) (*EnvelopeProtector, error) {
	keyID = strings.TrimSpace(keyID)
	if client == nil || keyID == "" {
		return nil, errors.New("KMS envelope protector requires client and key ID")
	}
	return &EnvelopeProtector{client: client, keyID: keyID}, nil
}

func (protector *EnvelopeProtector) Seal(ctx context.Context, plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 || len(plaintext) > 4096 {
		return nil, errors.New("rotation replay plaintext is outside the KMS envelope limit")
	}
	output, err := protector.client.Encrypt(ctx, &awskms.EncryptInput{
		KeyId: aws.String(protector.keyID), Plaintext: plaintext,
		EncryptionContext: map[string]string{envelopePurposeKey: envelopePurposeValue},
	})
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), output.CiphertextBlob...), nil
}

func (protector *EnvelopeProtector) Open(ctx context.Context, protected []byte) ([]byte, error) {
	if len(protected) == 0 || len(protected) > 8192 {
		return nil, errors.New("rotation replay ciphertext is invalid")
	}
	output, err := protector.client.Decrypt(ctx, &awskms.DecryptInput{
		KeyId: aws.String(protector.keyID), CiphertextBlob: protected,
		EncryptionContext: map[string]string{envelopePurposeKey: envelopePurposeValue},
	})
	if err != nil {
		return nil, err
	}
	if len(output.Plaintext) == 0 {
		return nil, errors.New("rotation replay plaintext is empty")
	}
	return append([]byte(nil), output.Plaintext...), nil
}

var _ integrations.RotationReplayProtector = (*EnvelopeProtector)(nil)
