package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const (
	currentPlanVersion  uint64 = 1
	sourceRevisionWidth        = 20
)

var planCatalogV1 = map[PlanID]PlanDefinition{
	PlanStarter: {
		PlanID: PlanStarter, PlanVersion: currentPlanVersion, DisplayName: "Starter",
		Limits:   PlanLimits{APIRequestsPerMonth: 10_000, MCPOperationsPerMonth: 1_000, PublishedRoutes: 5, WebhookSubscriptions: 1, WebhookDeliveriesPerMonth: 1_000, AnalyticsWindowDays: 7, EvidenceRetentionDays: 30},
		Features: PlanFeatures{Webhooks: true},
	},
	PlanGrowth: {
		PlanID: PlanGrowth, PlanVersion: currentPlanVersion, DisplayName: "Growth",
		Limits:   PlanLimits{APIRequestsPerMonth: 100_000, MCPOperationsPerMonth: 10_000, PublishedRoutes: 50, WebhookSubscriptions: 5, WebhookDeliveriesPerMonth: 25_000, AnalyticsWindowDays: 90, EvidenceRetentionDays: 180},
		Features: PlanFeatures{Approvals: true, Webhooks: true, AdvancedAnalytics: true},
	},
	PlanScale: {
		PlanID: PlanScale, PlanVersion: currentPlanVersion, DisplayName: "Scale",
		Limits:   PlanLimits{APIRequestsPerMonth: 1_000_000, MCPOperationsPerMonth: 100_000, PublishedRoutes: 500, WebhookSubscriptions: 25, WebhookDeliveriesPerMonth: 250_000, AnalyticsWindowDays: 365, EvidenceRetentionDays: 3_650},
		Features: PlanFeatures{Approvals: true, Webhooks: true, AdvancedAnalytics: true, PrioritySupport: true},
	},
}

type Service struct {
	repository    Repository
	authorizer    SellerAuthorizer
	clock         domain.Clock
	invalidator   EntitlementInvalidator
	auditRecorder audit.Recorder
}

func NewService(repository Repository, authorizer SellerAuthorizer, clock domain.Clock) *Service {
	return &Service{repository: repository, authorizer: authorizer, clock: clock, invalidator: noopEntitlementInvalidator{}, auditRecorder: audit.NoopRecorder{}}
}

func (service *Service) SetAuditRecorder(recorder audit.Recorder) {
	if recorder == nil {
		service.auditRecorder = audit.NoopRecorder{}
		return
	}
	service.auditRecorder = recorder
}

func (service *Service) SetEntitlementInvalidator(invalidator EntitlementInvalidator) {
	if invalidator == nil {
		service.invalidator = noopEntitlementInvalidator{}
		return
	}
	service.invalidator = invalidator
}

func V1PlanCatalog() []PlanDefinition {
	plans := make([]PlanDefinition, 0, len(planCatalogV1))
	for _, plan := range planCatalogV1 {
		plans = append(plans, plan)
	}
	sort.Slice(plans, func(leftIndex int, rightIndex int) bool {
		order := map[PlanID]int{PlanStarter: 0, PlanGrowth: 1, PlanScale: 2}
		return order[plans[leftIndex].PlanID] < order[plans[rightIndex].PlanID]
	})
	return plans
}

func (service *Service) GetSellerPlan(ctx context.Context, ownerSubject string, sellerID domain.ID) (SellerPlanResponse, error) {
	if err := service.authorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return SellerPlanResponse{}, ErrSellerEntitlementNotFound
	}
	return service.ResolveSellerPlan(ctx, sellerID)
}

func (service *Service) ResolveSellerPlan(ctx context.Context, sellerID domain.ID) (SellerPlanResponse, error) {
	entitlement, err := service.repository.Get(ctx, sellerID)
	if errors.Is(err, persistence.ErrNotFound) || errors.Is(err, ErrSellerEntitlementNotFound) {
		return SellerPlanResponse{}, ErrSellerEntitlementNotFound
	}
	if err != nil {
		return SellerPlanResponse{}, err
	}
	return sellerPlanResponse(entitlement, domain.NewTimestamp(service.clock.Now()))
}

