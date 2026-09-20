package sellerworkspace

import (
	"context"
	"errors"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/transactions"
)

var (
	ErrAuthenticationRequired         = errors.New("seller authentication is required")
	ErrSellerIdentityMissing          = errors.New("authenticated seller identity is incomplete")
	ErrPublicationBlocked             = errors.New("seller publication prerequisites are incomplete")
	ErrIntegrationVerificationInvalid = errors.New("integration verification does not match the current seller route")
	ErrCredentialIssuanceBlocked      = integrations.ErrCredentialIssuanceDenied
)

type Principal struct {
	Subject       string
	SellerID      *domain.ID
	EmailVerified bool
}

type AuthenticatedPrincipal interface {
	CurrentPrincipal(context.Context) (Principal, error)
}

type StepName string

const (
	StepAccountVerified            StepName = "account_verified"
	StepStorefrontCreated          StepName = "storefront_created"
	StepServiceConnectionVerified  StepName = "service_connection_verified"
	StepSubscriptionActive         StepName = "subscription_active"
	StepPaymentDestinationVerified StepName = "payment_destination_verified"
	StepProjectKeyCreated          StepName = "project_key_created"
	StepConnectorVerified          StepName = "connector_verified"
	StepProductConfigured          StepName = "product_configured"
	StepIntegrationVerification    StepName = "integration_verification"
	StepStorefrontPreviewed        StepName = "storefront_previewed"
)

type StepStatus string

const (
	StepComplete   StepStatus = "complete"
	StepIncomplete StepStatus = "incomplete"
	StepBlocked    StepStatus = "blocked"
)

type OnboardingStep struct {
	Name     StepName   `json:"name"`
	Status   StepStatus `json:"status"`
	Blocking bool       `json:"blocking"`
	Message  string     `json:"message"`
}

type PublicationReadiness struct {
	Allowed  bool       `json:"allowed"`
	Blockers []StepName `json:"blockers"`
}

type OnboardingView struct {
	SellerID                *domain.ID                                  `json:"sellerId"`
	Complete                bool                                        `json:"complete"`
	CurrentStep             StepName                                    `json:"currentStep,omitempty"`
	Steps                   []OnboardingStep                            `json:"steps"`
	Publication             PublicationReadiness                        `json:"publication"`
	IntegrationVerification *integrations.IntegrationVerificationResult `json:"integrationVerification,omitempty"`
	Version                 uint64                                      `json:"version"`
	UpdatedAt               *domain.Timestamp                           `json:"updatedAt,omitempty"`
}

type SellerSettings struct {
	SupportEmail                string           `json:"supportEmail"`
	SecurityNotificationEmail   string           `json:"securityNotificationEmail"`
	WebhookFailureNotifications bool             `json:"webhookFailureNotifications"`
	Version                     uint64           `json:"version"`
	UpdatedAt                   domain.Timestamp `json:"updatedAt"`
}

type UpdateSettingsRequest struct {
	SupportEmail                string `json:"supportEmail"`
	SecurityNotificationEmail   string `json:"securityNotificationEmail"`
	WebhookFailureNotifications bool   `json:"webhookFailureNotifications"`
	ExpectedVersion             uint64 `json:"expectedVersion"`
}

type WorkspaceState struct {
	SellerID                     domain.ID                                   `json:"sellerId"`
	OwnerSubjectHash             string                                      `json:"ownerSubjectHash"`
	ConnectorVerifiedAt          *domain.Timestamp                           `json:"connectorVerifiedAt,omitempty"`
	SandboxPurchaseTransactionID *domain.ID                                  `json:"sandboxPurchaseTransactionId,omitempty"`
	IntegrationVerification      *integrations.IntegrationVerificationResult `json:"integrationVerification,omitempty"`
	StorefrontPreviewedAt        *domain.Timestamp                           `json:"storefrontPreviewedAt,omitempty"`
	Settings                     SellerSettings                              `json:"settings"`
	CreatedAt                    domain.Timestamp                            `json:"createdAt"`
	UpdatedAt                    domain.Timestamp                            `json:"updatedAt"`
	Version                      uint64                                      `json:"version"`
}

type Repository interface {
	Get(context.Context, domain.ID) (WorkspaceState, error)
	Put(context.Context, WorkspaceState, uint64) error
}

type SellerReader interface {
	GetSeller(context.Context, domain.ID) (catalog.Seller, error)
}

