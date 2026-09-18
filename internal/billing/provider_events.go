package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/stripe/stripe-go/v86/webhook"
)

const maximumEntitlementReconcileAttempts = 3

var (
	ErrProviderEventNotFound            = errors.New("subscription provider event was not found")
	ErrProviderEventPayloadConflict     = errors.New("subscription provider event payload conflict")
	ErrProviderEventEnvironmentMismatch = errors.New("subscription provider event environment mismatch")
	ErrProviderEventQuarantined         = errors.New("subscription provider event is quarantined")
	ErrProviderEventAlreadyProcessing   = errors.New("subscription provider event is already processing")
	ErrStripeSignatureInvalid           = errors.New("Stripe webhook signature is invalid")
)

type ProviderEventProcessingState string

const (
	ProviderEventStateReceived    ProviderEventProcessingState = "received"
	ProviderEventStateProcessing  ProviderEventProcessingState = "processing"
	ProviderEventStateApplied     ProviderEventProcessingState = "applied"
	ProviderEventStateFailed      ProviderEventProcessingState = "failed"
	ProviderEventStateQuarantined ProviderEventProcessingState = "quarantined"
)

type ProviderEventStoreResult uint8

const (
	ProviderEventStoreResultInserted ProviderEventStoreResult = iota + 1
	ProviderEventStoreResultDuplicate
	ProviderEventStoreResultConflict
)

type VerifiedStripeEvent struct {
	EventID           string
	EventType         string
	Livemode          bool
	AccountID         string
	APIVersion        string
	ProviderCreatedAt domain.Timestamp
	CustomerID        string
	SubscriptionID    string
	InvoiceID         string
}

type SubscriptionProviderEvent struct {
	EventID                   string                       `json:"eventId"`
	Provider                  EntitlementProvider          `json:"provider"`
	EventType                 string                       `json:"eventType"`
	PayloadHash               string                       `json:"payloadHash"`
	ConflictPayloadHash       string                       `json:"conflictPayloadHash,omitempty"`
	Livemode                  bool                         `json:"livemode"`
	AccountID                 string                       `json:"accountId"`
	APIVersion                string                       `json:"apiVersion"`
	ProviderCreatedAt         domain.Timestamp             `json:"providerCreatedAt"`
	ReceivedAt                domain.Timestamp             `json:"receivedAt"`
	CustomerID                string                       `json:"customerId,omitempty"`
	SubscriptionID            string                       `json:"subscriptionId,omitempty"`
	InvoiceID                 string                       `json:"invoiceId,omitempty"`
	SellerID                  domain.ID                    `json:"sellerId,omitempty"`
	ProcessingState           ProviderEventProcessingState `json:"processingState"`
	AttemptCount              uint64                       `json:"attemptCount"`
	AppliedEntitlementVersion uint64                       `json:"appliedEntitlementVersion,omitempty"`
	ProcessedAt               *domain.Timestamp            `json:"processedAt"`
}

type ProviderEventRepository interface {
	Store(context.Context, SubscriptionProviderEvent) (ProviderEventStoreResult, error)
	Get(context.Context, string) (SubscriptionProviderEvent, error)
	MarkProcessing(context.Context, string, domain.Timestamp) error
	MarkFailed(context.Context, string, domain.Timestamp) error
	MarkApplied(context.Context, string, domain.ID, uint64, domain.Timestamp) error
}

type StripeEventVerifier interface {
	Verify([]byte, string) (VerifiedStripeEvent, error)
}

type StripeProviderStateReader interface {
	CurrentEntitlement(context.Context, SubscriptionProviderEvent) (EntitlementCandidate, error)
}

type StripeProviderEventConfig struct {
	ExpectedLivemode   bool
	ExpectedAccountID  string
	ExpectedAPIVersion string
}

type ProviderEventIngestionResult struct {
	Event     SubscriptionProviderEvent
	Duplicate bool
}

