package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awssdkdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awskmssdk "github.com/aws/aws-sdk-go-v2/service/kms"
	awss3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	awssecretssdk "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/awskms"
	"github.com/fourgeez/agentpay/internal/awssecrets"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/browserpurchase"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/evidencestore"
	"github.com/fourgeez/agentpay/internal/health"
	"github.com/fourgeez/agentpay/internal/identity"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/integrations/mcpserver"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/operations"
	"github.com/fourgeez/agentpay/internal/payments"
	dynamorepository "github.com/fourgeez/agentpay/internal/persistence/dynamodb"
	"github.com/fourgeez/agentpay/internal/persistence/memory"
	"github.com/fourgeez/agentpay/internal/proxy"
	"github.com/fourgeez/agentpay/internal/publicationops"
	"github.com/fourgeez/agentpay/internal/sellerworkspace"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/storefront"
	"github.com/fourgeez/agentpay/internal/transactions"
	x402 "github.com/x402-foundation/x402/go/v2"
	x402http "github.com/x402-foundation/x402/go/v2/http"
)

const awsDependencyHealthTimeout = 5 * time.Second

type transactionStore interface {
	payments.CheckoutTransactionRepository
	proxy.ForwardingRepository
	transactions.ReadRepository
	billing.TransactionReader
	mcpserver.TransactionReader
}

type webhookSecretStore interface {
	notifications.SecretStore
	notifications.SecretProvider
	proxy.SecretProvider
}

type capabilityKeyRing interface {
	authorization.CapabilitySigner
	authorization.CapabilityKeyProvider
}

type runtimeDependencies struct {
	catalog                    catalog.Repository
	intents                    intentsRepository
	transactions               transactionStore
	evidence                   evidence.Repository
	disputes                   disputes.Repository
	manualRefunds              disputes.ManualRefundRecordRepository
	browserPurchases           browserpurchase.Repository
	integrationCredentials     integrations.Repository
	confirmationGrants         authorization.ConfirmationGrantRepository
	paymentDestinations        settlement.Repository
	webhookSubscriptions       notifications.Repository
	webhookDeliveries          notifications.DeliveryRepository
	sellerEntitlements         billing.Repository
	providerEvents             billing.ProviderEventRepository
	usageMeterEvents           billing.UsageRepository
	quotaCounters              operations.Repository
	sellerSessionRevocations   identity.RevocationRepository
	sellerWorkspaces           sellerworkspace.Repository
	storefrontPublications     storefront.PublicationRepository
	publicationCompletions     publicationops.CompletionStore
	auditEvents                audit.Repository
	idempotency                domain.IdempotencyStore
	projectKeyExchangeLimiter  integrations.ExchangeRateLimiter
	credentialPepper           integrations.CredentialPepperProvider
	confirmationGrantPepper    integrations.CredentialPepperProvider
	rotationReplayProtector    integrations.RotationReplayProtector
	capabilityKeys             capabilityKeyRing
	evidenceSigner             evidence.Signer
	sellerSigner               proxy.RequestSigner
	webhookSecrets             webhookSecretStore
	paymentAdapter             payments.Adapter
	sellerForwarder            proxy.SellerForwarder
	healthController           *health.Controller
	configureDevelopmentSeed   func(*http.ServeMux) error
	stripeProviderEventEnabled bool
}

type intentsRepository interface {
	Create(context.Context, intents.PurchaseIntent) error
	Get(context.Context, domain.ID) (intents.PurchaseIntent, error)
	Update(context.Context, intents.PurchaseIntent, uint64) error
}

type runtimeConfig struct {
	Environment                  string
	RepositoryMode               string
	HTTPAddress                  string
	AWSRegion                    string
	WebOrigin                    string
	APIOrigin                    string
	PublicBaseURL                string
	TableName                    string
	EvidenceBucket               string
	EvidenceKMSKeyID             string
	CapabilitySigningKeyID       string
	CapabilityVerificationKeyIDs map[string]string
	ApplicationSecretsKMSKeyID   string
	CredentialPepperSecretARN    string
	ConfirmationPepperSecretARN  string
	SellerUserPoolID             string
	SellerUserPoolClientID       string
	PaymentMode                  string
	FacilitatorURL               string
	X402Network                  string
	X402Asset                    string
}

