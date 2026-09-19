package sellerworkspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"net/url"
	"strings"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/notifications"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/settlement"
	"github.com/fourgeez/agentpay/internal/transactions"
)

const dashboardPageSize = 100

const (
	launchAsset   = "USDC"
	launchNetwork = "eip155:84532"
)

type Service struct {
	dependencies Dependencies
}

func NewService(dependencies Dependencies) *Service {
	if dependencies.Clock == nil {
		dependencies.Clock = domain.SystemClock{}
	}
	return &Service{dependencies: dependencies}
}

func (service *Service) Onboarding(ctx context.Context, principal Principal) (OnboardingView, error) {
	resolved, err := service.resolveSeller(ctx, principal)
	if err != nil {
		if errors.Is(err, ErrSellerIdentityMissing) {
			return onboardingWithoutSeller(principal.EmailVerified), nil
		}
		return OnboardingView{}, err
	}
	state, err := service.loadOrCreateState(ctx, principal, resolved.SellerID)
	if err != nil {
		return OnboardingView{}, err
	}
	routes, err := service.dependencies.Products.ListRoutesBySeller(ctx, resolved.SellerID)
	if err != nil {
		return OnboardingView{}, err
	}
	destinations, err := service.dependencies.PaymentDestinations.ListPaymentDestinations(ctx, resolved.SellerID)
	if err != nil {
		return OnboardingView{}, err
	}
	credentials, err := service.dependencies.Credentials.ListCredentials(ctx, resolved.SellerID)
	if err != nil {
		return OnboardingView{}, err
	}
	entitlement, entitlementErr := service.dependencies.Billing.ResolveSellerPlan(ctx, resolved.SellerID)
	return buildOnboardingView(principal, resolved, state, routes, destinations, credentials, entitlement, entitlementErr, domain.NewTimestamp(service.dependencies.Clock.Now())), nil
}

func (service *Service) Dashboard(ctx context.Context, principal Principal) (DashboardOverview, error) {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return DashboardOverview{}, err
	}
	onboarding, err := service.Onboarding(ctx, principal)
	if err != nil {
		return DashboardOverview{}, err
	}
	routes, err := service.dependencies.Products.ListRoutesBySeller(ctx, seller.SellerID)
	if err != nil {
		return DashboardOverview{}, err
	}
	transactionRows, err := service.loadTransactions(ctx, seller.SellerID)
	if err != nil {
		return DashboardOverview{}, err
	}
	subscriptions, err := service.dependencies.WebhookSubscriptions.ListWebhookSubscriptions(ctx, seller.SellerID)
	if err != nil {
		return DashboardOverview{}, err
	}
	deliveries, _, err := service.dependencies.WebhookDeliveries.ListWebhookDeliveries(ctx, seller.SellerID, dashboardPageSize, "")
	if err != nil {
		return DashboardOverview{}, err
	}
	credentials, err := service.dependencies.Credentials.ListCredentials(ctx, seller.SellerID)
	if err != nil {
		return DashboardOverview{}, err
	}
	settings, err := service.Settings(ctx, principal)
	if err != nil {
		return DashboardOverview{}, err
	}
	billingSummary, err := service.Billing(ctx, principal)
	if err != nil {
		return DashboardOverview{}, err
	}
	evidenceSummary, err := service.evidenceSummary(ctx, transactionRows)
	if err != nil {
		return DashboardOverview{}, err
	}
	return DashboardOverview{
		Seller:     catalog.SellerResponse{SellerID: seller.SellerID, Name: seller.Name, Slug: seller.Slug, UpstreamBaseURL: seller.UpstreamBaseURL, Status: seller.Status, CreatedAt: seller.CreatedAt, UpdatedAt: seller.UpdatedAt, Version: seller.Version},
		Onboarding: onboarding, Products: summarizeProducts(routes), Transactions: summarizeTransactions(transactionRows), Evidence: evidenceSummary,
		Webhooks: summarizeWebhooks(subscriptions, deliveries), Billing: billingSummary,
		Credentials: summarizeCredentials(credentials, domain.NewTimestamp(service.dependencies.Clock.Now())), Settings: settings,
	}, nil
}