func (service *Service) ReconcileEntitlement(ctx context.Context, candidate EntitlementCandidate) (SellerPlanResponse, error) {
	var current *SellerEntitlement
	expectedVersion := uint64(0)
	stored, err := service.repository.Get(ctx, candidate.SellerID)
	if err == nil {
		current = &stored
		expectedVersion = stored.Version()
	} else if !errors.Is(err, persistence.ErrNotFound) && !errors.Is(err, ErrSellerEntitlementNotFound) {
		return SellerPlanResponse{}, err
	}
	now := domain.NewTimestamp(service.clock.Now())
	entitlement, reconciliation, err := ReconcileSellerEntitlement(current, candidate, now)
	if err != nil {
		return SellerPlanResponse{}, err
	}
	if err := service.repository.Apply(ctx, entitlement, reconciliation, expectedVersion); err != nil {
		if errors.Is(err, persistence.ErrConditionFailed) {
			return SellerPlanResponse{}, ErrSellerEntitlementConflict
		}
		return SellerPlanResponse{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID: entitlement.SellerID(), ActorType: audit.ActorTypeSystem, ActorID: "billing-reconciliation",
		Action: audit.ActionEntitlementChanged, TargetType: audit.TargetTypeSeller,
		TargetID: entitlement.SellerID().String(), Outcome: audit.OutcomeSucceeded,
		ChangedFields: []string{"status", "accessEndsAt", "entitlementEpoch", "sourceRevision", "credentialRotationRequired"},
	}); err != nil {
		return SellerPlanResponse{}, err
	}
	if err := service.invalidator.InvalidateSellerEntitlement(ctx, entitlement.SellerID()); err != nil {
		return SellerPlanResponse{}, err
	}
	return sellerPlanResponse(entitlement, now)
}

func (service *Service) AssignPlan(ctx context.Context, sellerID domain.ID, planID PlanID, expectedVersion uint64) (SellerPlanResponse, error) {
	current, err := service.repository.Get(ctx, sellerID)
	if err != nil {
		return SellerPlanResponse{}, err
	}
	if current.Version() != expectedVersion {
		return SellerPlanResponse{}, ErrSellerEntitlementConflict
	}
	planID = normalizePlanID(planID)
	if _, exists := planCatalogV1[planID]; !exists {
		return SellerPlanResponse{}, ErrPlanNotFound
	}
	candidate := candidateFromEntitlement(current)
	candidate.PlanID = planID
	candidate.PlanVersion = currentPlanVersion
	candidate.Source = EntitlementSourceLocal
	candidate.Provider = EntitlementProviderLocal
	return service.ReconcileEntitlement(ctx, candidate)
}