type StripeProviderEventService struct {
	config             StripeProviderEventConfig
	verifier           StripeEventVerifier
	repository         ProviderEventRepository
	providerState      StripeProviderStateReader
	entitlementService *Service
	clock              domain.Clock
}

func NewStripeProviderEventService(
	config StripeProviderEventConfig,
	verifier StripeEventVerifier,
	repository ProviderEventRepository,
	providerState StripeProviderStateReader,
	entitlementService *Service,
	clock domain.Clock,
) *StripeProviderEventService {
	return &StripeProviderEventService{config: config, verifier: verifier, repository: repository, providerState: providerState, entitlementService: entitlementService, clock: clock}
}

func (service *StripeProviderEventService) Ingest(ctx context.Context, rawBody []byte, signatureHeader string) (ProviderEventIngestionResult, error) {
	verified, err := service.verifier.Verify(rawBody, signatureHeader)
	if err != nil {
		return ProviderEventIngestionResult{}, fmt.Errorf("%w: %v", ErrStripeSignatureInvalid, err)
	}
	if verified.Livemode != service.config.ExpectedLivemode ||
		verified.AccountID != service.config.ExpectedAccountID ||
		verified.APIVersion != service.config.ExpectedAPIVersion {
		return ProviderEventIngestionResult{}, ErrProviderEventEnvironmentMismatch
	}
	if strings.TrimSpace(verified.EventID) == "" || strings.TrimSpace(verified.EventType) == "" {
		return ProviderEventIngestionResult{}, domain.NewValidationError("stripeEvent", "required", "event id and type are required")
	}
	digest := sha256.Sum256(rawBody)
	event := SubscriptionProviderEvent{
		EventID: verified.EventID, Provider: EntitlementProviderStripe, EventType: verified.EventType,
		PayloadHash: hex.EncodeToString(digest[:]), Livemode: verified.Livemode, AccountID: verified.AccountID,
		APIVersion: verified.APIVersion, ProviderCreatedAt: verified.ProviderCreatedAt,
		ReceivedAt: domain.NewTimestamp(service.clock.Now()), CustomerID: verified.CustomerID,
		SubscriptionID: verified.SubscriptionID, InvoiceID: verified.InvoiceID,
		ProcessingState: ProviderEventStateReceived,
	}
	result, err := service.repository.Store(ctx, event)
	if err != nil {
		return ProviderEventIngestionResult{}, err
	}
	switch result {
	case ProviderEventStoreResultInserted:
		return ProviderEventIngestionResult{Event: event}, nil
	case ProviderEventStoreResultDuplicate:
		stored, getErr := service.repository.Get(ctx, event.EventID)
		return ProviderEventIngestionResult{Event: stored, Duplicate: true}, getErr
	case ProviderEventStoreResultConflict:
		return ProviderEventIngestionResult{}, ErrProviderEventPayloadConflict
	default:
		return ProviderEventIngestionResult{}, errors.New("unknown provider event store result")
	}
}

