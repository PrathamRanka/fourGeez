//go:build agentpay_dev

// Package devseed owns deterministic, process-local launch fixtures.
package devseed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const (
	// ProfileLaunchReady identifies the deterministic LCH-008 fixture set.
	ProfileLaunchReady = "launch-ready"
	// InfoPath returns non-secret identifiers for the active local profile.
	InfoPath = "/__dev/seed-profile"
	// ResetPath clears disposable state and reapplies the active profile.
	ResetPath = "/__dev/seed-profile/reset"

	launchReadySellerIDValue      = "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	incompleteSellerIDValue       = "sel_01K5D09YJ0C0M7RJM4FWQ0K9H8"
	belowThresholdRouteIDValue    = "rte_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	approvalRequiredRouteIDValue  = "rte_01K5D09YJ0C0M7RJM4FWQ0K9H8"
	paymentPendingTransactionID   = "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	fulfilledTransactionID        = "txn_01K5D09YJ0C0M7RJM4FWQ0K9H8"
	failedTransactionID           = "txn_01K5D09YJ0C0M7RJM4FWQ0K9H9"
	disputedTransactionID         = "txn_01K5D09YJ0C0M7RJM4FWQ0K9HA"
	localSellerSigningReference   = "local/demo-seller-signing"
	localWebhookSigningReference  = "local/demo-webhook-signing"
	localWebhookMinimumSecretSize = 32
)

var fixedSeedTimestamp = domain.NewTimestamp(
	time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
)

var (
	// ErrDevelopmentOnly prevents a development profile from crossing environments.
	ErrDevelopmentOnly = errors.New("development seed profiles require a local in-memory runtime")
	// ErrUnknownProfile rejects unversioned or misspelled fixture names.
	ErrUnknownProfile = errors.New("unknown development seed profile")
)

// Config contains the runtime gates and local-only secret needed by the profile.
type Config struct {
	Environment          string
	RepositoryMode       string
	HTTPAddress          string
	ProfileName          string
	WebhookSigningSecret string
}

// TransactionRepository persists and lists seeded transactions.
type TransactionRepository interface {
	Create(context.Context, transactions.Transaction) error
	Get(context.Context, domain.ID) (transactions.Transaction, error)
	ListBySeller(context.Context, domain.ID, int, string) ([]transactions.Transaction, *string, error)
}

// Repositories are the existing process-local persistence boundaries used by the API.
type Repositories struct {
	Catalog              catalog.Repository
	PurchaseIntents      intents.Repository
	Transactions         TransactionRepository
	Evidence             evidence.Repository
	Disputes             disputes.Repository
	PaymentDestinations  settlement.Repository
	WebhookSubscriptions notifications.Repository
	WebhookDeliveries    notifications.DeliveryRepository
	WebhookSecrets       notifications.SecretStore
	SellerEntitlements   billing.Repository
	Reset                func(context.Context) error
}

// Metadata contains stable non-secret identifiers used by the local web runtime.
type Metadata struct {
	ProfileName                  string    `json:"profileName"`
	LaunchReadySellerID          domain.ID `json:"launchReadySellerId"`
	IncompleteSellerID           domain.ID `json:"incompleteSellerId"`
	BelowThresholdRouteID        domain.ID `json:"belowThresholdRouteId"`
	ApprovalRequiredRouteID      domain.ID `json:"approvalRequiredRouteId"`
	PaymentPendingTransactionID  domain.ID `json:"paymentPendingTransactionId"`
	FulfilledTransactionID       domain.ID `json:"fulfilledTransactionId"`
	FailedTransactionID          domain.ID `json:"failedTransactionId"`
	DisputedTransactionID        domain.ID `json:"disputedTransactionId"`
	ValidEvidenceTransactionID   domain.ID `json:"validEvidenceTransactionId"`
	InvalidEvidenceTransactionID domain.ID `json:"invalidEvidenceTransactionId"`
}

// Seeder owns one named fixture set and serializes reset operations.
type Seeder struct {
	config       Config
	repositories Repositories
	signer       evidence.Signer
	metadata     Metadata
	mutex        sync.Mutex
}

// New validates the development-only boundary before returning a seeder.
func New(config Config, repositories Repositories, signer evidence.Signer) (*Seeder, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	if err := validateRepositories(repositories, signer); err != nil {
		return nil, err
	}
	metadata, err := newMetadata()
	if err != nil {
		return nil, err
	}
	return &Seeder{
		config:       config,
		repositories: repositories,
		signer:       signer,
		metadata:     metadata,
	}, nil
}

