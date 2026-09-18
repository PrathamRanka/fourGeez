//go:build agentpay_dev

package main

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/fourgeez/agentpay/internal/devseed"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
)

// configureDevelopmentSeed validates, loads, and exposes the local reset route.
func configureDevelopmentSeed(
	mux *http.ServeMux,
	config developmentSeedConfig,
	repositories developmentSeedRepositories,
	signer evidence.Signer,
) error {
	if config.ProfileName == "" {
		return nil
	}
	resetter := memory.NewDevelopmentResetter(memory.DevelopmentRepositories{
		Catalog:                repositories.Catalog,
		PurchaseIntents:        repositories.PurchaseIntents,
		Approvals:              repositories.Approvals,
		Transactions:           repositories.Transactions,
		Evidence:               repositories.Evidence,
		Disputes:               repositories.Disputes,
		PaymentDestinations:    repositories.PaymentDestinations,
		WebhookSubscriptions:   repositories.WebhookSubscriptions,
		WebhookDeliveries:      repositories.WebhookDeliveries,
		WebhookSecrets:         repositories.WebhookSecrets,
		IntegrationCredentials: repositories.IntegrationCredentials,
		ConfirmationGrants:     repositories.ConfirmationGrants,
		SellerEntitlements:     repositories.SellerEntitlements,
		ProviderEvents:         repositories.ProviderEvents,
		AuditEvents:            repositories.AuditEvents,
		Idempotency:            repositories.Idempotency,
	})
	seeder, err := devseed.New(devseed.Config{
		Environment:          config.Environment,
		RepositoryMode:       config.RepositoryMode,
		HTTPAddress:          config.HTTPAddress,
		ProfileName:          config.ProfileName,
		WebhookSigningSecret: config.WebhookSigningSecret,
	}, devseed.Repositories{
		Catalog:              repositories.Catalog,
		PurchaseIntents:      repositories.PurchaseIntents,
		Transactions:         repositories.Transactions,
		Evidence:             repositories.Evidence,
		Disputes:             repositories.Disputes,
		PaymentDestinations:  repositories.PaymentDestinations,
		WebhookSubscriptions: repositories.WebhookSubscriptions,
		WebhookDeliveries:    repositories.WebhookDeliveries,
		WebhookSecrets:       repositories.WebhookSecrets,
		SellerEntitlements:   repositories.SellerEntitlements,
		Reset:                resetter.Reset,
	}, signer)
	if err != nil {
		return err
	}
	metadata, err := seeder.ResetAndSeed(context.Background())
	if err != nil {
		return err
	}
	seeder.RegisterRoutes(mux)
	slog.Info(
		"loaded local development seed profile",
		"profile", metadata.ProfileName,
		"sellerId", metadata.LaunchReadySellerID,
		"incompleteSellerId", metadata.IncompleteSellerID,
	)
	return nil
}
