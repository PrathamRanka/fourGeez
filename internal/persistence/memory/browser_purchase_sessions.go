package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

type browserPurchaseCreationRecord struct {
	requestHash       string
	purchaseSessionID browserpurchase.PurchaseSessionID
}

type BrowserPurchaseSessionRepository struct {
	mutex       sync.RWMutex
	sessions    map[browserpurchase.PurchaseSessionID]browserpurchase.BrowserPurchaseSession
	grantIndex  map[string]browserpurchase.PurchaseSessionID
	idempotency map[string]browserPurchaseCreationRecord
	challenges  map[browserpurchase.RecoveryChallengeID]browserpurchase.BrowserPurchaseRecoveryChallengeRecord
}

func NewBrowserPurchaseSessionRepository() *BrowserPurchaseSessionRepository {
	return &BrowserPurchaseSessionRepository{
		sessions:    make(map[browserpurchase.PurchaseSessionID]browserpurchase.BrowserPurchaseSession),
		grantIndex:  make(map[string]browserpurchase.PurchaseSessionID),
		idempotency: make(map[string]browserPurchaseCreationRecord),
		challenges:  make(map[browserpurchase.RecoveryChallengeID]browserpurchase.BrowserPurchaseRecoveryChallengeRecord),
	}
}

func (repository *BrowserPurchaseSessionRepository) FindCreation(_ context.Context, scope string, key domain.IdempotencyKey, requestHash string) (browserpurchase.BrowserPurchaseSession, bool, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	record, exists := repository.idempotency[browserPurchaseIdempotencyKey(scope, key)]
	if !exists {
		return browserpurchase.BrowserPurchaseSession{}, false, nil
	}
	if record.requestHash != requestHash {
		return browserpurchase.BrowserPurchaseSession{}, false, browserpurchase.ErrIdempotencyConflict
	}
	stored, exists := repository.sessions[record.purchaseSessionID]
	if !exists {
		return browserpurchase.BrowserPurchaseSession{}, false, persistence.ErrConditionFailed
	}
	return cloneBrowserPurchaseSession(stored), true, nil
}

func (repository *BrowserPurchaseSessionRepository) Create(_ context.Context, session browserpurchase.BrowserPurchaseSession, scope string, key domain.IdempotencyKey, requestHash string) (browserpurchase.BrowserPurchaseSession, bool, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	indexKey := browserPurchaseIdempotencyKey(scope, key)
	if record, exists := repository.idempotency[indexKey]; exists {
		if record.requestHash != requestHash {
			return browserpurchase.BrowserPurchaseSession{}, false, browserpurchase.ErrIdempotencyConflict
		}
		stored, exists := repository.sessions[record.purchaseSessionID]
		if !exists {
			return browserpurchase.BrowserPurchaseSession{}, false, persistence.ErrConditionFailed
		}
		return cloneBrowserPurchaseSession(stored), true, nil
	}
	if _, exists := repository.sessions[session.PurchaseSessionID]; exists {
		return browserpurchase.BrowserPurchaseSession{}, false, persistence.ErrAlreadyExists
	}
	if _, exists := repository.grantIndex[session.BrowserGrantHash]; exists {
		return browserpurchase.BrowserPurchaseSession{}, false, persistence.ErrAlreadyExists
	}
	repository.sessions[session.PurchaseSessionID] = cloneBrowserPurchaseSession(session)
	repository.grantIndex[session.BrowserGrantHash] = session.PurchaseSessionID
	repository.idempotency[indexKey] = browserPurchaseCreationRecord{requestHash: requestHash, purchaseSessionID: session.PurchaseSessionID}
	return cloneBrowserPurchaseSession(session), false, nil
}

func (repository *BrowserPurchaseSessionRepository) Get(_ context.Context, purchaseSessionID browserpurchase.PurchaseSessionID) (browserpurchase.BrowserPurchaseSession, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	session, exists := repository.sessions[purchaseSessionID]
	if !exists {
		return browserpurchase.BrowserPurchaseSession{}, persistence.ErrNotFound
	}
	return cloneBrowserPurchaseSession(session), nil
}

func (repository *BrowserPurchaseSessionRepository) GetByGrantHash(_ context.Context, grantHash string) (browserpurchase.BrowserPurchaseSession, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	purchaseSessionID, exists := repository.grantIndex[grantHash]
	if !exists {
		return browserpurchase.BrowserPurchaseSession{}, persistence.ErrNotFound
	}
	session, exists := repository.sessions[purchaseSessionID]
	if !exists {
		return browserpurchase.BrowserPurchaseSession{}, persistence.ErrNotFound
	}
	return cloneBrowserPurchaseSession(session), nil
}