// Metadata returns stable identifiers without exposing local credentials.
func (seeder *Seeder) Metadata() Metadata {
	return seeder.metadata
}

// ResetAndSeed atomically serializes local reset requests and reapplies the profile.
func (seeder *Seeder) ResetAndSeed(ctx context.Context) (Metadata, error) {
	seeder.mutex.Lock()
	defer seeder.mutex.Unlock()

	if err := seeder.repositories.Reset(ctx); err != nil {
		return Metadata{}, fmt.Errorf("reset development repositories: %w", err)
	}
	if err := seeder.seed(ctx); err != nil {
		_ = seeder.repositories.Reset(ctx)
		return Metadata{}, err
	}
	return seeder.metadata, nil
}

// RegisterRoutes exposes local-only profile inspection and reset endpoints.
func (seeder *Seeder) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+InfoPath, func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusOK, seeder.Metadata())
	})
	mux.HandleFunc("POST "+ResetPath, func(response http.ResponseWriter, request *http.Request) {
		metadata, err := seeder.ResetAndSeed(request.Context())
		if err != nil {
			writeJSON(response, http.StatusInternalServerError, map[string]string{
				"error": "development seed reset failed",
			})
			return
		}
		writeJSON(response, http.StatusOK, metadata)
	})
}

func (seeder *Seeder) seed(ctx context.Context) error {
	launchReadySeller, err := seeder.seedSellers(ctx)
	if err != nil {
		return err
	}
	if err := seeder.seedEntitlements(ctx); err != nil {
		return err
	}
	if err := seeder.seedPaymentDestinations(ctx, launchReadySeller); err != nil {
		return err
	}
	routes, err := seeder.seedRoutes(ctx, launchReadySeller)
	if err != nil {
		return err
	}
	transactionFixtures, err := seeder.seedTransactions(ctx, launchReadySeller, routes)
	if err != nil {
		return err
	}
	if err := seeder.seedEvidence(ctx, transactionFixtures); err != nil {
		return err
	}
	if err := seeder.seedDispute(ctx, transactionFixtures.disputed); err != nil {
		return err
	}
	if err := seeder.seedWebhookHistory(ctx, launchReadySeller, transactionFixtures); err != nil {
		return err
	}
	return nil
}

func (seeder *Seeder) seedEntitlements(ctx context.Context) error {
	periodStart := domain.NewTimestamp(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC))
	periodEnd := domain.NewTimestamp(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC))
	candidates := []billing.EntitlementCandidate{
		{
			SellerID: seeder.metadata.LaunchReadySellerID, PlanID: billing.PlanGrowth, PlanVersion: 1,
			Status: billing.EntitlementStatusActive, BillingPeriodStart: periodStart, BillingPeriodEnd: periodEnd, AccessEndsAt: periodEnd,
			Source: billing.EntitlementSourceLocal, Provider: billing.EntitlementProviderLocal,
		},
		{
			SellerID: seeder.metadata.IncompleteSellerID, PlanID: billing.PlanStarter, PlanVersion: 1,
			Status: billing.EntitlementStatusSuspended, BillingPeriodStart: periodStart, BillingPeriodEnd: periodEnd, AccessEndsAt: fixedSeedTimestamp,
			Source: billing.EntitlementSourceLocal, Provider: billing.EntitlementProviderLocal,
			StatusReason: billing.EntitlementStatusReasonProviderIncomplete,
		},
	}
	for _, candidate := range candidates {
		entitlement, reconciliation, err := billing.ReconcileSellerEntitlement(nil, candidate, fixedSeedTimestamp)
		if err != nil {
			return err
		}
		if err := seeder.repositories.SellerEntitlements.Apply(ctx, entitlement, reconciliation, 0); err != nil {
			return err
		}
	}
	return nil
}