type ProductReader interface {
	ListRoutesBySeller(context.Context, domain.ID) ([]catalog.PaidRoute, error)
}

type PaymentDestinationReader interface {
	ListPaymentDestinations(context.Context, domain.ID) ([]settlement.PaymentDestination, error)
}

type CredentialReader interface {
	ListCredentials(context.Context, domain.ID) ([]integrations.CredentialView, error)
}

type TransactionReader interface {
	ListTransactions(context.Context, domain.ID, int, string) ([]transactions.Transaction, *string, error)
}

type EvidenceReader interface {
	ListByTransaction(context.Context, domain.ID) ([]evidence.Event, error)
}

type WebhookSubscriptionReader interface {
	ListWebhookSubscriptions(context.Context, domain.ID) ([]notifications.SubscriptionView, error)
}

type WebhookDeliveryReader interface {
	ListWebhookDeliveries(context.Context, domain.ID, int, string) ([]notifications.DeliveryView, *string, error)
}

type BillingReader interface {
	ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error)
}

type ProviderPlanState struct {
	Provider        string `json:"provider"`
	CustomerID      string `json:"customerId,omitempty"`
	SubscriptionID  string `json:"subscriptionId,omitempty"`
	PriceID         string `json:"priceId,omitempty"`
	PortalAvailable bool   `json:"portalAvailable"`
}

type BillingPortalSession struct {
	URL       string           `json:"url"`
	ExpiresAt domain.Timestamp `json:"expiresAt"`
}

type BillingPortal interface {
	PlanState(context.Context, domain.ID) (ProviderPlanState, error)
	CreatePortalSession(context.Context, domain.ID, string) (BillingPortalSession, error)
}

type ProductSummary struct {
	Total             int `json:"total"`
	Draft             int `json:"draft"`
	Published         int `json:"published"`
	Paused            int `json:"paused"`
	Archived          int `json:"archived"`
	EmergencyDisabled int `json:"emergencyDisabled"`
}

type TransactionSummary struct {
	Total     int `json:"total"`
	Live      int `json:"live"`
	Test      int `json:"test"`
	Pending   int `json:"pending"`
	Abandoned int `json:"abandoned"`
	Fulfilled int `json:"fulfilled"`
	Failed    int `json:"failed"`
	Disputed  int `json:"disputed"`
}

type EvidenceSummary struct {
	TransactionChains int `json:"transactionChains"`
	EventCount        int `json:"eventCount"`
}

type WebhookSummary struct {
	Subscriptions        int `json:"subscriptions"`
	ActiveSubscriptions  int `json:"activeSubscriptions"`
	PendingDeliveries    int `json:"pendingDeliveries"`
	DeadLetterDeliveries int `json:"deadLetterDeliveries"`
}

type CredentialSummary struct {
	Total   int `json:"total"`
	Active  int `json:"active"`
	Expired int `json:"expired"`
	Revoked int `json:"revoked"`
}

type BillingSummary struct {
	Entitlement     *billing.SellerEntitlementView `json:"entitlement,omitempty"`
	Plan            *billing.PlanDefinition        `json:"plan,omitempty"`
	Provider        ProviderPlanState              `json:"provider"`
	PortalAvailable bool                           `json:"portalAvailable"`
	Unavailable     bool                           `json:"unavailable"`
}

type DashboardOverview struct {
	Seller       catalog.SellerResponse `json:"seller"`
	Onboarding   OnboardingView         `json:"onboarding"`
	Products     ProductSummary         `json:"products"`
	Transactions TransactionSummary     `json:"transactions"`
	Evidence     EvidenceSummary        `json:"evidence"`
	Webhooks     WebhookSummary         `json:"webhooks"`
	Billing      BillingSummary         `json:"billing"`
	Credentials  CredentialSummary      `json:"credentials"`
	Settings     SellerSettings         `json:"settings"`
}

type Dependencies struct {
	Workspaces                   Repository
	Sellers                      SellerReader
	Products                     ProductReader
	PaymentDestinations          PaymentDestinationReader
	Credentials                  CredentialReader
	Transactions                 TransactionReader
	Evidence                     EvidenceReader
	WebhookSubscriptions         WebhookSubscriptionReader
	WebhookDeliveries            WebhookDeliveryReader
	Billing                      BillingReader
	BillingPortal                BillingPortal
	AccountVerification          AccountVerificationReader
	AuditRecorder                audit.Recorder
	Clock                        domain.Clock
	AllowLocalDevelopmentService bool
}
