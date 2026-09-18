//go:build agentpay_dev

package memory

import (
	"context"
	"time"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/transactions"
)

// DevelopmentRepositories identifies disposable repositories owned by the local API.
type DevelopmentRepositories struct {
	Catalog                  *CatalogRepository
	PurchaseIntents          *PurchaseIntentRepository
	Approvals                *ApprovalRepository
	Transactions             *TransactionRepository
	Evidence                 *EvidenceRepository
	Disputes                 *DisputeRepository
	PaymentDestinations      *PaymentDestinationRepository
	WebhookSubscriptions     *WebhookSubscriptionRepository
	WebhookDeliveries        *WebhookDeliveryRepository
	WebhookSecrets           *WebhookSecretStore
	IntegrationCredentials   *IntegrationCredentialRepository
	ConfirmationGrants       *ConfirmationGrantRepository
	SellerEntitlements       *SellerEntitlementRepository
	ProviderEvents           *ProviderEventRepository
	AuditEvents              *AuditEventRepository
	Idempotency              *IdempotencyStore
	SellerSessionRevocations *SellerSessionRevocationRepository
}

// DevelopmentResetter clears process-local state without exposing production stores.
type DevelopmentResetter struct {
	repositories DevelopmentRepositories
}

// NewDevelopmentResetter creates the build-tagged local reset boundary.
func NewDevelopmentResetter(repositories DevelopmentRepositories) *DevelopmentResetter {
	return &DevelopmentResetter{repositories: repositories}
}

// Reset discards all configured in-memory state. The process remains running.
func (resetter *DevelopmentResetter) Reset(_ context.Context) error {
	resetCatalog(resetter.repositories.Catalog)
	resetPurchaseIntents(resetter.repositories.PurchaseIntents)
	resetApprovals(resetter.repositories.Approvals)
	resetTransactions(resetter.repositories.Transactions)
	resetEvidence(resetter.repositories.Evidence)
	resetDisputes(resetter.repositories.Disputes)
	resetPaymentDestinations(resetter.repositories.PaymentDestinations)
	resetWebhookSubscriptions(resetter.repositories.WebhookSubscriptions)
	resetWebhookDeliveries(resetter.repositories.WebhookDeliveries)
	resetWebhookSecrets(resetter.repositories.WebhookSecrets)
	resetIntegrationCredentials(resetter.repositories.IntegrationCredentials)
	resetConfirmationGrants(resetter.repositories.ConfirmationGrants)
	resetSellerEntitlements(resetter.repositories.SellerEntitlements)
	resetProviderEvents(resetter.repositories.ProviderEvents)
	resetAuditEvents(resetter.repositories.AuditEvents)
	resetIdempotency(resetter.repositories.Idempotency)
	resetSellerSessionRevocations(resetter.repositories.SellerSessionRevocations)
	return nil
}

func resetSellerEntitlements(repository *SellerEntitlementRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.entitlements = make(map[domain.ID]billing.SellerEntitlementSnapshot)
	repository.reconciliations = make(map[domain.ID]map[string]billing.EntitlementReconciliation)
}

func resetProviderEvents(repository *ProviderEventRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.events = make(map[string]billing.SubscriptionProviderEvent)
}

func resetCatalog(repository *CatalogRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.sellers = make(map[domain.ID]catalog.Seller)
	repository.sellerBySlug = make(map[string]domain.ID)
	repository.sellerByOwnerSubject = make(map[string]domain.ID)
	repository.routes = make(map[domain.ID]catalog.PaidRoute)
	repository.routesBySeller = make(map[domain.ID][]domain.ID)
	repository.productSlugsBySeller = make(map[domain.ID]map[string]domain.ID)
}

func resetSellerSessionRevocations(repository *SellerSessionRevocationRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.revocations = make(map[string]time.Time)
}

func resetPurchaseIntents(repository *PurchaseIntentRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.intents = make(map[domain.ID]intents.PurchaseIntent)
}

func resetApprovals(repository *ApprovalRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.sessions = make(map[domain.ID]approvals.Session)
}

func resetTransactions(repository *TransactionRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.transactions = make(map[domain.ID]transactions.Transaction)
	repository.paymentIdentifiers = make(map[string]domain.ID)
}

func resetEvidence(repository *EvidenceRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.events = make(map[domain.ID][]evidence.Event)
}

func resetDisputes(repository *DisputeRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.disputes = make(map[domain.ID]disputes.Dispute)
}

func resetPaymentDestinations(repository *PaymentDestinationRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.destinations = make(map[domain.ID]settlement.PaymentDestination)
}

func resetWebhookSubscriptions(repository *WebhookSubscriptionRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.subscriptions = make(map[domain.ID]notifications.SubscriptionSnapshot)
}

func resetWebhookDeliveries(repository *WebhookDeliveryRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.deliveries = make(map[domain.ID]notifications.DeliverySnapshot)
	repository.identities = make(map[string]domain.ID)
}

func resetWebhookSecrets(store *WebhookSecretStore) {
	if store == nil {
		return
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.secrets = make(map[string][]byte)
}

func resetIntegrationCredentials(repository *IntegrationCredentialRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.credentials = make(map[domain.ID]integrations.Snapshot)
	repository.rotationReplays = make(map[string]integrations.RotationReplay)
}

func resetConfirmationGrants(repository *ConfirmationGrantRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.grants = make(map[domain.ID]authorization.ConfirmationGrant)
	repository.bindings = make(map[string]domain.ID)
}

func resetAuditEvents(repository *AuditEventRepository) {
	if repository == nil {
		return
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.events = make(map[domain.ID]audit.EventSnapshot)
}

func resetIdempotency(store *IdempotencyStore) {
	if store == nil {
		return
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.records = make(map[string]domain.IdempotencyRecord)
}
