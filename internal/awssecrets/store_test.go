package awssecrets

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func TestPepperProviderLoadsAndCachesSecretValue(t *testing.T) {
	t.Parallel()
	client := &fakeSecretsClient{secretString: "0123456789abcdef0123456789abcdef"}
	provider, err := NewPepperProvider(client, "arn:aws:secretsmanager:ap-south-1:123456789012:secret:pepper")
	if err != nil {
		t.Fatal(err)
	}
	first, err := provider.CredentialPepper(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.CredentialPepper(t.Context())
	if err != nil || string(first) != string(second) || client.getCalls != 1 {
		t.Fatalf("cached pepper = %q/%q calls=%d error=%v", first, second, client.getCalls, err)
	}
}

func TestWebhookStoreMapsLogicalReferenceInsideSellerNamespace(t *testing.T) {
	t.Parallel()
	client := &fakeSecretsClient{secretString: "webhook-secret-value-with-32-bytes"}
	store, err := NewWebhookSecretStore(client, "agentpay", "dev", "kms-key")
	if err != nil {
		t.Fatal(err)
	}
	logicalReference := "agentpay/webhooks/sel_123/whk_456"
	if err := store.PutSecret(t.Context(), logicalReference, []byte("webhook-secret-value-with-32-bytes")); err != nil {
		t.Fatal(err)
	}
	if client.createInput == nil || *client.createInput.Name != "agentpay/dev/sellers/sel_123/webhooks/whk_456" {
		t.Fatalf("CreateSecret name = %#v", client.createInput)
	}
	loaded, err := store.GetSecret(t.Context(), logicalReference)
	if err != nil || string(loaded) != client.secretString {
		t.Fatalf("GetSecret() = %q, %v", loaded, err)
	}
	if client.getInput == nil || *client.getInput.SecretId != "agentpay/dev/sellers/sel_123/webhooks/whk_456" {
		t.Fatalf("GetSecretValue ID = %#v", client.getInput)
	}
}

type fakeSecretsClient struct {
	secretString string
	getCalls     int
	createInput  *secretsmanager.CreateSecretInput
	getInput     *secretsmanager.GetSecretValueInput
}

func (client *fakeSecretsClient) CreateSecret(_ context.Context, input *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	client.createInput = input
	return &secretsmanager.CreateSecretOutput{}, nil
}

func (client *fakeSecretsClient) GetSecretValue(_ context.Context, input *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	client.getCalls++
	client.getInput = input
	return &secretsmanager.GetSecretValueOutput{SecretString: &client.secretString}, nil
}