func NewSellerEntitlement(params SellerEntitlementParams) (SellerEntitlement, error) {
	var validationErrors domain.ValidationErrors
	if params.SellerID.Prefix() != domain.SellerIDPrefix {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "prefix", "must use the seller prefix"))
	}
	plan, exists := planCatalogV1[params.PlanID]
	if !exists || plan.PlanVersion != params.PlanVersion {
		validationErrors = append(validationErrors, domain.NewValidationError("plan", "version", "must reference a supported plan version"))
	}
	if !validEntitlementStatus(params.Status) {
		validationErrors = append(validationErrors, domain.NewValidationError("status", "supported", "must use a supported entitlement status"))
	}
	if params.BillingPeriodStart.Time().IsZero() || params.BillingPeriodEnd.Time().IsZero() || !params.BillingPeriodStart.Before(params.BillingPeriodEnd) {
		validationErrors = append(validationErrors, domain.NewValidationError("billingPeriod", "range", "must have ordered UTC boundaries"))
	}
	if params.AccessEndsAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("accessEndsAt", "required", "is required"))
	}
	if params.Status == EntitlementStatusGrace {
		if params.GraceEndsAt == nil || !params.GraceEndsAt.Time().Equal(params.AccessEndsAt.Add(EntitlementGraceDuration).Time()) {
			validationErrors = append(validationErrors, domain.NewValidationError("graceEndsAt", "exact", "must equal accessEndsAt plus 72 hours"))
		}
	} else if params.GraceEndsAt != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("graceEndsAt", "state", "is only allowed for grace status"))
	}
	if params.EntitlementEpoch == 0 {
		validationErrors = append(validationErrors, domain.NewValidationError("entitlementEpoch", "minimum", "must be at least one"))
	}
	if !validEntitlementSource(params.Source) {
		validationErrors = append(validationErrors, domain.NewValidationError("source", "supported", "must use a supported entitlement source"))
	}
	if !validSourceRevision(params.SourceRevision) {
		validationErrors = append(validationErrors, domain.NewValidationError("sourceRevision", "sequence", "must be a zero-padded local sequence"))
	}
	if !validEntitlementProvider(params.Provider) {
		validationErrors = append(validationErrors, domain.NewValidationError("provider", "supported", "must use a supported entitlement provider"))
	}
	if params.Provider == EntitlementProviderStripe && (strings.TrimSpace(params.ProviderCustomerID) == "" || strings.TrimSpace(params.ProviderSubscriptionID) == "" || strings.TrimSpace(params.ProviderPriceID) == "") {
		validationErrors = append(validationErrors, domain.NewValidationError("providerIdentifiers", "required", "Stripe projections require customer, subscription, and price identifiers"))
	}
	if params.AssignedAt.Time().IsZero() || params.UpdatedAt.Time().IsZero() || params.UpdatedAt.Before(params.AssignedAt) {
		validationErrors = append(validationErrors, domain.NewValidationError("timestamps", "chronology", "assignment timestamps must be ordered"))
	}
	if params.Version == 0 {
		validationErrors = append(validationErrors, domain.NewValidationError("version", "minimum", "must be at least one"))
	}
	if len(validationErrors) > 0 {
		return SellerEntitlement{}, validationErrors
	}
	return SellerEntitlement{
		sellerID: params.SellerID, planID: params.PlanID, planVersion: params.PlanVersion, status: params.Status,
		billingPeriodStart: params.BillingPeriodStart, billingPeriodEnd: params.BillingPeriodEnd, accessEndsAt: params.AccessEndsAt,
		graceEndsAt: copyTimestamp(params.GraceEndsAt), cancelAtPeriodEnd: params.CancelAtPeriodEnd, entitlementEpoch: params.EntitlementEpoch,
		source: params.Source, sourceRevision: params.SourceRevision, statusReason: params.StatusReason, provider: params.Provider,
		providerCustomerID: strings.TrimSpace(params.ProviderCustomerID), providerSubscriptionID: strings.TrimSpace(params.ProviderSubscriptionID), providerPriceID: strings.TrimSpace(params.ProviderPriceID),
		lastProviderEventID: strings.TrimSpace(params.LastProviderEventID), lastReconciledAt: copyTimestamp(params.LastReconciledAt), credentialRotationRequired: params.CredentialRotationRequired,
		assignedAt: params.AssignedAt, updatedAt: params.UpdatedAt, version: params.Version,
	}, nil
}

func NewSellerPlan(params SellerPlanParams) (SellerPlan, error) { return NewSellerEntitlement(params) }

func ReconcileSellerEntitlement(current *SellerEntitlement, candidate EntitlementCandidate, reconciledAt domain.Timestamp) (SellerEntitlement, EntitlementReconciliation, error) {
	if reconciledAt.Time().IsZero() {
		return SellerEntitlement{}, EntitlementReconciliation{}, domain.NewValidationError("reconciledAt", "required", "is required")
	}
	normalizeCandidateAtBoundary(&candidate, reconciledAt)
	triggeringEventID := candidate.LastProviderEventID
	if current != nil && current.StatusReason() == EntitlementStatusReasonFraudQuarantine && candidate.Source == EntitlementSourceBillingProvider {
		candidate = candidateFromEntitlement(*current)
		candidate.LastProviderEventID = triggeringEventID
	}
	if current != nil && current.Status() == EntitlementStatusClosed {
		candidate = candidateFromEntitlement(*current)
		candidate.LastProviderEventID = triggeringEventID
		candidate.Source = EntitlementSourceOperator
		candidate.Provider = EntitlementProviderOperator
		candidate.Status = EntitlementStatusClosed
		candidate.StatusReason = EntitlementStatusReasonAccountClosed
	}
	revision, err := nextSourceRevision(current)
	if err != nil {
		return SellerEntitlement{}, EntitlementReconciliation{}, err
	}
	assignedAt := reconciledAt
	epoch := uint64(1)
	version := uint64(1)
	rotationRequired := false
	if current != nil {
		assignedAt = current.AssignedAt()
		if current.PlanID() != candidate.PlanID || current.PlanVersion() != candidate.PlanVersion {
			assignedAt = reconciledAt
		}
		epoch = current.EntitlementEpoch()
		version = current.Version() + 1
		rotationRequired = current.CredentialRotationRequired()
		if invalidatesCapabilities(*current, candidate) {
			epoch++
		}
		if current.Status() != EntitlementStatusActive && candidate.Status == EntitlementStatusActive {
			rotationRequired = true
		}
	}
	lastReconciledAt := reconciledAt
	params := SellerEntitlementParams{
		SellerID: candidate.SellerID, PlanID: candidate.PlanID, PlanVersion: candidate.PlanVersion, Status: candidate.Status,
		BillingPeriodStart: candidate.BillingPeriodStart, BillingPeriodEnd: candidate.BillingPeriodEnd, AccessEndsAt: candidate.AccessEndsAt,
		GraceEndsAt: graceEndForCandidate(candidate), CancelAtPeriodEnd: candidate.CancelAtPeriodEnd, EntitlementEpoch: epoch,
		Source: candidate.Source, SourceRevision: revision, StatusReason: candidate.StatusReason, Provider: candidate.Provider,
		ProviderCustomerID: candidate.ProviderCustomerID, ProviderSubscriptionID: candidate.ProviderSubscriptionID, ProviderPriceID: candidate.ProviderPriceID,
		LastProviderEventID: candidate.LastProviderEventID, LastReconciledAt: &lastReconciledAt, CredentialRotationRequired: rotationRequired,
		AssignedAt: assignedAt, UpdatedAt: reconciledAt, Version: version,
	}
	entitlement, err := NewSellerEntitlement(params)
	if err != nil {
		return SellerEntitlement{}, EntitlementReconciliation{}, err
	}
	hash, err := entitlementSnapshotHash(entitlement.Snapshot())
	if err != nil {
		return SellerEntitlement{}, EntitlementReconciliation{}, err
	}
	eventIDs := []string{}
	if candidate.LastProviderEventID != "" {
		eventIDs = append(eventIDs, candidate.LastProviderEventID)
	}
	return entitlement, EntitlementReconciliation{SellerID: candidate.SellerID, SourceRevision: revision, Source: candidate.Source, TriggeringEventIDs: eventIDs, SnapshotHash: hash, ReconciledAt: reconciledAt, AppliedVersion: version}, nil
}

