package browserpurchase

import (
	"context"

	"github.com/fourgeez/agentpay/internal/domain"
)

type Repository interface {
	FindCreation(context.Context, string, domain.IdempotencyKey, string) (BrowserPurchaseSession, bool, error)
	Create(context.Context, BrowserPurchaseSession, string, domain.IdempotencyKey, string) (BrowserPurchaseSession, bool, error)
	Get(context.Context, PurchaseSessionID) (BrowserPurchaseSession, error)
	GetByGrantHash(context.Context, string) (BrowserPurchaseSession, error)
	ClaimTransaction(context.Context, PurchaseSessionID, domain.ID, domain.Timestamp) (BrowserPurchaseSession, error)
	Complete(context.Context, BrowserPurchaseSession) error
	CreateChallenge(context.Context, BrowserPurchaseRecoveryChallengeRecord) error
	GetChallenge(context.Context, PurchaseSessionID, RecoveryChallengeID) (BrowserPurchaseRecoveryChallengeRecord, error)
	Recover(context.Context, BrowserPurchaseSession, string, BrowserPurchaseRecoveryChallengeRecord) error
}