func (seeder *Seeder) seedSellers(ctx context.Context) (catalog.Seller, error) {
	launchReadySeller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID:        seeder.metadata.LaunchReadySellerID,
		OwnerSubject:    "local-seller",
		Slug:            "demo-seller",
		Name:            "Northstar Research",
		UpstreamBaseURL: "http://127.0.0.1:8090",
		CreatedAt:       fixedSeedTimestamp,
	})
	if err != nil {
		return catalog.Seller{}, err
	}
	if err := launchReadySeller.Activate(
		localSellerSigningReference,
		fixedSeedTimestamp.Add(time.Minute),
	); err != nil {
		return catalog.Seller{}, err
	}
	incompleteSeller, err := catalog.NewSeller(catalog.SellerParams{
		SellerID:        seeder.metadata.IncompleteSellerID,
		OwnerSubject:    "local-incomplete-owner",
		Slug:            "setup-incomplete",
		Name:            "Setup Incomplete",
		UpstreamBaseURL: "https://incomplete.example",
		CreatedAt:       fixedSeedTimestamp,
	})
	if err != nil {
		return catalog.Seller{}, err
	}
	if err := seeder.repositories.Catalog.CreateSeller(ctx, launchReadySeller); err != nil {
		return catalog.Seller{}, err
	}
	if err := seeder.repositories.Catalog.CreateSeller(ctx, incompleteSeller); err != nil {
		return catalog.Seller{}, err
	}
	return launchReadySeller, nil
}