func sellerPlanResponse(entitlement SellerEntitlement, now domain.Timestamp) (SellerPlanResponse, error) {
	plan, exists := planCatalogV1[entitlement.PlanID()]
	if !exists || plan.PlanVersion != entitlement.PlanVersion() {
		return SellerPlanResponse{}, ErrPlanNotFound
	}
	return SellerPlanResponse{Assignment: sellerEntitlementView(entitlement, now), Plan: plan}, nil
}

func sellerEntitlementView(entitlement SellerEntitlement, now domain.Timestamp) SellerEntitlementView {
	networkAccess := NetworkAccessBlocked
	if entitlement.AllowsNetworkAccess(now) {
		networkAccess = NetworkAccessEnabled
	}
	dashboardAccess := DashboardAccessRecoveryReadOnly
	if entitlement.Status() == EntitlementStatusActive && now.Before(entitlement.AccessEndsAt()) {
		dashboardAccess = DashboardAccessFull
	} else if entitlement.Status() == EntitlementStatusClosed {
		dashboardAccess = DashboardAccessClosed
	}
	var reason *EntitlementStatusReason
	if entitlement.StatusReason() != "" {
		value := entitlement.StatusReason()
		reason = &value
	}
	return SellerEntitlementView{
		SellerID: entitlement.SellerID(), PlanID: entitlement.PlanID(), PlanVersion: entitlement.PlanVersion(), Status: entitlement.Status(),
		BillingPeriodStart: entitlement.BillingPeriodStart(), BillingPeriodEnd: entitlement.BillingPeriodEnd(), AccessEndsAt: entitlement.AccessEndsAt(), GraceEndsAt: entitlement.GraceEndsAt(),
		CancelAtPeriodEnd: entitlement.CancelAtPeriodEnd(), EntitlementEpoch: entitlement.EntitlementEpoch(), Source: entitlement.Source(), SourceRevision: entitlement.SourceRevision(),
		StatusReason: reason, CredentialRotationRequired: entitlement.CredentialRotationRequired(), NetworkAccess: networkAccess, DashboardAccess: dashboardAccess,
		Provider: entitlement.Provider(), LastReconciledAt: entitlement.LastReconciledAt(), AssignedAt: entitlement.AssignedAt(), UpdatedAt: entitlement.UpdatedAt(), Version: entitlement.Version(),
	}
}