func loadRuntimeConfig() (runtimeConfig, error) {
	config := runtimeConfig{
		Environment: os.Getenv("AGENTPAY_ENV"), RepositoryMode: os.Getenv("AGENTPAY_REPOSITORY_MODE"),
		HTTPAddress: os.Getenv("AGENTPAY_HTTP_ADDR"), AWSRegion: os.Getenv("AWS_REGION"),
		WebOrigin: os.Getenv("AGENTPAY_WEB_ORIGIN"), APIOrigin: os.Getenv("AGENTPAY_API_ORIGIN"),
		PublicBaseURL: os.Getenv("AGENTPAY_PUBLIC_BASE_URL"), TableName: os.Getenv("AGENTPAY_TABLE_NAME"),
		EvidenceBucket: os.Getenv("AGENTPAY_EVIDENCE_BUCKET"), EvidenceKMSKeyID: os.Getenv("AGENTPAY_EVIDENCE_KMS_KEY_ID"),
		CapabilitySigningKeyID:      os.Getenv("AGENTPAY_CAPABILITY_SIGNING_KEY_ID"),
		ApplicationSecretsKMSKeyID:  os.Getenv("AGENTPAY_APPLICATION_SECRETS_KMS_KEY_ID"),
		CredentialPepperSecretARN:   os.Getenv("AGENTPAY_CREDENTIAL_PEPPER_SECRET_ARN"),
		ConfirmationPepperSecretARN: os.Getenv("AGENTPAY_CONFIRMATION_GRANT_PEPPER_SECRET_ARN"),
		SellerUserPoolID:            os.Getenv("AGENTPAY_SELLER_USER_POOL_ID"), SellerUserPoolClientID: os.Getenv("AGENTPAY_SELLER_USER_POOL_CLIENT_ID"),
		PaymentMode: os.Getenv("AGENTPAY_PAYMENT_MODE"), FacilitatorURL: os.Getenv("AGENTPAY_FACILITATOR_URL"), X402Network: os.Getenv("AGENTPAY_X402_NETWORK"), X402Asset: os.Getenv("AGENTPAY_X402_ASSET"),
	}
	if config.HTTPAddress == "" {
		config.HTTPAddress = ":8080"
	}
	if config.Environment == "local" {
		if config.APIOrigin == "" {
			config.APIOrigin = "http://localhost:8080"
		}
		if config.PublicBaseURL == "" {
			config.PublicBaseURL = config.APIOrigin
		}
		if config.WebOrigin == "" {
			config.WebOrigin = "http://localhost:3000"
		}
		if err := validatePaymentRuntimeConfig(paymentRuntimeConfig{
			Environment: config.Environment, Mode: config.PaymentMode,
			FacilitatorURL: config.FacilitatorURL, Network: config.X402Network, Asset: config.X402Asset,
		}); err != nil {
			return runtimeConfig{}, err
		}
		return config, nil
	}
	if err := decodeKeyIDs(os.Getenv("AGENTPAY_CAPABILITY_VERIFICATION_KEY_IDS"), &config.CapabilityVerificationKeyIDs); err != nil {
		return runtimeConfig{}, err
	}
	for name, value := range map[string]string{
		"AWS_REGION": config.AWSRegion, "AGENTPAY_WEB_ORIGIN": config.WebOrigin, "AGENTPAY_API_ORIGIN": config.APIOrigin,
		"AGENTPAY_PUBLIC_BASE_URL": config.PublicBaseURL, "AGENTPAY_TABLE_NAME": config.TableName,
		"AGENTPAY_EVIDENCE_BUCKET": config.EvidenceBucket, "AGENTPAY_EVIDENCE_KMS_KEY_ID": config.EvidenceKMSKeyID,
		"AGENTPAY_CAPABILITY_SIGNING_KEY_ID":            config.CapabilitySigningKeyID,
		"AGENTPAY_APPLICATION_SECRETS_KMS_KEY_ID":       config.ApplicationSecretsKMSKeyID,
		"AGENTPAY_CREDENTIAL_PEPPER_SECRET_ARN":         config.CredentialPepperSecretARN,
		"AGENTPAY_CONFIRMATION_GRANT_PEPPER_SECRET_ARN": config.ConfirmationPepperSecretARN,
		"AGENTPAY_SELLER_USER_POOL_ID":                  config.SellerUserPoolID,
		"AGENTPAY_SELLER_USER_POOL_CLIENT_ID":           config.SellerUserPoolClientID,
		"AGENTPAY_PAYMENT_MODE":                         config.PaymentMode,
		"AGENTPAY_FACILITATOR_URL":                      config.FacilitatorURL, "AGENTPAY_X402_NETWORK": config.X402Network,
		"AGENTPAY_X402_ASSET": config.X402Asset,
	} {
		if strings.TrimSpace(value) == "" {
			return runtimeConfig{}, errors.New(name + " is required for the AWS runtime")
		}
	}
	if err := validatePaymentRuntimeConfig(paymentRuntimeConfig{
		Environment: config.Environment, Mode: config.PaymentMode,
		FacilitatorURL: config.FacilitatorURL, Network: config.X402Network, Asset: config.X402Asset,
	}); err != nil {
		return runtimeConfig{}, err
	}
	return config, nil
}