func (service *Service) Products(ctx context.Context, principal Principal) (ProductSummary, error) {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return ProductSummary{}, err
	}
	routes, err := service.dependencies.Products.ListRoutesBySeller(ctx, seller.SellerID)
	if err != nil {
		return ProductSummary{}, err
	}
	return summarizeProducts(routes), nil
}

func (service *Service) Transactions(ctx context.Context, principal Principal) (TransactionSummary, error) {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return TransactionSummary{}, err
	}
	rows, err := service.loadTransactions(ctx, seller.SellerID)
	if err != nil {
		return TransactionSummary{}, err
	}
	return summarizeTransactions(rows), nil
}

func (service *Service) Evidence(ctx context.Context, principal Principal) (EvidenceSummary, error) {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return EvidenceSummary{}, err
	}
	rows, err := service.loadTransactions(ctx, seller.SellerID)
	if err != nil {
		return EvidenceSummary{}, err
	}
	return service.evidenceSummary(ctx, rows)
}

func (service *Service) Webhooks(ctx context.Context, principal Principal) (WebhookSummary, error) {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return WebhookSummary{}, err
	}
	subscriptions, err := service.dependencies.WebhookSubscriptions.ListWebhookSubscriptions(ctx, seller.SellerID)
	if err != nil {
		return WebhookSummary{}, err
	}
	deliveries, _, err := service.dependencies.WebhookDeliveries.ListWebhookDeliveries(ctx, seller.SellerID, dashboardPageSize, "")
	if err != nil {
		return WebhookSummary{}, err
	}
	return summarizeWebhooks(subscriptions, deliveries), nil
}

func (service *Service) Credentials(ctx context.Context, principal Principal) (CredentialSummary, error) {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return CredentialSummary{}, err
	}
	credentials, err := service.dependencies.Credentials.ListCredentials(ctx, seller.SellerID)
	if err != nil {
		return CredentialSummary{}, err
	}
	return summarizeCredentials(credentials, domain.NewTimestamp(service.dependencies.Clock.Now())), nil
}

func (service *Service) Billing(ctx context.Context, principal Principal) (BillingSummary, error) {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return BillingSummary{}, err
	}
	provider, err := service.dependencies.BillingPortal.PlanState(ctx, seller.SellerID)
	if err != nil {
		return BillingSummary{}, err
	}
	plan, err := service.dependencies.Billing.ResolveSellerPlan(ctx, seller.SellerID)
	if errors.Is(err, billing.ErrSellerEntitlementNotFound) || errors.Is(err, persistence.ErrNotFound) {
		return BillingSummary{Provider: provider, PortalAvailable: provider.PortalAvailable, Unavailable: true}, nil
	}
	if err != nil {
		return BillingSummary{}, err
	}
	return BillingSummary{Entitlement: &plan.Assignment, Plan: &plan.Plan, Provider: provider, PortalAvailable: provider.PortalAvailable}, nil
}

func (service *Service) Settings(ctx context.Context, principal Principal) (SellerSettings, error) {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return SellerSettings{}, err
	}
	state, err := service.loadOrCreateState(ctx, principal, seller.SellerID)
	if err != nil {
		return SellerSettings{}, err
	}
	return state.Settings, nil
}

func (service *Service) UpdateSettings(ctx context.Context, principal Principal, request UpdateSettingsRequest) (SellerSettings, error) {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return SellerSettings{}, err
	}
	state, err := service.loadOrCreateState(ctx, principal, seller.SellerID)
	if err != nil {
		return SellerSettings{}, err
	}
	if request.ExpectedVersion != state.Version {
		return SellerSettings{}, persistence.ErrConditionFailed
	}
	if err := validateOptionalEmail("supportEmail", request.SupportEmail); err != nil {
		return SellerSettings{}, err
	}
	if err := validateOptionalEmail("securityNotificationEmail", request.SecurityNotificationEmail); err != nil {
		return SellerSettings{}, err
	}
	now := domain.NewTimestamp(service.dependencies.Clock.Now())
	state.Settings = SellerSettings{SupportEmail: strings.TrimSpace(request.SupportEmail), SecurityNotificationEmail: strings.TrimSpace(request.SecurityNotificationEmail), WebhookFailureNotifications: request.WebhookFailureNotifications, Version: state.Version + 1, UpdatedAt: now}
	state.UpdatedAt = now
	state.Version++
	if err := service.dependencies.Workspaces.Put(ctx, state, request.ExpectedVersion); err != nil {
		return SellerSettings{}, err
	}
	return state.Settings, nil
}