func candidateFromEntitlement(entitlement SellerEntitlement) EntitlementCandidate {
	return EntitlementCandidate{SellerID: entitlement.SellerID(), PlanID: entitlement.PlanID(), PlanVersion: entitlement.PlanVersion(), Status: entitlement.Status(), BillingPeriodStart: entitlement.BillingPeriodStart(), BillingPeriodEnd: entitlement.BillingPeriodEnd(), AccessEndsAt: entitlement.AccessEndsAt(), CancelAtPeriodEnd: entitlement.CancelAtPeriodEnd(), Source: entitlement.Source(), StatusReason: entitlement.StatusReason(), Provider: entitlement.Provider(), ProviderCustomerID: entitlement.ProviderCustomerID(), ProviderSubscriptionID: entitlement.ProviderSubscriptionID(), ProviderPriceID: entitlement.ProviderPriceID(), LastProviderEventID: entitlement.LastProviderEventID()}
}

func normalizeCandidateAtBoundary(candidate *EntitlementCandidate, now domain.Timestamp) {
	if candidate.Status == EntitlementStatusActive && !now.Before(candidate.AccessEndsAt) {
		switch {
		case candidate.CancelAtPeriodEnd:
			candidate.Status = EntitlementStatusCancelled
			candidate.StatusReason = EntitlementStatusReasonCancelled
		case candidate.StatusReason == EntitlementStatusReasonPaymentFailed || candidate.StatusReason == EntitlementStatusReasonPaymentActionRequired:
			if now.Before(candidate.AccessEndsAt.Add(EntitlementGraceDuration)) {
				candidate.Status = EntitlementStatusGrace
			} else {
				candidate.Status = EntitlementStatusSuspended
				candidate.StatusReason = EntitlementStatusReasonPaymentFailed
			}
		default:
			candidate.Status = EntitlementStatusSuspended
			candidate.StatusReason = EntitlementStatusReasonProviderIncomplete
		}
	}
	if candidate.Status == EntitlementStatusGrace && !now.Before(candidate.AccessEndsAt.Add(EntitlementGraceDuration)) {
		candidate.Status = EntitlementStatusSuspended
		candidate.StatusReason = EntitlementStatusReasonPaymentFailed
	}
}

func graceEndForCandidate(candidate EntitlementCandidate) *domain.Timestamp {
	if candidate.Status != EntitlementStatusGrace {
		return nil
	}
	value := candidate.AccessEndsAt.Add(EntitlementGraceDuration)
	return &value
}

func invalidatesCapabilities(current SellerEntitlement, candidate EntitlementCandidate) bool {
	if current.Status() != candidate.Status {
		return candidate.Status == EntitlementStatusGrace || candidate.Status == EntitlementStatusSuspended || candidate.Status == EntitlementStatusCancelled || candidate.Status == EntitlementStatusClosed || candidate.Status == EntitlementStatusActive
	}
	return candidate.Status == EntitlementStatusSuspended && candidate.StatusReason == EntitlementStatusReasonFraudQuarantine && current.StatusReason() != EntitlementStatusReasonFraudQuarantine
}

func nextSourceRevision(current *SellerEntitlement) (string, error) {
	sequence := uint64(1)
	if current != nil {
		parsed, err := strconv.ParseUint(current.SourceRevision(), 10, 64)
		if err != nil || parsed == ^uint64(0) {
			return "", domain.NewValidationError("sourceRevision", "sequence", "stored source revision is invalid")
		}
		sequence = parsed + 1
	}
	return fmt.Sprintf("%0*d", sourceRevisionWidth, sequence), nil
}

func entitlementSnapshotHash(snapshot SellerEntitlementSnapshot) (string, error) {
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validEntitlementStatus(status EntitlementStatus) bool {
	return status == EntitlementStatusActive || status == EntitlementStatusGrace || status == EntitlementStatusSuspended || status == EntitlementStatusCancelled || status == EntitlementStatusClosed
}

func validEntitlementSource(source EntitlementSource) bool {
	return source == EntitlementSourceBillingProvider || source == EntitlementSourceOperator || source == EntitlementSourceLocal
}

func validEntitlementProvider(provider EntitlementProvider) bool {
	return provider == EntitlementProviderStripe || provider == EntitlementProviderOperator || provider == EntitlementProviderLocal
}

func validSourceRevision(revision string) bool {
	if len(revision) != sourceRevisionWidth {
		return false
	}
	_, err := strconv.ParseUint(revision, 10, 64)
	return err == nil && revision != strings.Repeat("0", sourceRevisionWidth)
}

func normalizePlanID(planID PlanID) PlanID { return PlanID(strings.TrimSpace(string(planID))) }

type noopEntitlementInvalidator struct{}

func (noopEntitlementInvalidator) InvalidateSellerEntitlement(context.Context, domain.ID) error {
	return nil
}