func decodeKeyIDs(encoded string, destination *map[string]string) error {
	if strings.TrimSpace(encoded) == "" || json.Unmarshal([]byte(encoded), destination) != nil || len(*destination) == 0 {
		return errors.New("AGENTPAY_CAPABILITY_VERIFICATION_KEY_IDS must be a non-empty JSON object")
	}
	return nil
}

func newRuntimeDependencies(ctx context.Context, config runtimeConfig, clock domain.Clock) (runtimeDependencies, error) {
	if config.Environment == "local" {
		return newLocalRuntimeDependencies(config, clock)
	}
	return newAWSRuntimeDependencies(ctx, config, clock)
}

func newLocalRuntimeDependencies(config runtimeConfig, clock domain.Clock) (runtimeDependencies, error) {
	catalogRepository := memory.NewCatalogRepository()
	intentRepository := memory.NewPurchaseIntentRepository()
	approvalRepository := memory.NewApprovalRepository()
	transactionRepository := memory.NewTransactionRepository()
	evidenceRepository := memory.NewEvidenceRepository()
	disputeRepository := memory.NewDisputeRepository()
	manualRefundRepository := memory.NewManualRefundRecordRepository()
	browserRepository := memory.NewBrowserPurchaseSessionRepository()
	integrationRepository := memory.NewIntegrationCredentialRepository()
	confirmationRepository := memory.NewConfirmationGrantRepository()
	destinationRepository := memory.NewPaymentDestinationRepository()
	webhookRepository := memory.NewWebhookSubscriptionRepository()
	webhookDeliveryRepository := memory.NewWebhookDeliveryRepository()
	entitlementRepository := memory.NewSellerPlanRepository()
	providerEventRepository := memory.NewProviderEventRepository()
	usageRepository := memory.NewUsageMeterEventRepository()
	quotaRepository := memory.NewQuotaCounterRepository()
	revocationRepository := memory.NewSellerSessionRevocationRepository()
	workspaceRepository := memory.NewSellerWorkspaceRepository()
	publicationRepository := memory.NewStorefrontPublicationRepository()
	auditRepository := memory.NewAuditEventRepository()
	webhookSecrets := memory.NewWebhookSecretStore()
	idempotencyStore := memory.NewIdempotencyStore()
	evidenceSigner, err := evidence.NewLocalHMACSigner(os.Getenv("AGENTPAY_LOCAL_EVIDENCE_KEY_ID"), []byte(os.Getenv("AGENTPAY_LOCAL_EVIDENCE_SIGNING_SECRET")))
	if err != nil {
		return runtimeDependencies{}, err
	}
	capabilityKeys, err := authorization.NewLocalES256KeyRing(clock)
	if err != nil {
		return runtimeDependencies{}, err
	}
	rotationReplayProtector, err := integrations.NewLocalRotationReplayProtector(mustRandomSecret("rotation replay key"))
	if err != nil {
		return runtimeDependencies{}, err
	}
	sellerSigningSecret := []byte(os.Getenv("AGENTPAY_LOCAL_SELLER_SIGNING_SECRET"))
	sellerSigner := proxy.NewHMACSigner(proxy.NewLocalSecretProvider(sellerSigningSecret), clock)
	sellerForwarder, err := newSellerForwarder(config.Environment, os.Getenv("AGENTPAY_SELLER_READINESS_URL"))
	if err != nil {
		return runtimeDependencies{}, err
	}
	healthController, err := newDependencyHealthController(dependencyHealthConfig{
		RepositoryMode: config.RepositoryMode, PaymentReadinessURL: os.Getenv("AGENTPAY_PAYMENT_READINESS_URL"),
		SellerReadinessURL: os.Getenv("AGENTPAY_SELLER_READINESS_URL"), Timeout: defaultDependencyHealthTimeout,
	}, dependencyHealthDependencies{Catalog: catalogRepository, EvidenceSigner: evidenceSigner, SellerSigner: sellerSigner, SellerSigningSecret: sellerSigningSecret})
	if err != nil {
		return runtimeDependencies{}, err
	}
	seed := func(mux *http.ServeMux) error {
		return configureDevelopmentSeed(mux, developmentSeedConfig{
			Environment: config.Environment, RepositoryMode: config.RepositoryMode, HTTPAddress: config.HTTPAddress,
			ProfileName: os.Getenv("AGENTPAY_LOCAL_SEED_PROFILE"), WebhookSigningSecret: os.Getenv("AGENTPAY_LOCAL_WEBHOOK_SIGNING_SECRET"),
		}, developmentSeedRepositories{
			Catalog: catalogRepository, PurchaseIntents: intentRepository, Approvals: approvalRepository,
			Transactions: transactionRepository, Evidence: evidenceRepository, Disputes: disputeRepository,
			ManualRefundRecords: manualRefundRepository, BrowserPurchaseSessions: browserRepository,
			PaymentDestinations: destinationRepository, WebhookSubscriptions: webhookRepository,
			WebhookDeliveries: webhookDeliveryRepository, WebhookSecrets: webhookSecrets,
			IntegrationCredentials: integrationRepository, ConfirmationGrants: confirmationRepository,
			SellerEntitlements: entitlementRepository, ProviderEvents: providerEventRepository,
			AuditEvents: auditRepository, Idempotency: idempotencyStore,
			SellerSessionRevocations: revocationRepository, SellerWorkspaces: workspaceRepository,
			StorefrontPublications: publicationRepository,
		}, evidenceSigner)
	}
	return runtimeDependencies{
		catalog: catalogRepository, intents: intentRepository, transactions: transactionRepository,
		evidence: evidenceRepository, disputes: disputeRepository, manualRefunds: manualRefundRepository,
		browserPurchases: browserRepository, integrationCredentials: integrationRepository,
		confirmationGrants: confirmationRepository, paymentDestinations: destinationRepository,
		webhookSubscriptions: webhookRepository, webhookDeliveries: webhookDeliveryRepository,
		sellerEntitlements: entitlementRepository, providerEvents: providerEventRepository,
		usageMeterEvents: usageRepository, quotaCounters: quotaRepository,
		sellerSessionRevocations: revocationRepository, sellerWorkspaces: workspaceRepository,
		storefrontPublications: publicationRepository, auditEvents: auditRepository, idempotency: idempotencyStore,
		projectKeyExchangeLimiter: memory.NewProjectKeyExchangeRateLimiter(clock, integrations.DefaultProjectKeyExchangeAttempts, integrations.DefaultProjectKeyExchangeWindow),
		credentialPepper:          integrations.StaticCredentialPepperProvider{Value: mustRandomSecret("credential pepper")},
		confirmationGrantPepper:   integrations.StaticCredentialPepperProvider{Value: mustRandomSecret("confirmation grant pepper")},
		rotationReplayProtector:   rotationReplayProtector,
		capabilityKeys:            capabilityKeys, evidenceSigner: evidenceSigner, sellerSigner: sellerSigner,
		webhookSecrets: webhookSecrets, paymentAdapter: localPaymentAdapter(config), sellerForwarder: sellerForwarder,
		healthController:         healthController,
		configureDevelopmentSeed: seed, stripeProviderEventEnabled: os.Getenv("AGENTPAY_STRIPE_WEBHOOK_SECRET") != "",
	}, nil
}