func (service *Service) CreateBillingPortalSession(ctx context.Context, principal Principal, returnURL string) (BillingPortalSession, error) {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return BillingPortalSession{}, err
	}
	if strings.TrimSpace(returnURL) == "" {
		return BillingPortalSession{}, domain.NewValidationError("returnUrl", "required", "is required")
	}
	return service.dependencies.BillingPortal.CreatePortalSession(ctx, seller.SellerID, returnURL)
}

func (service *Service) AuthorizePublication(ctx context.Context, sellerID domain.ID) error {
	state, err := service.dependencies.Workspaces.Get(ctx, sellerID)
	if err != nil {
		return ErrPublicationBlocked
	}
	principal := Principal{Subject: "publication-gate", SellerID: &sellerID, EmailVerified: true}
	_ = state
	view, err := service.onboardingForSeller(ctx, principal, sellerID, state)
	if err != nil || !view.Publication.Allowed {
		return ErrPublicationBlocked
	}
	return nil
}

// AuthorizeCredentialIssuance gates reveal-once project keys on authoritative
// pre-connector requirements only. Full signed sandbox validation follows MCP
// integration and remains mandatory for publication.
func (service *Service) AuthorizeCredentialIssuance(ctx context.Context, ownerSubject string, sellerID domain.ID) error {
	principal := Principal{Subject: ownerSubject, SellerID: &sellerID}
	if service.dependencies.AccountVerification == nil {
		return integrations.ErrCredentialIssuanceUnavailable
	}
	verified, err := service.dependencies.AccountVerification.EmailVerified(ctx, ownerSubject)
	if err != nil {
		return err
	}
	principal.EmailVerified = verified
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return err
	}
	destinations, err := service.dependencies.PaymentDestinations.ListPaymentDestinations(ctx, sellerID)
	if err != nil {
		return err
	}
	entitlement, err := service.dependencies.Billing.ResolveSellerPlan(ctx, sellerID)
	if err != nil {
		return err
	}
	now := domain.NewTimestamp(service.dependencies.Clock.Now())
	if !verified || !sellerProfileReady(seller) || !serviceConnectionReady(seller) ||
		!hasSupportedActiveDestination(destinations) ||
		entitlement.Assignment.Status != billing.EntitlementStatusActive ||
		entitlement.Assignment.NetworkAccess != billing.NetworkAccessEnabled ||
		!now.Before(entitlement.Assignment.AccessEndsAt) {
		return ErrCredentialIssuanceBlocked
	}
	if _, err := service.loadOrCreateState(ctx, principal, sellerID); err != nil {
		return err
	}
	return nil
}

func (service *Service) RecordConnectorVerification(ctx context.Context, principal Principal) error {
	return service.recordProgress(ctx, principal, func(state *WorkspaceState, now domain.Timestamp) { state.ConnectorVerifiedAt = &now })
}

// RecordAuthenticatedConnectorVerification records the cloud-authorized MCP
// boundary without trusting caller-supplied seller progress. Replays are no-ops.
func (service *Service) RecordAuthenticatedConnectorVerification(ctx context.Context, sellerID domain.ID) error {
	state, err := service.dependencies.Workspaces.Get(ctx, sellerID)
	if err != nil {
		return err
	}
	if state.ConnectorVerifiedAt != nil {
		return nil
	}
	expectedVersion := state.Version
	now := domain.NewTimestamp(service.dependencies.Clock.Now())
	state.ConnectorVerifiedAt = &now
	state.UpdatedAt = now
	state.Version++
	state.Settings.Version = state.Version
	state.Settings.UpdatedAt = now
	if err := service.dependencies.Workspaces.Put(ctx, state, expectedVersion); err != nil {
		if errors.Is(err, persistence.ErrConditionFailed) {
			latest, loadErr := service.dependencies.Workspaces.Get(ctx, sellerID)
			if loadErr == nil && latest.ConnectorVerifiedAt != nil {
				return nil
			}
		}
		return err
	}
	return nil
}

