package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/notifications"
)

// WebhookSecretStore holds development-only webhook secrets in process memory.
type WebhookSecretStore struct {
	mutex   sync.RWMutex
	secrets map[string][]byte
}

// NewWebhookSecretStore creates an empty local secret store.
func NewWebhookSecretStore() *WebhookSecretStore {
	return &WebhookSecretStore{secrets: make(map[string][]byte)}
}

// PutSecret stores a copied secret under its opaque reference.
func (store *WebhookSecretStore) PutSecret(
	_ context.Context,
	reference string,
	secret []byte,
) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.secrets[reference] = append([]byte(nil), secret...)
	return nil
}

// GetSecret returns a copied secret for outbound signing.
func (store *WebhookSecretStore) GetSecret(
	_ context.Context,
	reference string,
) ([]byte, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	secret, exists := store.secrets[reference]
	if !exists {
		return nil, notifications.ErrSigningUnavailable
	}
	return append([]byte(nil), secret...), nil
}
