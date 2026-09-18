package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/persistence"
)

// IntegrationCredentialRepository stores credential snapshots for local use.
type IntegrationCredentialRepository struct {
	mutex           sync.RWMutex
	credentials     map[domain.ID]integrations.Snapshot
	rotationReplays map[string]integrations.RotationReplay
}

// NewIntegrationCredentialRepository creates an empty credential repository.
func NewIntegrationCredentialRepository() *IntegrationCredentialRepository {
	return &IntegrationCredentialRepository{
		credentials:     make(map[domain.ID]integrations.Snapshot),
		rotationReplays: make(map[string]integrations.RotationReplay),
	}
}

// Create stores one credential without replacing an existing identifier.
func (repository *IntegrationCredentialRepository) Create(
	_ context.Context,
	credential integrations.Credential,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	if _, exists := repository.credentials[credential.CredentialID()]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.credentials[credential.CredentialID()] = credential.Snapshot()
	return nil
}

// Get loads one credential only when the requested seller owns it.
func (repository *IntegrationCredentialRepository) Get(
	_ context.Context,
	sellerID domain.ID,
	credentialID domain.ID,
) (integrations.Credential, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()

	snapshot, exists := repository.credentials[credentialID]
	if !exists || snapshot.SellerID != sellerID {
		return integrations.Credential{}, persistence.ErrNotFound
	}
	return integrations.RestoreCredential(snapshot), nil
}

// GetByID resolves the non-secret public credential lookup identifier.
func (repository *IntegrationCredentialRepository) GetByID(
	_ context.Context,
	credentialID domain.ID,
) (integrations.Credential, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	snapshot, exists := repository.credentials[credentialID]
	if !exists {
		return integrations.Credential{}, persistence.ErrNotFound
	}
	return integrations.RestoreCredential(snapshot), nil
}

// ListBySeller returns deterministic credential metadata for one seller.
func (repository *IntegrationCredentialRepository) ListBySeller(
	_ context.Context,
	sellerID domain.ID,
) ([]integrations.Credential, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()

	credentials := make([]integrations.Credential, 0)
	for _, snapshot := range repository.credentials {
		if snapshot.SellerID == sellerID {
			credentials = append(
				credentials,
				integrations.RestoreCredential(snapshot),
			)
		}
	}
	sort.Slice(credentials, func(leftIndex, rightIndex int) bool {
		return credentials[leftIndex].CredentialID().String() <
			credentials[rightIndex].CredentialID().String()
	})
	return credentials, nil
}

// Update replaces one credential only at the expected version.
func (repository *IntegrationCredentialRepository) Update(
	_ context.Context,
	credential integrations.Credential,
	expectedVersion uint64,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	stored, exists := repository.credentials[credential.CredentialID()]
	if !exists || stored.SellerID != credential.SellerID() {
		return persistence.ErrNotFound
	}
	if stored.Version != expectedVersion ||
		credential.Version() != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	repository.credentials[credential.CredentialID()] = credential.Snapshot()
	return nil
}

// Rotate atomically revokes the predecessor and creates its successor.
func (repository *IntegrationCredentialRepository) Rotate(
	_ context.Context,
	predecessor integrations.Credential,
	successor integrations.Credential,
	expectedVersion uint64,
	replay integrations.RotationReplay,
) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.credentials[predecessor.CredentialID()]
	if !exists || stored.SellerID != predecessor.SellerID() {
		return persistence.ErrNotFound
	}
	if stored.Version != expectedVersion || predecessor.Version() != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	if _, exists := repository.credentials[successor.CredentialID()]; exists {
		return persistence.ErrAlreadyExists
	}
	replayKey := rotationReplayMapKey(replay.Scope, replay.Key)
	if _, exists := repository.rotationReplays[replayKey]; exists {
		return persistence.ErrConditionFailed
	}
	repository.credentials[predecessor.CredentialID()] = predecessor.Snapshot()
	repository.credentials[successor.CredentialID()] = successor.Snapshot()
	replay.ProtectedResponse = append([]byte(nil), replay.ProtectedResponse...)
	repository.rotationReplays[replayKey] = replay
	return nil
}

func (repository *IntegrationCredentialRepository) LoadRotationReplay(
	_ context.Context,
	scope string,
	key domain.IdempotencyKey,
) (integrations.RotationReplay, bool, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	replay, found := repository.rotationReplays[rotationReplayMapKey(scope, key)]
	if !found {
		return integrations.RotationReplay{}, false, nil
	}
	replay.ProtectedResponse = append([]byte(nil), replay.ProtectedResponse...)
	return replay, true, nil
}

func rotationReplayMapKey(scope string, key domain.IdempotencyKey) string {
	return scope + "\x00" + string(key)
}