func (service *Service) RecordSandboxPurchase(ctx context.Context, principal Principal, transactionID domain.ID) error {
	if transactionID.Prefix() != domain.TransactionIDPrefix {
		return domain.NewValidationError("transactionId", "prefix", "must identify a transaction")
	}
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return err
	}
	rows, err := service.loadTransactions(ctx, seller.SellerID)
	if err != nil {
		return err
	}
	valid := false
	for _, transaction := range rows {
		if transaction.TransactionID() == transactionID && transaction.SellerID() == seller.SellerID && transaction.Status() == transactions.StatusFulfilled {
			valid = true
			break
		}
	}
	if !valid {
		return ErrSandboxPurchaseInvalid
	}
	return service.recordProgress(ctx, principal, func(state *WorkspaceState, _ domain.Timestamp) { state.SandboxPurchaseTransactionID = &transactionID })
}

func (service *Service) RecordStorefrontPreview(ctx context.Context, principal Principal) error {
	return service.recordProgress(ctx, principal, func(state *WorkspaceState, now domain.Timestamp) { state.StorefrontPreviewedAt = &now })
}

func (service *Service) recordProgress(ctx context.Context, principal Principal, mutate func(*WorkspaceState, domain.Timestamp)) error {
	seller, err := service.resolveSeller(ctx, principal)
	if err != nil {
		return err
	}
	state, err := service.loadOrCreateState(ctx, principal, seller.SellerID)
	if err != nil {
		return err
	}
	expectedVersion := state.Version
	now := domain.NewTimestamp(service.dependencies.Clock.Now())
	mutate(&state, now)
	state.UpdatedAt = now
	state.Version++
	state.Settings.Version = state.Version
	state.Settings.UpdatedAt = now
	return service.dependencies.Workspaces.Put(ctx, state, expectedVersion)
}

func (service *Service) onboardingForSeller(ctx context.Context, principal Principal, sellerID domain.ID, state WorkspaceState) (OnboardingView, error) {
	seller, err := service.dependencies.Sellers.GetSeller(ctx, sellerID)
	if err != nil {
		return OnboardingView{}, err
	}
	routes, err := service.dependencies.Products.ListRoutesBySeller(ctx, sellerID)
	if err != nil {
		return OnboardingView{}, err
	}
	destinations, err := service.dependencies.PaymentDestinations.ListPaymentDestinations(ctx, sellerID)
	if err != nil {
		return OnboardingView{}, err
	}
	credentials, err := service.dependencies.Credentials.ListCredentials(ctx, sellerID)
	if err != nil {
		return OnboardingView{}, err
	}
	entitlement, entitlementErr := service.dependencies.Billing.ResolveSellerPlan(ctx, sellerID)
	return buildOnboardingView(principal, seller, state, routes, destinations, credentials, entitlement, entitlementErr, domain.NewTimestamp(service.dependencies.Clock.Now())), nil
}

func (service *Service) resolveSeller(ctx context.Context, principal Principal) (catalog.Seller, error) {
	if strings.TrimSpace(principal.Subject) == "" {
		return catalog.Seller{}, ErrAuthenticationRequired
	}
	if principal.SellerID == nil {
		return catalog.Seller{}, ErrSellerIdentityMissing
	}
	seller, err := service.dependencies.Sellers.GetSeller(ctx, *principal.SellerID)
	if err != nil {
		return catalog.Seller{}, err
	}
	if seller.OwnerSubject != principal.Subject {
		return catalog.Seller{}, persistence.ErrNotFound
	}
	return seller, nil
}