func (repository *BrowserPurchaseSessionRepository) ClaimTransaction(_ context.Context, purchaseSessionID browserpurchase.PurchaseSessionID, transactionID domain.ID, updatedAt domain.Timestamp) (browserpurchase.BrowserPurchaseSession, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	session, exists := repository.sessions[purchaseSessionID]
	if !exists {
		return browserpurchase.BrowserPurchaseSession{}, persistence.ErrNotFound
	}
	if session.Status != browserpurchase.StatusActive {
		return browserpurchase.BrowserPurchaseSession{}, persistence.ErrConditionFailed
	}
	if session.TransactionID != nil {
		if *session.TransactionID == transactionID {
			return cloneBrowserPurchaseSession(session), nil
		}
		return browserpurchase.BrowserPurchaseSession{}, persistence.ErrConditionFailed
	}
	session.TransactionID = &transactionID
	session.UpdatedAt = updatedAt
	repository.sessions[purchaseSessionID] = cloneBrowserPurchaseSession(session)
	return cloneBrowserPurchaseSession(session), nil
}

func (repository *BrowserPurchaseSessionRepository) Complete(_ context.Context, session browserpurchase.BrowserPurchaseSession) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.sessions[session.PurchaseSessionID]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Status != browserpurchase.StatusActive || stored.TransactionID != nil && (session.TransactionID == nil || *stored.TransactionID != *session.TransactionID) || session.Status != browserpurchase.StatusCompleted || session.TransactionID == nil || session.WalletBindingHash == nil {
		return persistence.ErrConditionFailed
	}
	repository.sessions[session.PurchaseSessionID] = cloneBrowserPurchaseSession(session)
	return nil
}

func (repository *BrowserPurchaseSessionRepository) CreateChallenge(_ context.Context, challenge browserpurchase.BrowserPurchaseRecoveryChallengeRecord) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.sessions[challenge.PurchaseSessionID]; !exists {
		return persistence.ErrNotFound
	}
	if _, exists := repository.challenges[challenge.ChallengeID]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.challenges[challenge.ChallengeID] = cloneBrowserPurchaseChallenge(challenge)
	return nil
}

func (repository *BrowserPurchaseSessionRepository) GetChallenge(_ context.Context, challengeID browserpurchase.RecoveryChallengeID) (browserpurchase.BrowserPurchaseRecoveryChallengeRecord, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	challenge, exists := repository.challenges[challengeID]
	if !exists {
		return browserpurchase.BrowserPurchaseRecoveryChallengeRecord{}, persistence.ErrNotFound
	}
	return cloneBrowserPurchaseChallenge(challenge), nil
}

func (repository *BrowserPurchaseSessionRepository) Recover(_ context.Context, session browserpurchase.BrowserPurchaseSession, previousGrantHash string, challenge browserpurchase.BrowserPurchaseRecoveryChallengeRecord) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	storedSession, sessionExists := repository.sessions[session.PurchaseSessionID]
	storedChallenge, challengeExists := repository.challenges[challenge.ChallengeID]
	if !sessionExists || !challengeExists {
		return persistence.ErrNotFound
	}
	if storedSession.BrowserGrantHash != previousGrantHash || storedSession.Status != browserpurchase.StatusCompleted ||
		storedChallenge.PurchaseSessionID != session.PurchaseSessionID || storedChallenge.UsedAt != nil || challenge.UsedAt == nil ||
		storedChallenge.MessageHash != challenge.MessageHash || storedChallenge.ExpectedWalletBindingHash != challenge.ExpectedWalletBindingHash {
		return persistence.ErrConditionFailed
	}
	if existingSessionID, exists := repository.grantIndex[session.BrowserGrantHash]; exists && existingSessionID != session.PurchaseSessionID {
		return persistence.ErrConditionFailed
	}
	delete(repository.grantIndex, previousGrantHash)
	repository.sessions[session.PurchaseSessionID] = cloneBrowserPurchaseSession(session)
	repository.grantIndex[session.BrowserGrantHash] = session.PurchaseSessionID
	repository.challenges[challenge.ChallengeID] = cloneBrowserPurchaseChallenge(challenge)
	return nil
}

func browserPurchaseIdempotencyKey(scope string, key domain.IdempotencyKey) string {
	return scope + "\x00" + string(key)
}

func cloneBrowserPurchaseSession(session browserpurchase.BrowserPurchaseSession) browserpurchase.BrowserPurchaseSession {
	if session.WalletBindingHash != nil {
		walletBindingHash := *session.WalletBindingHash
		session.WalletBindingHash = &walletBindingHash
	}
	if session.TransactionID != nil {
		transactionID := *session.TransactionID
		session.TransactionID = &transactionID
	}
	return session
}

func cloneBrowserPurchaseChallenge(challenge browserpurchase.BrowserPurchaseRecoveryChallengeRecord) browserpurchase.BrowserPurchaseRecoveryChallengeRecord {
	if challenge.UsedAt != nil {
		usedAt := *challenge.UsedAt
		challenge.UsedAt = &usedAt
	}
	return challenge
}

var _ browserpurchase.Repository = (*BrowserPurchaseSessionRepository)(nil)