func newAWSRuntimeDependencies(ctx context.Context, config runtimeConfig, clock domain.Clock) (runtimeDependencies, error) {
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(config.AWSRegion))
	if err != nil {
		return runtimeDependencies{}, err
	}
	dynamoClient := awssdkdynamodb.NewFromConfig(awsConfig)
	kmsClient := awskmssdk.NewFromConfig(awsConfig)
	s3Client := awss3sdk.NewFromConfig(awsConfig)
	secretsClient := awssecretssdk.NewFromConfig(awsConfig)
	credentialPepper, err := awssecrets.NewPepperProvider(secretsClient, config.CredentialPepperSecretARN)
	if err != nil {
		return runtimeDependencies{}, err
	}
	confirmationPepper, err := awssecrets.NewPepperProvider(secretsClient, config.ConfirmationPepperSecretARN)
	if err != nil {
		return runtimeDependencies{}, err
	}
	if _, err := credentialPepper.CredentialPepper(ctx); err != nil {
		return runtimeDependencies{}, err
	}
	if _, err := confirmationPepper.CredentialPepper(ctx); err != nil {
		return runtimeDependencies{}, err
	}
	webhookSecrets, err := awssecrets.NewWebhookSecretStore(secretsClient, "agentpay", config.Environment, config.ApplicationSecretsKMSKeyID)
	if err != nil {
		return runtimeDependencies{}, err
	}
	capabilityKeys, err := awskms.NewCapabilityKeyRing(kmsClient, config.CapabilitySigningKeyID, config.CapabilityVerificationKeyIDs)
	if err != nil {
		return runtimeDependencies{}, err
	}
	evidenceSigner, err := awskms.NewEvidenceSigner(kmsClient, config.EvidenceKMSKeyID)
	if err != nil {
		return runtimeDependencies{}, err
	}
	rotationProtector, err := awskms.NewEnvelopeProtector(kmsClient, config.ApplicationSecretsKMSKeyID)
	if err != nil {
		return runtimeDependencies{}, err
	}
	evidenceRepository, err := evidencestore.NewS3Repository(s3Client, config.EvidenceBucket)
	if err != nil {
		return runtimeDependencies{}, err
	}
	facilitator := x402http.NewHTTPFacilitatorClient(&x402http.FacilitatorConfig{URL: config.FacilitatorURL, Timeout: 8 * time.Second})
	catalogRepository := dynamorepository.NewCatalogRepository(dynamoClient, config.TableName)
	healthController, err := newAWSDependencyHealthController(catalogRepository, evidenceRepository, evidenceSigner, capabilityKeys, credentialPepper, confirmationPepper, facilitator)
	if err != nil {
		return runtimeDependencies{}, err
	}
	return runtimeDependencies{
		catalog:                   catalogRepository,
		intents:                   dynamorepository.NewPurchaseIntentRepository(dynamoClient, config.TableName),
		transactions:              dynamorepository.NewTransactionRepository(dynamoClient, config.TableName),
		evidence:                  evidenceRepository,
		disputes:                  dynamorepository.NewDisputeRepository(dynamoClient, config.TableName),
		manualRefunds:             dynamorepository.NewManualRefundRecordRepository(dynamoClient, config.TableName),
		browserPurchases:          dynamorepository.NewBrowserPurchaseSessionRepository(dynamoClient, config.TableName),
		integrationCredentials:    dynamorepository.NewIntegrationCredentialRepository(dynamoClient, config.TableName),
		confirmationGrants:        dynamorepository.NewConfirmationGrantRepository(dynamoClient, config.TableName),
		paymentDestinations:       dynamorepository.NewPaymentDestinationRepository(dynamoClient, config.TableName),
		webhookSubscriptions:      dynamorepository.NewWebhookSubscriptionRepository(dynamoClient, config.TableName),
		webhookDeliveries:         dynamorepository.NewWebhookDeliveryRepository(dynamoClient, config.TableName),
		sellerEntitlements:        dynamorepository.NewSellerPlanRepository(dynamoClient, config.TableName),
		providerEvents:            dynamorepository.NewProviderEventRepository(dynamoClient, config.TableName),
		usageMeterEvents:          dynamorepository.NewUsageMeterEventRepository(dynamoClient, config.TableName),
		quotaCounters:             dynamorepository.NewQuotaCounterRepository(dynamoClient, config.TableName),
		sellerSessionRevocations:  dynamorepository.NewSellerSessionRevocationRepository(dynamoClient, config.TableName),
		sellerWorkspaces:          dynamorepository.NewSellerWorkspaceRepository(dynamoClient, config.TableName),
		storefrontPublications:    dynamorepository.NewStorefrontPublicationRepository(dynamoClient, config.TableName),
		publicationCompletions:    dynamorepository.NewPublicationCompletionStore(dynamoClient, config.TableName),
		auditEvents:               dynamorepository.NewAuditEventRepository(dynamoClient, config.TableName),
		idempotency:               dynamorepository.NewIdempotencyStore(dynamoClient, config.TableName),
		projectKeyExchangeLimiter: dynamorepository.NewProjectKeyExchangeRateLimiter(dynamoClient, config.TableName, clock, integrations.DefaultProjectKeyExchangeAttempts, integrations.DefaultProjectKeyExchangeWindow),
		credentialPepper:          credentialPepper, confirmationGrantPepper: confirmationPepper,
		rotationReplayProtector: rotationProtector, capabilityKeys: capabilityKeys,
		evidenceSigner: evidenceSigner, sellerSigner: proxy.NewHMACSigner(webhookSecrets, clock), webhookSecrets: webhookSecrets,
		paymentAdapter:   payments.NewX402AdapterWithFacilitator(facilitator, 8*time.Second),
		sellerForwarder:  proxy.NewForwarder(nil),
		healthController: healthController,
	}, nil
}