func (service *Service) loadOrCreateState(ctx context.Context, principal Principal, sellerID domain.ID) (WorkspaceState, error) {
	state, err := service.dependencies.Workspaces.Get(ctx, sellerID)
	if err == nil {
		if state.OwnerSubjectHash != hashSubject(principal.Subject) {
			return WorkspaceState{}, persistence.ErrNotFound
		}
		return state, nil
	}
	if !errors.Is(err, persistence.ErrNotFound) {
		return WorkspaceState{}, err
	}
	now := domain.NewTimestamp(service.dependencies.Clock.Now())
	state = WorkspaceState{SellerID: sellerID, OwnerSubjectHash: hashSubject(principal.Subject), CreatedAt: now, UpdatedAt: now, Version: 1, Settings: SellerSettings{Version: 1, UpdatedAt: now}}
	if err := service.dependencies.Workspaces.Put(ctx, state, 0); err != nil {
		return WorkspaceState{}, err
	}
	return state, nil
}

func (service *Service) loadTransactions(ctx context.Context, sellerID domain.ID) ([]transactions.Transaction, error) {
	rows := make([]transactions.Transaction, 0)
	cursor := ""
	for len(rows) < 1000 {
		page, next, err := service.dependencies.Transactions.ListTransactions(ctx, sellerID, dashboardPageSize, cursor)
		if err != nil {
			return nil, err
		}
		rows = append(rows, page...)
		if next == nil {
			return rows, nil
		}
		cursor = *next
	}
	return rows, nil
}

func (service *Service) evidenceSummary(ctx context.Context, rows []transactions.Transaction) (EvidenceSummary, error) {
	summary := EvidenceSummary{}
	for _, transaction := range rows {
		events, err := service.dependencies.Evidence.ListByTransaction(ctx, transaction.TransactionID())
		if err != nil {
			return EvidenceSummary{}, err
		}
		if len(events) > 0 {
			summary.TransactionChains++
		}
		summary.EventCount += len(events)
	}
	return summary, nil
}

func buildOnboardingView(principal Principal, seller catalog.Seller, state WorkspaceState, routes []catalog.PaidRoute, destinations []settlement.PaymentDestination, credentials []integrations.CredentialView, entitlement billing.SellerPlanResponse, entitlementErr error, now domain.Timestamp) OnboardingView {
	checks := []struct {
		name     StepName
		complete bool
		blocked  bool
		message  string
	}{
		{StepAccountVerified, principal.EmailVerified, !principal.EmailVerified, "Verify the seller account email."},
		{StepStorefrontCreated, state.SellerID != "", state.SellerID == "", "Create the seller storefront."},
		{StepServiceConnectionVerified, serviceConnectionReady(seller), !serviceConnectionReady(seller), "Configure an active HTTPS service endpoint and AgentPay request signing."},
		{StepSubscriptionActive, entitlementErr == nil && entitlement.Assignment.NetworkAccess == billing.NetworkAccessEnabled && now.Before(entitlement.Assignment.AccessEndsAt), entitlementErr != nil || entitlement.Assignment.NetworkAccess != billing.NetworkAccessEnabled, "Request an active testnet launch entitlement."},
		{StepPaymentDestinationVerified, hasSupportedActiveDestination(destinations), false, "Verify USDC on Base Sepolia to a seller-controlled address."},
		{StepProjectKeyCreated, hasActiveCredential(credentials, now), false, "Create an active project connection key."},
		{StepConnectorVerified, state.ConnectorVerifiedAt != nil, false, "Connect and verify the local MCP connector."},
		{StepProductConfigured, len(routes) > 0, false, "Configure at least one product."},
		{StepSandboxPurchase, state.SandboxPurchaseTransactionID != nil, false, "Complete the exactly-once sandbox purchase."},
		{StepStorefrontPreviewed, state.StorefrontPreviewedAt != nil, false, "Review the storefront preview."},
	}
	view := OnboardingView{SellerID: &state.SellerID, Complete: true, Publication: PublicationReadiness{Allowed: true, Blockers: []StepName{}}, Version: state.Version, UpdatedAt: &state.UpdatedAt}
	for _, check := range checks {
		status := StepComplete
		if !check.complete {
			status = StepIncomplete
			if check.blocked {
				status = StepBlocked
			}
			view.Complete = false
			view.Publication.Allowed = false
			view.Publication.Blockers = append(view.Publication.Blockers, check.name)
			if view.CurrentStep == "" {
				view.CurrentStep = check.name
			}
		}
		view.Steps = append(view.Steps, OnboardingStep{Name: check.name, Status: status, Blocking: !check.complete, Message: check.message})
	}
	return view
}