func (seeder *Seeder) seedPaymentDestinations(ctx context.Context, seller catalog.Seller) error {
	destinations := []settlement.PaymentDestinationSnapshot{
		{
			DestinationID: mustProfileID("dst_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.PaymentDestinationIDPrefix),
			SellerID:      seller.SellerID,
			Asset:         "USDC",
			Network:       "eip155:84532",
			Address:       "0x1111111111111111111111111111111111111111",
		},
		{
			DestinationID: mustProfileID("dst_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.PaymentDestinationIDPrefix),
			SellerID:      seller.SellerID,
			Asset:         "EURC",
			Network:       "eip155:84532",
			Address:       "0x2222222222222222222222222222222222222222",
		},
	}
	verifiedAt := fixedSeedTimestamp.Add(2 * time.Minute)
	for _, snapshot := range destinations {
		snapshot.Status = settlement.PaymentDestinationStatusActive
		snapshot.VerifiedAt = &verifiedAt
		snapshot.CreatedAt = fixedSeedTimestamp
		snapshot.UpdatedAt = verifiedAt
		snapshot.Version = 2
		destination, err := settlement.RestorePaymentDestination(snapshot)
		if err != nil {
			return err
		}
		if err := seeder.repositories.PaymentDestinations.Create(ctx, destination); err != nil {
			return err
		}
	}
	return nil
}

type routeFixtures struct {
	belowThreshold   catalog.PaidRoute
	approvalRequired catalog.PaidRoute
}

func (seeder *Seeder) seedRoutes(ctx context.Context, seller catalog.Seller) (routeFixtures, error) {
	approvalThreshold := domain.MustParseAmount("10000000")
	belowThreshold, err := catalog.NewPaidRoute(catalog.PaidRouteParams{
		RouteID:                 seeder.metadata.BelowThresholdRouteID,
		SellerID:                seller.SellerID,
		DisplayName:             "Market Snapshot",
		ProductSlug:             "market-snapshot",
		Method:                  catalog.RouteMethodPost,
		PathPattern:             "/research/basic",
		Description:             "A concise, source-ready market snapshot.",
		MIMEType:                "application/json",
		Amount:                  domain.MustParseAmount("2500000"),
		Asset:                   "USDC",
		Network:                 "eip155:84532",
		PayTo:                   "0x1111111111111111111111111111111111111111",
		ApprovalThresholdAmount: &approvalThreshold,
		UpstreamTimeoutSeconds:  20,
		CreatedAt:               fixedSeedTimestamp.Add(3 * time.Minute),
	})
	if err != nil {
		return routeFixtures{}, err
	}
	approvalRequired, err := catalog.NewPaidRoute(catalog.PaidRouteParams{
		RouteID:                 seeder.metadata.ApprovalRequiredRouteID,
		SellerID:                seller.SellerID,
		DisplayName:             "Board Research Brief",
		ProductSlug:             "board-research-brief",
		Method:                  catalog.RouteMethodPost,
		PathPattern:             "/research/board",
		Description:             "A detailed board-ready research brief.",
		MIMEType:                "application/json",
		Amount:                  domain.MustParseAmount("25000000"),
		Asset:                   "EURC",
		Network:                 "eip155:84532",
		PayTo:                   "0x2222222222222222222222222222222222222222",
		ApprovalThresholdAmount: &approvalThreshold,
		UpstreamTimeoutSeconds:  20,
		CreatedAt:               fixedSeedTimestamp.Add(4 * time.Minute),
	})
	if err != nil {
		return routeFixtures{}, err
	}
	for _, route := range []catalog.PaidRoute{belowThreshold, approvalRequired} {
		if err := seeder.repositories.Catalog.CreateRoute(ctx, route); err != nil {
			return routeFixtures{}, err
		}
	}
	return routeFixtures{belowThreshold: belowThreshold, approvalRequired: approvalRequired}, nil
}

type transactionFixtures struct {
	paymentPending transactions.Transaction
	fulfilled      transactions.Transaction
	failed         transactions.Transaction
	disputed       transactions.Transaction
}

func (seeder *Seeder) seedTransactions(
	ctx context.Context,
	seller catalog.Seller,
	routes routeFixtures,
) (transactionFixtures, error) {
	specifications := []struct {
		intentID      string
		transactionID string
		buyerID       string
		route         catalog.PaidRoute
		advance       func(*transactions.Transaction, domain.Timestamp) error
	}{
		{
			intentID: "int_01K5D09YJ0C0M7RJM4FWQ0K9H7", transactionID: paymentPendingTransactionID,
			buyerID: "buyer-local-pending", route: routes.belowThreshold, advance: advancePaymentPending,
		},
		{
			intentID: "int_01K5D09YJ0C0M7RJM4FWQ0K9H8", transactionID: fulfilledTransactionID,
			buyerID: "buyer-local-fulfilled", route: routes.belowThreshold, advance: advanceFulfilled,
		},
		{
			intentID: "int_01K5D09YJ0C0M7RJM4FWQ0K9H9", transactionID: failedTransactionID,
			buyerID: "buyer-local-failed", route: routes.approvalRequired, advance: advanceFailed,
		},
		{
			intentID: "int_01K5D09YJ0C0M7RJM4FWQ0K9HA", transactionID: disputedTransactionID,
			buyerID: "buyer-local-disputed", route: routes.approvalRequired, advance: advanceDisputed,
		},
	}
	created := make([]transactions.Transaction, 0, len(specifications))
	for index, specification := range specifications {
		createdAt := fixedSeedTimestamp.Add(time.Duration(10+index*10) * time.Minute)
		intentID := mustProfileID(specification.intentID, domain.IntentIDPrefix)
		purchaseIntent, err := intents.NewPurchaseIntent(intents.PurchaseIntentParams{
			IntentID:         intentID,
			SellerID:         seller.SellerID,
			RouteID:          specification.route.RouteID,
			BuyerID:          specification.buyerID,
			RequestMethod:    intents.RequestMethodPost,
			RequestPath:      specification.route.PathPattern,
			RequestBodyHash:  mustDigest(strings.Repeat("0", 64)),
			Amount:           specification.route.Amount,
			Asset:            specification.route.Asset,
			Network:          specification.route.Network,
			MaximumAmount:    specification.route.Amount,
			RequiresApproval: specification.route.RouteID == routes.approvalRequired.RouteID,
			CreatedAt:        createdAt,
			ExpiresAt:        createdAt.Add(15 * time.Minute),
		})
		if err != nil {
			return transactionFixtures{}, err
		}
		if err := seeder.repositories.PurchaseIntents.Create(ctx, purchaseIntent); err != nil {
			return transactionFixtures{}, err
		}
		transaction, err := transactions.NewTransaction(transactions.TransactionParams{
			TransactionID: mustProfileID(specification.transactionID, domain.TransactionIDPrefix),
			IntentID:      intentID,
			SellerID:      seller.SellerID,
			RouteID:       specification.route.RouteID,
			BuyerID:       specification.buyerID,
			Amount:        specification.route.Amount,
			Asset:         specification.route.Asset,
			Network:       specification.route.Network,
			CreatedAt:     createdAt,
		})
		if err != nil {
			return transactionFixtures{}, err
		}
		if err := specification.advance(&transaction, createdAt); err != nil {
			return transactionFixtures{}, err
		}
		if err := seeder.repositories.Transactions.Create(ctx, transaction); err != nil {
			return transactionFixtures{}, err
		}
		created = append(created, transaction)
	}
	return transactionFixtures{
		paymentPending: created[0],
		fulfilled:      created[1],
		failed:         created[2],
		disputed:       created[3],
	}, nil
}

func advancePaymentPending(transaction *transactions.Transaction, createdAt domain.Timestamp) error {
	return transaction.RequirePayment(createdAt.Add(time.Minute))
}

func advanceFulfilled(transaction *transactions.Transaction, createdAt domain.Timestamp) error {
	if err := transaction.RequirePayment(createdAt.Add(time.Minute)); err != nil {
		return err
	}
	if err := verifyAndFinalize(transaction, "fulfilled", createdAt.Add(2*time.Minute)); err != nil {
		return err
	}
	if err := transaction.MarkForwarded(createdAt.Add(4 * time.Minute)); err != nil {
		return err
	}
	return transaction.MarkFulfilled(
		http.StatusOK,
		mustDigest(strings.Repeat("a", 64)),
		transactions.ResponseSummary{ContentType: "application/json", ContentLength: 128},
		createdAt.Add(5*time.Minute),
	)
}

func advanceFailed(transaction *transactions.Transaction, createdAt domain.Timestamp) error {
	if err := transaction.RequireApproval(createdAt.Add(time.Minute)); err != nil {
		return err
	}
	if err := transaction.MarkApproved(createdAt.Add(2 * time.Minute)); err != nil {
		return err
	}
	if err := transaction.RequirePayment(createdAt.Add(3 * time.Minute)); err != nil {
		return err
	}
	if err := verifyAndFinalize(transaction, "failed", createdAt.Add(4*time.Minute)); err != nil {
		return err
	}
	if err := transaction.MarkForwarded(createdAt.Add(6 * time.Minute)); err != nil {
		return err
	}
	status := http.StatusServiceUnavailable
	return transaction.MarkFailed(
		"seller_timeout",
		&status,
		nil,
		createdAt.Add(7*time.Minute),
	)
}

func advanceDisputed(transaction *transactions.Transaction, createdAt domain.Timestamp) error {
	if err := transaction.RequireApproval(createdAt.Add(time.Minute)); err != nil {
		return err
	}
	if err := transaction.MarkApproved(createdAt.Add(2 * time.Minute)); err != nil {
		return err
	}
	if err := transaction.RequirePayment(createdAt.Add(3 * time.Minute)); err != nil {
		return err
	}
	if err := verifyAndFinalize(transaction, "disputed", createdAt.Add(4*time.Minute)); err != nil {
		return err
	}
	if err := transaction.MarkForwarded(createdAt.Add(6 * time.Minute)); err != nil {
		return err
	}
	if err := transaction.MarkFulfilled(
		http.StatusOK,
		mustDigest(strings.Repeat("b", 64)),
		transactions.ResponseSummary{ContentType: "application/json", ContentLength: 512},
		createdAt.Add(7*time.Minute),
	); err != nil {
		return err
	}
	return transaction.OpenDispute(createdAt.Add(8 * time.Minute))
}

func verifyAndFinalize(
	transaction *transactions.Transaction,
	suffix string,
	verifiedAt domain.Timestamp,
) error {
	paymentIdentifier := "mock_payment_" + suffix
	if err := transaction.VerifyPayment(
		paymentIdentifier,
		mustDigest(strings.Repeat("c", 64)),
		verifiedAt,
	); err != nil {
		return err
	}
	return transaction.FinalizePayment(
		paymentIdentifier,
		"mock_settlement_"+suffix,
		verifiedAt.Add(time.Minute),
	)
}

func (seeder *Seeder) seedEvidence(ctx context.Context, fixtures transactionFixtures) error {
	validEvent, err := evidence.Append(ctx, evidence.EventParams{
		EventID:       mustProfileID("evt_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.EvidenceIDPrefix),
		TransactionID: fixtures.fulfilled.TransactionID(),
		Sequence:      1,
		EventType:     evidence.EventDeliverySucceeded,
		ActorType:     evidence.ActorSystem,
		Payload: map[string]any{
			"statusCode":   http.StatusOK,
			"responseHash": strings.Repeat("a", 64),
		},
		CreatedAt: fixtures.fulfilled.UpdatedAt(),
	}, nil, seeder.signer)
	if err != nil {
		return err
	}
	if err := seeder.repositories.Evidence.Append(ctx, validEvent); err != nil {
		return err
	}

	invalidEvent, err := evidence.Append(ctx, evidence.EventParams{
		EventID:       mustProfileID("evt_01K5D09YJ0C0M7RJM4FWQ0K9H8", domain.EvidenceIDPrefix),
		TransactionID: fixtures.failed.TransactionID(),
		Sequence:      1,
		EventType:     evidence.EventDeliveryFailed,
		ActorType:     evidence.ActorSystem,
		Payload: map[string]any{
			"failureCode": "seller_timeout",
			"statusCode":  http.StatusServiceUnavailable,
		},
		CreatedAt: fixtures.failed.UpdatedAt(),
	}, nil, seeder.signer)
	if err != nil {
		return err
	}
	invalidEvent.KMSSignature = "invalid-local-signature"
	return seeder.repositories.Evidence.Append(ctx, invalidEvent)
}

func (seeder *Seeder) seedDispute(ctx context.Context, transaction transactions.Transaction) error {
	dispute, err := disputes.Classify(disputes.Params{
		DisputeID:     mustProfileID("dsp_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.DisputeIDPrefix),
		TransactionID: transaction.TransactionID(),
		Reason:        disputes.ReasonQualityOrOutput,
		Statement:     "The delivered report needs seller review.",
		CreatedAt:     transaction.UpdatedAt(),
	}, disputes.Facts{})
	if err != nil {
		return err
	}
	return seeder.repositories.Disputes.Create(ctx, dispute)
}

func (seeder *Seeder) seedWebhookHistory(
	ctx context.Context,
	seller catalog.Seller,
	fixtures transactionFixtures,
) error {
	subscriptionID := mustProfileID(
		"whk_01K5D09YJ0C0M7RJM4FWQ0K9H7",
		domain.WebhookSubscriptionIDPrefix,
	)
	subscription, err := notifications.NewSubscription(notifications.SubscriptionParams{
		SubscriptionID: subscriptionID,
		SellerID:       seller.SellerID,
		EndpointURL:    "https://hooks.example.com/agentpay",
		EventTypes: []notifications.EventType{
			notifications.EventFulfillmentSucceeded,
			notifications.EventFulfillmentFailed,
			notifications.EventDisputeChanged,
		},
		SecretRef: localWebhookSigningReference,
		CreatedAt: fixedSeedTimestamp.Add(70 * time.Minute),
	})
	if err != nil {
		return err
	}
	if err := seeder.repositories.WebhookSubscriptions.Create(ctx, subscription); err != nil {
		return err
	}
	if err := seeder.repositories.WebhookSecrets.PutSecret(
		ctx,
		localWebhookSigningReference,
		[]byte(seeder.config.WebhookSigningSecret),
	); err != nil {
		return err
	}

	deliverySpecifications := []struct {
		deliveryID  string
		eventID     string
		eventType   notifications.EventType
		transaction transactions.Transaction
		advance     func(*notifications.Delivery) error
	}{
		{
			deliveryID: "whd_01K5D09YJ0C0M7RJM4FWQ0K9H7", eventID: "evt_01K5D09YJ0C0M7RJM4FWQ0K9H9",
			eventType: notifications.EventFulfillmentSucceeded, transaction: fixtures.fulfilled,
			advance: func(delivery *notifications.Delivery) error {
				return delivery.RecordSuccess(fixedSeedTimestamp.Add(72*time.Minute), http.StatusNoContent, strings.Repeat("d", 64))
			},
		},
		{
			deliveryID: "whd_01K5D09YJ0C0M7RJM4FWQ0K9H8", eventID: "evt_01K5D09YJ0C0M7RJM4FWQ0K9HA",
			eventType: notifications.EventFulfillmentFailed, transaction: fixtures.failed,
			advance: func(delivery *notifications.Delivery) error {
				return delivery.RecordFailure(fixedSeedTimestamp.Add(73*time.Minute), notifications.DeliveryFailureTimeout, 0, "")
			},
		},
		{
			deliveryID: "whd_01K5D09YJ0C0M7RJM4FWQ0K9H9", eventID: "evt_01K5D09YJ0C0M7RJM4FWQ0K9HB",
			eventType: notifications.EventDisputeChanged, transaction: fixtures.disputed,
			advance: advanceDeadLetter,
		},
	}
	for _, specification := range deliverySpecifications {
		event := notifications.WebhookEvent{
			SchemaVersion: "1",
			EventID:       mustProfileID(specification.eventID, domain.EvidenceIDPrefix),
			SellerID:      seller.SellerID,
			EventType:     specification.eventType,
			OccurredAt:    fixedSeedTimestamp.Add(71 * time.Minute),
			Payload: map[string]any{
				"transactionId": specification.transaction.TransactionID().String(),
				"status":        specification.transaction.Status(),
			},
		}
		body, err := notifications.CanonicalEventBody(event)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(body)
		delivery, err := notifications.NewDelivery(notifications.DeliveryParams{
			DeliveryID:     mustProfileID(specification.deliveryID, domain.WebhookDeliveryIDPrefix),
			SellerID:       seller.SellerID,
			SubscriptionID: subscriptionID,
			Event:          event,
			PayloadHash:    hex.EncodeToString(digest[:]),
			CreatedAt:      fixedSeedTimestamp.Add(71 * time.Minute),
		})
		if err != nil {
			return err
		}
		if err := specification.advance(&delivery); err != nil {
			return err
		}
		if _, inserted, err := seeder.repositories.WebhookDeliveries.CreateIfAbsent(ctx, delivery); err != nil {
			return err
		} else if !inserted {
			return errors.New("seed webhook delivery was not inserted")
		}
	}
	return nil
}

func advanceDeadLetter(delivery *notifications.Delivery) error {
	for delivery.Status() != notifications.DeliveryStatusDeadLetter {
		nextAttemptAt := delivery.NextAttemptAt()
		if nextAttemptAt == nil {
			return errors.New("retrying webhook has no next attempt")
		}
		if err := delivery.RecordFailure(
			*nextAttemptAt,
			notifications.DeliveryFailureRetryableResponse,
			http.StatusServiceUnavailable,
			strings.Repeat("e", 64),
		); err != nil {
			return err
		}
	}
	return nil
}

func newMetadata() (Metadata, error) {
	return Metadata{
		ProfileName:                  ProfileLaunchReady,
		LaunchReadySellerID:          mustProfileID(launchReadySellerIDValue, domain.SellerIDPrefix),
		IncompleteSellerID:           mustProfileID(incompleteSellerIDValue, domain.SellerIDPrefix),
		BelowThresholdRouteID:        mustProfileID(belowThresholdRouteIDValue, domain.RouteIDPrefix),
		ApprovalRequiredRouteID:      mustProfileID(approvalRequiredRouteIDValue, domain.RouteIDPrefix),
		PaymentPendingTransactionID:  mustProfileID(paymentPendingTransactionID, domain.TransactionIDPrefix),
		FulfilledTransactionID:       mustProfileID(fulfilledTransactionID, domain.TransactionIDPrefix),
		FailedTransactionID:          mustProfileID(failedTransactionID, domain.TransactionIDPrefix),
		DisputedTransactionID:        mustProfileID(disputedTransactionID, domain.TransactionIDPrefix),
		ValidEvidenceTransactionID:   mustProfileID(fulfilledTransactionID, domain.TransactionIDPrefix),
		InvalidEvidenceTransactionID: mustProfileID(failedTransactionID, domain.TransactionIDPrefix),
	}, nil
}

func validateConfig(config Config) error {
	if config.ProfileName != ProfileLaunchReady {
		return fmt.Errorf("%w: %q", ErrUnknownProfile, config.ProfileName)
	}
	if config.Environment != "local" || config.RepositoryMode != "memory" || !loopbackAddress(config.HTTPAddress) {
		return ErrDevelopmentOnly
	}
	if len(config.WebhookSigningSecret) < localWebhookMinimumSecretSize {
		return errors.New("local webhook signing secret must contain at least 32 bytes")
	}
	return nil
}

func validateRepositories(repositories Repositories, signer evidence.Signer) error {
	if repositories.Catalog == nil ||
		repositories.PurchaseIntents == nil ||
		repositories.Transactions == nil ||
		repositories.Evidence == nil ||
		repositories.Disputes == nil ||
		repositories.PaymentDestinations == nil ||
		repositories.WebhookSubscriptions == nil ||
		repositories.WebhookDeliveries == nil ||
		repositories.WebhookSecrets == nil ||
		repositories.SellerEntitlements == nil ||
		repositories.Reset == nil ||
		signer == nil {
		return errors.New("development seed repositories and signer are required")
	}
	return nil
}

func loopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	parsed := net.ParseIP(host)
	return parsed != nil && parsed.IsLoopback()
}

func mustProfileID(raw string, prefix domain.IDPrefix) domain.ID {
	identifier, err := domain.ParseID(raw, prefix)
	if err != nil {
		panic(err)
	}
	return identifier
}

func mustDigest(raw string) intents.SHA256Digest {
	digest, err := intents.ParseSHA256Digest(raw)
	if err != nil {
		panic(err)
	}
	return digest
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