func localPaymentAdapter(config runtimeConfig) payments.Adapter {
	if config.PaymentMode == paymentModeMock {
		return payments.NewMockAdapter()
	}
	return payments.NewX402AdapterForFacilitator(config.FacilitatorURL)
}

func newAWSDependencyHealthController(catalogRepository catalog.Repository, evidenceRepository evidence.Repository, evidenceSigner evidence.Signer, capabilityKeys capabilityKeyRing, credentialPepper, confirmationPepper integrations.CredentialPepperProvider, facilitator x402.FacilitatorClient) (*health.Controller, error) {
	sellerID, err := domain.ParseID(readinessSellerID, domain.SellerIDPrefix)
	if err != nil {
		return nil, err
	}
	transactionID, err := domain.ParseID(readinessTransactionID, domain.TransactionIDPrefix)
	if err != nil {
		return nil, err
	}
	return health.NewController([]health.Dependency{
		{Name: "dynamodb", Check: func(ctx context.Context) error {
			_, err := catalogRepository.ListRoutesBySeller(ctx, sellerID)
			return err
		}},
		{Name: "evidence_store", Check: func(ctx context.Context) error {
			_, err := evidenceRepository.ListByTransaction(ctx, transactionID)
			return err
		}},
		{Name: "kms", Check: func(ctx context.Context) error {
			digest := sha256.Sum256([]byte("agentpay.readiness.v1"))
			signature, err := evidenceSigner.Sign(ctx, digest[:])
			if err != nil {
				return err
			}
			valid, err := evidenceSigner.Verify(ctx, signature.KeyID, digest[:], signature.Value)
			if err != nil || !valid {
				return errors.New("KMS evidence signature verification failed")
			}
			keyID, err := capabilityKeys.CurrentKeyID(ctx)
			if err != nil {
				return err
			}
			input := []byte("agentpay.capability.readiness.v1")
			rawSignature, err := capabilityKeys.Sign(ctx, keyID, input)
			if err != nil {
				return err
			}
			publicKey, err := capabilityKeys.VerificationKey(ctx, keyID)
			if err != nil || !verifyCapabilitySignature(publicKey, input, rawSignature) {
				return errors.New("KMS capability signature verification failed")
			}
			return nil
		}},
		{Name: "secrets", Check: func(ctx context.Context) error {
			if _, err := credentialPepper.CredentialPepper(ctx); err != nil {
				return err
			}
			_, err := confirmationPepper.CredentialPepper(ctx)
			return err
		}},
		{Name: "x402_facilitator", Check: func(ctx context.Context) error { _, err := facilitator.GetSupported(ctx); return err }},
	}, awsDependencyHealthTimeout)
}

func verifyCapabilitySignature(publicKey *ecdsa.PublicKey, message, signature []byte) bool {
	if publicKey == nil || len(signature) != 64 {
		return false
	}
	digest := sha256.Sum256(message)
	return ecdsa.Verify(publicKey, digest[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:]))
}