func onboardingWithoutSeller(emailVerified bool) OnboardingView {
	state := WorkspaceState{}
	return buildOnboardingView(Principal{EmailVerified: emailVerified}, catalog.Seller{}, state, nil, nil, nil, billing.SellerPlanResponse{}, billing.ErrSellerEntitlementNotFound, domain.Timestamp{})
}

func summarizeProducts(routes []catalog.PaidRoute) ProductSummary {
	summary := ProductSummary{Total: len(routes)}
	for _, route := range routes {
		switch route.LifecycleStatus {
		case catalog.RouteLifecycleDraft:
			summary.Draft++
		case catalog.RouteLifecyclePublished:
			summary.Published++
		case catalog.RouteLifecyclePaused:
			summary.Paused++
		case catalog.RouteLifecycleArchived:
			summary.Archived++
		case catalog.RouteLifecycleEmergencyDisabled:
			summary.EmergencyDisabled++
		}
	}
	return summary
}
func summarizeTransactions(rows []transactions.Transaction) TransactionSummary {
	summary := TransactionSummary{Total: len(rows)}
	for _, row := range rows {
		switch row.Status() {
		case transactions.StatusFulfilled:
			summary.Fulfilled++
		case transactions.StatusFailed, transactions.StatusRefundRecommended:
			summary.Failed++
		case transactions.StatusDisputed:
			summary.Disputed++
		default:
			summary.Pending++
		}
	}
	return summary
}
func summarizeWebhooks(subscriptions []notifications.SubscriptionView, deliveries []notifications.DeliveryView) WebhookSummary {
	summary := WebhookSummary{Subscriptions: len(subscriptions)}
	for _, subscription := range subscriptions {
		if subscription.Status == notifications.SubscriptionStatusActive {
			summary.ActiveSubscriptions++
		}
	}
	for _, delivery := range deliveries {
		switch delivery.Status {
		case notifications.DeliveryStatusPending, notifications.DeliveryStatusRetryScheduled:
			summary.PendingDeliveries++
		case notifications.DeliveryStatusDeadLetter:
			summary.DeadLetterDeliveries++
		}
	}
	return summary
}
func summarizeCredentials(credentials []integrations.CredentialView, now domain.Timestamp) CredentialSummary {
	summary := CredentialSummary{Total: len(credentials)}
	for _, credential := range credentials {
		if credential.RevokedAt != nil {
			summary.Revoked++
		} else if credential.ExpiresAt != nil && !now.Before(*credential.ExpiresAt) {
			summary.Expired++
		} else {
			summary.Active++
		}
	}
	return summary
}
func hasActiveDestination(destinations []settlement.PaymentDestination) bool {
	for _, destination := range destinations {
		if destination.Status == settlement.PaymentDestinationStatusActive {
			return true
		}
	}
	return false
}

func hasSupportedActiveDestination(destinations []settlement.PaymentDestination) bool {
	for _, destination := range destinations {
		if destination.Status == settlement.PaymentDestinationStatusActive && destination.Asset == launchAsset && destination.Network == launchNetwork {
			return true
		}
	}
	return false
}

func sellerProfileReady(seller catalog.Seller) bool {
	return strings.TrimSpace(seller.Name) != "" && strings.TrimSpace(seller.Slug) != ""
}

func serviceConnectionReady(seller catalog.Seller) bool {
	serviceURL, err := url.Parse(strings.TrimSpace(seller.UpstreamBaseURL))
	return err == nil && serviceURL.Scheme == "https" && serviceURL.Host != "" && seller.Status == catalog.SellerStatusActive && strings.TrimSpace(seller.SigningSecretRef) != ""
}
func hasActiveCredential(credentials []integrations.CredentialView, now domain.Timestamp) bool {
	return summarizeCredentials(credentials, now).Active > 0
}
func hashSubject(subject string) string {
	digest := sha256.Sum256([]byte(subject))
	return hex.EncodeToString(digest[:])
}
func validateOptionalEmail(field, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	address, err := mail.ParseAddress(trimmed)
	if err != nil || address.Address != trimmed {
		return domain.NewValidationError(field, "email", "must be a valid email address")
	}
	return nil
}
