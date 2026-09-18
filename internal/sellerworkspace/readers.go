package sellerworkspace

import (
	"context"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/transactions"
)

type credentialListRepository interface {
	ListBySeller(context.Context, domain.ID) ([]integrations.Credential, error)
}

type CredentialRepositoryReader struct {
	repository credentialListRepository
}

func NewCredentialRepositoryReader(repository credentialListRepository) *CredentialRepositoryReader {
	return &CredentialRepositoryReader{repository: repository}
}

func (reader *CredentialRepositoryReader) ListCredentials(ctx context.Context, sellerID domain.ID) ([]integrations.CredentialView, error) {
	credentials, err := reader.repository.ListBySeller(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	views := make([]integrations.CredentialView, len(credentials))
	for index, credential := range credentials {
		snapshot := credential.Snapshot()
		views[index] = integrations.CredentialView{
			CredentialID: snapshot.CredentialID, SellerID: snapshot.SellerID, Label: snapshot.Label,
			Scopes: append([]integrations.Scope(nil), snapshot.Scopes...), ExpiresAt: snapshot.ExpiresAt,
			RevokedAt: snapshot.RevokedAt, LastUsedAt: snapshot.LastUsedAt,
			ReplacedByCredentialID: snapshot.ReplacedByCredentialID, CreatedAt: snapshot.CreatedAt,
			UpdatedAt: snapshot.UpdatedAt, Version: snapshot.Version,
		}
	}
	return views, nil
}

type paymentDestinationListRepository interface {
	ListBySeller(context.Context, domain.ID) ([]settlement.PaymentDestination, error)
}

type PaymentDestinationRepositoryReader struct {
	repository paymentDestinationListRepository
}

func NewPaymentDestinationRepositoryReader(repository paymentDestinationListRepository) *PaymentDestinationRepositoryReader {
	return &PaymentDestinationRepositoryReader{repository: repository}
}
func (reader *PaymentDestinationRepositoryReader) ListPaymentDestinations(ctx context.Context, sellerID domain.ID) ([]settlement.PaymentDestination, error) {
	return reader.repository.ListBySeller(ctx, sellerID)
}

type transactionListRepository interface {
	ListBySeller(context.Context, domain.ID, int, string) ([]transactions.Transaction, *string, error)
}

type TransactionRepositoryReader struct{ repository transactionListRepository }

func NewTransactionRepositoryReader(repository transactionListRepository) *TransactionRepositoryReader {
	return &TransactionRepositoryReader{repository: repository}
}
func (reader *TransactionRepositoryReader) ListTransactions(ctx context.Context, sellerID domain.ID, limit int, cursor string) ([]transactions.Transaction, *string, error) {
	return reader.repository.ListBySeller(ctx, sellerID, limit, cursor)
}

type webhookSubscriptionListRepository interface {
	ListBySeller(context.Context, domain.ID) ([]notifications.Subscription, error)
}

type WebhookSubscriptionRepositoryReader struct {
	repository webhookSubscriptionListRepository
}

func NewWebhookSubscriptionRepositoryReader(repository webhookSubscriptionListRepository) *WebhookSubscriptionRepositoryReader {
	return &WebhookSubscriptionRepositoryReader{repository: repository}
}
func (reader *WebhookSubscriptionRepositoryReader) ListWebhookSubscriptions(ctx context.Context, sellerID domain.ID) ([]notifications.SubscriptionView, error) {
	subscriptions, err := reader.repository.ListBySeller(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	views := make([]notifications.SubscriptionView, len(subscriptions))
	for index, subscription := range subscriptions {
		snapshot := subscription.Snapshot()
		views[index] = notifications.SubscriptionView{SubscriptionID: snapshot.SubscriptionID, SellerID: snapshot.SellerID, EndpointURL: snapshot.EndpointURL, EventTypes: append([]notifications.EventType(nil), snapshot.EventTypes...), Status: snapshot.Status, CreatedAt: snapshot.CreatedAt, UpdatedAt: snapshot.UpdatedAt, Version: snapshot.Version}
	}
	return views, nil
}

type webhookDeliveryListRepository interface {
	ListBySeller(context.Context, domain.ID, int, string) ([]notifications.Delivery, *string, error)
}

type WebhookDeliveryRepositoryReader struct{ repository webhookDeliveryListRepository }

func NewWebhookDeliveryRepositoryReader(repository webhookDeliveryListRepository) *WebhookDeliveryRepositoryReader {
	return &WebhookDeliveryRepositoryReader{repository: repository}
}
func (reader *WebhookDeliveryRepositoryReader) ListWebhookDeliveries(ctx context.Context, sellerID domain.ID, limit int, cursor string) ([]notifications.DeliveryView, *string, error) {
	deliveries, nextCursor, err := reader.repository.ListBySeller(ctx, sellerID, limit, cursor)
	if err != nil {
		return nil, nil, err
	}
	views := make([]notifications.DeliveryView, len(deliveries))
	for index, delivery := range deliveries {
		snapshot := delivery.Snapshot()
		views[index] = notifications.DeliveryView{DeliveryID: snapshot.DeliveryID, SellerID: snapshot.SellerID, SubscriptionID: snapshot.SubscriptionID, EventID: snapshot.Event.EventID, EventType: snapshot.Event.EventType, PayloadHash: snapshot.PayloadHash, Status: snapshot.Status, AttemptCount: snapshot.AttemptCount, NextAttemptAt: snapshot.NextAttemptAt, LastAttemptAt: snapshot.LastAttemptAt, DeliveredAt: snapshot.DeliveredAt, ResponseStatusCode: snapshot.ResponseStatusCode, ResponseBodyHash: snapshot.ResponseBodyHash, ErrorCode: snapshot.ErrorCode, CreatedAt: snapshot.CreatedAt, UpdatedAt: snapshot.UpdatedAt, Version: snapshot.Version}
	}
	return views, nextCursor, nil
}
