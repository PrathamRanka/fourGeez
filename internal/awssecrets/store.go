package awssecrets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/proxy"
)

const minimumSecretBytes = 32

type Client interface {
	CreateSecret(context.Context, *secretsmanager.CreateSecretInput, ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	GetSecretValue(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type PepperProvider struct {
	client   Client
	secretID string
	mutex    sync.RWMutex
	value    []byte
}

func NewPepperProvider(client Client, secretID string) (*PepperProvider, error) {
	secretID = strings.TrimSpace(secretID)
	if client == nil || secretID == "" {
		return nil, errors.New("Secrets Manager client and pepper secret ID are required")
	}
	return &PepperProvider{client: client, secretID: secretID}, nil
}

func (provider *PepperProvider) CredentialPepper(ctx context.Context) ([]byte, error) {
	provider.mutex.RLock()
	if len(provider.value) >= minimumSecretBytes {
		value := append([]byte(nil), provider.value...)
		provider.mutex.RUnlock()
		return value, nil
	}
	provider.mutex.RUnlock()

	value, err := loadSecret(ctx, provider.client, provider.secretID)
	if err != nil {
		return nil, err
	}
	if len(value) < minimumSecretBytes {
		return nil, errors.New("pepper secret must contain at least 32 bytes")
	}
	provider.mutex.Lock()
	if len(provider.value) == 0 {
		provider.value = append([]byte(nil), value...)
	}
	value = append([]byte(nil), provider.value...)
	provider.mutex.Unlock()
	return value, nil
}

type WebhookSecretStore struct {
	client       Client
	projectName  string
	environment  string
	kmsKeyID     string
	secretPrefix string
}

func NewWebhookSecretStore(client Client, projectName, environment, kmsKeyID string) (*WebhookSecretStore, error) {
	projectName = strings.TrimSpace(projectName)
	environment = strings.TrimSpace(environment)
	kmsKeyID = strings.TrimSpace(kmsKeyID)
	if client == nil || projectName == "" || environment == "" || kmsKeyID == "" {
		return nil, errors.New("Secrets Manager webhook store requires client, project, environment, and KMS key")
	}
	return &WebhookSecretStore{
		client: client, projectName: projectName, environment: environment, kmsKeyID: kmsKeyID,
		secretPrefix: projectName + "/" + environment + "/sellers/",
	}, nil
}

func (store *WebhookSecretStore) PutSecret(ctx context.Context, reference string, secret []byte) error {
	if len(secret) < minimumSecretBytes {
		return errors.New("webhook secret must contain at least 32 bytes")
	}
	secretID, err := store.resolveReference(reference)
	if err != nil {
		return err
	}
	_, err = store.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name: aws.String(secretID), Description: aws.String("AgentPay seller webhook signing secret"),
		KmsKeyId: aws.String(store.kmsKeyID), SecretBinary: append([]byte(nil), secret...),
		Tags: []types.Tag{
			{Key: aws.String("Project"), Value: aws.String("AgentPay")},
			{Key: aws.String("Environment"), Value: aws.String(store.environment)},
			{Key: aws.String("ManagedBy"), Value: aws.String("AgentPay")},
		},
	})
	return err
}

func (store *WebhookSecretStore) GetSecret(ctx context.Context, reference string) ([]byte, error) {
	secretID, err := store.resolveReference(reference)
	if err != nil {
		return nil, err
	}
	return loadSecret(ctx, store.client, secretID)
}

func (store *WebhookSecretStore) resolveReference(reference string) (string, error) {
	reference = strings.TrimSpace(reference)
	if strings.HasPrefix(reference, store.secretPrefix) {
		return reference, nil
	}
	prefix := store.projectName + "/webhooks/"
	if !strings.HasPrefix(reference, prefix) {
		return "", errors.New("secret reference is outside the AgentPay seller namespace")
	}
	suffix := strings.TrimPrefix(reference, prefix)
	parts := strings.Split(suffix, "/")
	if len(parts) != 2 || !safeReferencePart(parts[0]) || !safeReferencePart(parts[1]) {
		return "", errors.New("webhook secret reference is invalid")
	}
	return store.secretPrefix + parts[0] + "/webhooks/" + parts[1], nil
}

func loadSecret(ctx context.Context, client Client, secretID string) ([]byte, error) {
	output, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(secretID)})
	if err != nil {
		return nil, fmt.Errorf("load secret: %w", err)
	}
	var value []byte
	if output.SecretString != nil {
		value = []byte(*output.SecretString)
	} else {
		value = append([]byte(nil), output.SecretBinary...)
	}
	if len(value) == 0 {
		return nil, errors.New("secret has no current value")
	}
	return value, nil
}

func safeReferencePart(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

var _ integrations.CredentialPepperProvider = (*PepperProvider)(nil)
var _ notifications.SecretStore = (*WebhookSecretStore)(nil)
var _ notifications.SecretProvider = (*WebhookSecretStore)(nil)
var _ proxy.SecretProvider = (*WebhookSecretStore)(nil)
