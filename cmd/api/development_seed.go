package main

import "github.com/fourgeez/agentpay/internal/persistence/memory"

// developmentSeedConfig contains only explicit local-runtime gates.
type developmentSeedConfig struct {
	Environment          string
	RepositoryMode       string
	HTTPAddress          string
	ProfileName          string
	WebhookSigningSecret string
}

// developmentSeedRepositories exposes the existing API repository instances.
type developmentSeedRepositories struct {
	Catalog                *memory.CatalogRepository
	PurchaseIntents        *memory.PurchaseIntentRepository
	Approvals              *memory.ApprovalRepository
	Transactions           *memory.TransactionRepository
	Evidence               *memory.EvidenceRepository
	Disputes               *memory.DisputeRepository
	PaymentDestinations    *memory.PaymentDestinationRepository
	WebhookSubscriptions   *memory.WebhookSubscriptionRepository
	WebhookDeliveries      *memory.WebhookDeliveryRepository
	WebhookSecrets         *memory.WebhookSecretStore
	IntegrationCredentials *memory.IntegrationCredentialRepository
	AuditEvents            *memory.AuditEventRepository
	Idempotency            *memory.IdempotencyStore
}