func (service *StripeProviderEventService) Reconcile(ctx context.Context, eventID string) (SellerPlanResponse, error) {
	event, err := service.repository.Get(ctx, eventID)
	if err != nil {
		return SellerPlanResponse{}, err
	}
	if event.ProcessingState == ProviderEventStateQuarantined {
		return SellerPlanResponse{}, ErrProviderEventQuarantined
	}
	if event.ProcessingState == ProviderEventStateApplied {
		return service.entitlementService.ResolveSellerPlan(ctx, event.SellerID)
	}
	if err := service.repository.MarkProcessing(ctx, eventID, domain.NewTimestamp(service.clock.Now())); err != nil {
		if errors.Is(err, ErrProviderEventAlreadyProcessing) {
			latest, getErr := service.repository.Get(ctx, eventID)
			if getErr == nil && latest.ProcessingState == ProviderEventStateApplied {
				return service.entitlementService.ResolveSellerPlan(ctx, latest.SellerID)
			}
		}
		return SellerPlanResponse{}, err
	}
	for attempt := 0; attempt < maximumEntitlementReconcileAttempts; attempt++ {
		candidate, readErr := service.providerState.CurrentEntitlement(ctx, event)
		if readErr != nil {
			_ = service.repository.MarkFailed(ctx, eventID, domain.NewTimestamp(service.clock.Now()))
			return SellerPlanResponse{}, readErr
		}
		candidate.Source = EntitlementSourceBillingProvider
		candidate.Provider = EntitlementProviderStripe
		candidate.LastProviderEventID = event.EventID
		current, currentErr := service.entitlementService.repository.Get(ctx, candidate.SellerID)
		if currentErr == nil && current.LastProviderEventID() == event.EventID {
			if err := service.repository.MarkApplied(ctx, eventID, candidate.SellerID, current.Version(), domain.NewTimestamp(service.clock.Now())); err != nil {
				return SellerPlanResponse{}, err
			}
			return sellerPlanResponse(current, domain.NewTimestamp(service.clock.Now()))
		}
		if currentErr != nil && !errors.Is(currentErr, persistence.ErrNotFound) && !errors.Is(currentErr, ErrSellerEntitlementNotFound) {
			_ = service.repository.MarkFailed(ctx, eventID, domain.NewTimestamp(service.clock.Now()))
			return SellerPlanResponse{}, currentErr
		}
		response, reconcileErr := service.entitlementService.ReconcileEntitlement(ctx, candidate)
		if errors.Is(reconcileErr, ErrSellerEntitlementConflict) {
			continue
		}
		if reconcileErr != nil {
			_ = service.repository.MarkFailed(ctx, eventID, domain.NewTimestamp(service.clock.Now()))
			return SellerPlanResponse{}, reconcileErr
		}
		if err := service.repository.MarkApplied(ctx, eventID, candidate.SellerID, response.Assignment.Version, domain.NewTimestamp(service.clock.Now())); err != nil {
			_ = service.repository.MarkFailed(ctx, eventID, domain.NewTimestamp(service.clock.Now()))
			return SellerPlanResponse{}, err
		}
		return response, nil
	}
	_ = service.repository.MarkFailed(ctx, eventID, domain.NewTimestamp(service.clock.Now()))
	return SellerPlanResponse{}, ErrSellerEntitlementConflict
}

type StripeWebhookVerifier struct {
	endpointSecret string
	tolerance      time.Duration
}

func NewStripeWebhookVerifier(endpointSecret string, tolerance time.Duration) *StripeWebhookVerifier {
	return &StripeWebhookVerifier{endpointSecret: endpointSecret, tolerance: tolerance}
}

func (verifier *StripeWebhookVerifier) Verify(payload []byte, signatureHeader string) (VerifiedStripeEvent, error) {
	event, err := webhook.ConstructEventWithOptions(payload, signatureHeader, verifier.endpointSecret, webhook.ConstructEventOptions{
		Tolerance: verifier.tolerance, IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		return VerifiedStripeEvent{}, err
	}
	verified := VerifiedStripeEvent{
		EventID: event.ID, EventType: string(event.Type), Livemode: event.Livemode, AccountID: event.Account,
		APIVersion: event.APIVersion, ProviderCreatedAt: domain.NewTimestamp(time.Unix(event.Created, 0)),
	}
	if event.Data == nil {
		return verified, nil
	}
	verified.CustomerID = stripeObjectID(event.Data.Object["customer"])
	verified.SubscriptionID = stripeObjectID(event.Data.Object["subscription"])
	verified.InvoiceID = stripeObjectID(event.Data.Object["invoice"])
	objectID := stripeObjectID(event.Data.Object["id"])
	if strings.HasPrefix(verified.EventType, "customer.subscription.") {
		verified.SubscriptionID = objectID
	} else if strings.HasPrefix(verified.EventType, "invoice.") {
		verified.InvoiceID = objectID
	}
	return verified, nil
}

func stripeObjectID(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		identifier, _ := typed["id"].(string)
		return strings.TrimSpace(identifier)
	default:
		return ""
	}
}
