package authorization

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/gowebpki/jcs"
)

const (
	confirmationGrantTokenVersion    = "mcg1"
	MaximumConfirmationGrantLifetime = 5 * time.Minute
	minimumConfirmationSummaryLength = 10
	maximumConfirmationSummaryLength = 500
)

var (
	ErrConfirmationDenied   = errors.New("MCP confirmation grant is not valid for this mutation")
	ErrConfirmationReplayed = errors.New("MCP confirmation grant was already consumed")
)

type ConfirmationTool string

const (
	ToolConfigureStorefront ConfirmationTool = "configure_storefront"
	ToolConfigureRoute      ConfirmationTool = "configure_route"
	ToolChangeRoutePrice    ConfirmationTool = "change_route_price"
	ToolPublishRoute        ConfirmationTool = "publish_route"
)

type ConfirmationTargetType string

const (
	ConfirmationTargetSeller    ConfirmationTargetType = "seller"
	ConfirmationTargetPaidRoute ConfirmationTargetType = "paid_route"
)

type CreateConfirmationGrantRequest struct {
	CredentialID            domain.ID              `json:"credentialId"`
	Tool                    ConfirmationTool       `json:"tool"`
	TargetType              ConfirmationTargetType `json:"targetType"`
	TargetID                domain.ID              `json:"targetId"`
	ArgumentsSHA256         string                 `json:"argumentsSha256"`
	ExpectedResourceVersion uint64                 `json:"expectedResourceVersion"`
	Summary                 string                 `json:"summary"`
}

type ConfirmationGrantCreated struct {
	ConfirmationGrantID     domain.ID              `json:"confirmationGrantId"`
	ConfirmationGrant       string                 `json:"confirmationGrant"`
	SellerID                domain.ID              `json:"sellerId"`
	CredentialID            domain.ID              `json:"credentialId"`
	Tool                    ConfirmationTool       `json:"tool"`
	TargetType              ConfirmationTargetType `json:"targetType"`
	TargetID                domain.ID              `json:"targetId"`
	ArgumentsSHA256         string                 `json:"argumentsSha256"`
	ExpectedResourceVersion uint64                 `json:"expectedResourceVersion"`
	IssuedAt                domain.Timestamp       `json:"issuedAt"`
	ExpiresAt               domain.Timestamp       `json:"expiresAt"`
}

type ConfirmationGrant struct {
	ConfirmationGrantID          domain.ID              `json:"confirmationGrantId"`
	SellerID                     domain.ID              `json:"sellerId"`
	CredentialID                 domain.ID              `json:"credentialId"`
	Tool                         ConfirmationTool       `json:"tool"`
	TargetType                   ConfirmationTargetType `json:"targetType"`
	TargetID                     domain.ID              `json:"targetId"`
	ArgumentsSHA256              string                 `json:"argumentsSha256"`
	ExpectedResourceVersion      uint64                 `json:"expectedResourceVersion"`
	Summary                      string                 `json:"summary"`
	TokenDigest                  string                 `json:"tokenDigest"`
	BindingHash                  string                 `json:"bindingHash"`
	IssuedBySellerPrincipal      string                 `json:"issuedBySellerPrincipal"`
	IssuedAt                     domain.Timestamp       `json:"issuedAt"`
	ExpiresAt                    domain.Timestamp       `json:"expiresAt"`
	ConsumedAt                   *domain.Timestamp      `json:"consumedAt"`
	ConsumedByIdempotencyKeyHash string                 `json:"consumedByIdempotencyKeyHash"`
	RevokedAt                    *domain.Timestamp      `json:"revokedAt"`
	Version                      uint64                 `json:"version"`
}

type ConfirmationConsumption struct {
	ConfirmationGrant       string
	Tool                    ConfirmationTool
	TargetType              ConfirmationTargetType
	TargetID                domain.ID
	ArgumentsSHA256         string
	ExpectedResourceVersion uint64
	IdempotencyKey          string
}

type ConfirmationGrantRepository interface {
	CreateReplacing(context.Context, ConfirmationGrant) error
	Get(context.Context, domain.ID) (ConfirmationGrant, error)
	Consume(context.Context, ConfirmationGrant, uint64) error
}

type ConfirmationSellerAuthorizer interface {
	AuthorizeSeller(context.Context, string, domain.ID) error
}

type ConfirmationTargetReader interface {
	GetSeller(context.Context, domain.ID) (catalog.Seller, error)
	GetRoute(context.Context, domain.ID) (catalog.PaidRoute, error)
}

type ConfirmationGrantService struct {
	repository       ConfirmationGrantRepository
	sellerAuthorizer ConfirmationSellerAuthorizer
	credentials      CredentialReader
	entitlements     EntitlementReader
	targets          ConfirmationTargetReader
	idGenerator      domain.IDGenerator
	tokenGenerator   integrations.TokenGenerator
	digester         integrations.CredentialDigester
	clock            domain.Clock
	auditRecorder    audit.Recorder
}

func NewConfirmationGrantService(
	repository ConfirmationGrantRepository,
	sellerAuthorizer ConfirmationSellerAuthorizer,
	credentials CredentialReader,
	entitlements EntitlementReader,
	targets ConfirmationTargetReader,
	idGenerator domain.IDGenerator,
	tokenGenerator integrations.TokenGenerator,
	digester integrations.CredentialDigester,
	clock domain.Clock,
	auditRecorder audit.Recorder,
) *ConfirmationGrantService {
	return &ConfirmationGrantService{
		repository: repository, sellerAuthorizer: sellerAuthorizer, credentials: credentials,
		entitlements: entitlements, targets: targets, idGenerator: idGenerator, tokenGenerator: tokenGenerator,
		digester: digester, clock: clock, auditRecorder: auditRecorder,
	}
}

func (service *ConfirmationGrantService) Create(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	request CreateConfirmationGrantRequest,
) (ConfirmationGrantCreated, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return ConfirmationGrantCreated{}, err
	}
	if err := validateConfirmationRequest(sellerID, request); err != nil {
		return ConfirmationGrantCreated{}, err
	}
	if err := service.validateTarget(ctx, sellerID, request); err != nil {
		return ConfirmationGrantCreated{}, err
	}
	credential, err := service.credentials.GetByID(ctx, request.CredentialID)
	if err != nil {
		return ConfirmationGrantCreated{}, err
	}
	now := domain.NewTimestamp(service.clock.Now())
	if credential.SellerID() != sellerID || credential.RevokedAt() != nil ||
		(credential.ExpiresAt() != nil && !now.Before(*credential.ExpiresAt())) ||
		!credential.HasScope(requiredToolScope(request.Tool)) {
		return ConfirmationGrantCreated{}, ErrConfirmationDenied
	}
	entitlement, err := service.entitlements.ResolveSellerPlan(ctx, sellerID)
	if errors.Is(err, billing.ErrSellerEntitlementNotFound) || errors.Is(err, persistence.ErrNotFound) {
		return ConfirmationGrantCreated{}, ErrSubscriptionInactive
	}
	if err != nil {
		return ConfirmationGrantCreated{}, ErrAuthorizationUnavailable
	}
	if entitlement.Assignment.Status != billing.EntitlementStatusActive || !now.Before(entitlement.Assignment.AccessEndsAt) ||
		credential.EntitlementEpoch() != entitlement.Assignment.EntitlementEpoch {
		return ConfirmationGrantCreated{}, ErrSubscriptionInactive
	}
	grantID, err := service.idGenerator.New(domain.MCPConfirmationGrantIDPrefix)
	if err != nil {
		return ConfirmationGrantCreated{}, err
	}
	secret, err := service.tokenGenerator.NewToken()
	if err != nil {
		return ConfirmationGrantCreated{}, err
	}
	decodedSecret, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(decodedSecret) < 32 {
		return ConfirmationGrantCreated{}, errors.New("confirmation grant generator returned insufficient entropy")
	}
	rawGrant := confirmationGrantToken(grantID, secret)
	digest, err := service.digester.Digest(ctx, rawGrant)
	if err != nil {
		return ConfirmationGrantCreated{}, err
	}
	grant := ConfirmationGrant{
		ConfirmationGrantID: grantID, SellerID: sellerID, CredentialID: request.CredentialID,
		Tool: request.Tool, TargetType: request.TargetType, TargetID: request.TargetID,
		ArgumentsSHA256: request.ArgumentsSHA256, ExpectedResourceVersion: request.ExpectedResourceVersion,
		Summary: strings.TrimSpace(request.Summary), TokenDigest: digest,
		BindingHash: confirmationBindingHash(sellerID, request), IssuedBySellerPrincipal: ownerSubject,
		IssuedAt: now, ExpiresAt: now.Add(MaximumConfirmationGrantLifetime), Version: 1,
	}
	if err := service.repository.CreateReplacing(ctx, grant); err != nil {
		return ConfirmationGrantCreated{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID: sellerID, ActorType: audit.ActorTypeSellerUser, ActorID: ownerSubject,
		Action: audit.ActionMCPConfirmationIssued, TargetType: audit.TargetTypeMCPConfirmationGrant,
		TargetID: grantID.String(), Outcome: audit.OutcomeSucceeded,
		ChangedFields: []string{"credentialId", "tool", "targetType", "targetId", "argumentsSha256", "expectedResourceVersion", "expiresAt"},
	}); err != nil {
		return ConfirmationGrantCreated{}, err
	}
	return ConfirmationGrantCreated{
		ConfirmationGrantID: grantID, ConfirmationGrant: rawGrant, SellerID: sellerID,
		CredentialID: request.CredentialID, Tool: request.Tool, TargetType: request.TargetType,
		TargetID: request.TargetID, ArgumentsSHA256: request.ArgumentsSHA256,
		ExpectedResourceVersion: request.ExpectedResourceVersion, IssuedAt: now, ExpiresAt: grant.ExpiresAt,
	}, nil
}

func (service *ConfirmationGrantService) Consume(
	ctx context.Context,
	principal integrations.Principal,
	request ConfirmationConsumption,
) error {
	grantID, err := parseConfirmationGrantToken(request.ConfirmationGrant)
	if err != nil {
		return ErrConfirmationDenied
	}
	grant, err := service.repository.Get(ctx, grantID)
	if errors.Is(err, persistence.ErrNotFound) {
		return ErrConfirmationDenied
	}
	if err != nil {
		return fmt.Errorf("load confirmation grant: %w", err)
	}
	digest, err := service.digester.Digest(ctx, request.ConfirmationGrant)
	if err != nil {
		return fmt.Errorf("digest confirmation grant: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(digest), []byte(grant.TokenDigest)) != 1 ||
		grant.SellerID != principal.SellerID || grant.CredentialID != principal.CredentialID ||
		grant.Tool != request.Tool || grant.TargetType != request.TargetType || grant.TargetID != request.TargetID ||
		grant.ArgumentsSHA256 != request.ArgumentsSHA256 || grant.ExpectedResourceVersion != request.ExpectedResourceVersion ||
		grant.RevokedAt != nil || !service.clock.Now().UTC().Before(grant.ExpiresAt.Time()) {
		return ErrConfirmationDenied
	}
	idempotencyKey, err := domain.ParseIdempotencyKey(request.IdempotencyKey)
	if err != nil {
		return err
	}
	idempotencyDigest := sha256.Sum256([]byte(principal.CredentialID.String() + "\x00" + string(idempotencyKey)))
	idempotencyHash := hex.EncodeToString(idempotencyDigest[:])
	if grant.ConsumedAt != nil {
		if grant.ConsumedByIdempotencyKeyHash == idempotencyHash {
			return nil
		}
		return ErrConfirmationReplayed
	}
	now := domain.NewTimestamp(service.clock.Now())
	expectedVersion := grant.Version
	grant.ConsumedAt = &now
	grant.ConsumedByIdempotencyKeyHash = idempotencyHash
	grant.Version++
	if err := service.repository.Consume(ctx, grant, expectedVersion); err != nil {
		if errors.Is(err, persistence.ErrConditionFailed) {
			latest, loadErr := service.repository.Get(ctx, grantID)
			if loadErr == nil && latest.ConsumedByIdempotencyKeyHash == idempotencyHash {
				return nil
			}
			if loadErr == nil && latest.ConsumedAt == nil {
				return ErrConfirmationDenied
			}
			return ErrConfirmationReplayed
		}
		return fmt.Errorf("consume confirmation grant: %w", err)
	}
	return service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID: principal.SellerID, ActorType: audit.ActorTypeIntegrationCredential,
		ActorID: principal.CredentialID.String(), Action: audit.ActionMCPConfirmationConsumed,
		TargetType: audit.TargetTypeMCPConfirmationGrant, TargetID: grantID.String(),
		Outcome: audit.OutcomeSucceeded, ChangedFields: []string{"consumedAt", "consumedByIdempotencyKeyHash"},
	})
}

func validateConfirmationRequest(sellerID domain.ID, request CreateConfirmationGrantRequest) error {
	if request.CredentialID.Prefix() != domain.CredentialIDPrefix {
		return domain.NewValidationError("credentialId", "format", "must identify an integration credential")
	}
	if request.ExpectedResourceVersion == 0 {
		return domain.NewValidationError("expectedResourceVersion", "minimum", "must be at least one")
	}
	if len(request.ArgumentsSHA256) != 64 {
		return domain.NewValidationError("argumentsSha256", "format", "must be a lowercase SHA-256 digest")
	}
	if _, err := hex.DecodeString(request.ArgumentsSHA256); err != nil || request.ArgumentsSHA256 != strings.ToLower(request.ArgumentsSHA256) {
		return domain.NewValidationError("argumentsSha256", "format", "must be a lowercase SHA-256 digest")
	}
	summary := strings.TrimSpace(request.Summary)
	if len(summary) < minimumConfirmationSummaryLength || len(summary) > maximumConfirmationSummaryLength {
		return domain.NewValidationError("summary", "length", "must contain 10-500 characters")
	}
	switch request.Tool {
	case ToolConfigureStorefront, ToolConfigureRoute:
		if request.TargetType != ConfirmationTargetSeller || request.TargetID != sellerID {
			return ErrConfirmationDenied
		}
	case ToolChangeRoutePrice, ToolPublishRoute:
		if request.TargetType != ConfirmationTargetPaidRoute || request.TargetID.Prefix() != domain.RouteIDPrefix {
			return ErrConfirmationDenied
		}
	default:
		return domain.NewValidationError("tool", "enum", "is not a confirmable MCP mutation")
	}
	return nil
}

func (service *ConfirmationGrantService) validateTarget(ctx context.Context, sellerID domain.ID, request CreateConfirmationGrantRequest) error {
	if service.targets == nil {
		return ErrAuthorizationUnavailable
	}
	if request.TargetType == ConfirmationTargetSeller {
		seller, err := service.targets.GetSeller(ctx, sellerID)
		if err != nil {
			return err
		}
		if seller.SellerID != sellerID || seller.Version != request.ExpectedResourceVersion {
			return persistence.ErrConditionFailed
		}
		return nil
	}
	route, err := service.targets.GetRoute(ctx, request.TargetID)
	if err != nil {
		return err
	}
	if route.SellerID != sellerID {
		return ErrConfirmationDenied
	}
	if route.Version != request.ExpectedResourceVersion {
		return persistence.ErrConditionFailed
	}
	return nil
}

func requiredToolScope(tool ConfirmationTool) integrations.Scope {
	if tool == ToolPublishRoute {
		return integrations.ScopePublish
	}
	return integrations.ScopeConfigure
}

func confirmationBindingHash(sellerID domain.ID, request CreateConfirmationGrantRequest) string {
	payload := strings.Join([]string{
		sellerID.String(), request.CredentialID.String(), string(request.Tool), string(request.TargetType),
		request.TargetID.String(), request.ArgumentsSHA256,
		strconv.FormatUint(request.ExpectedResourceVersion, 10),
	}, "\x00")
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])
}

func confirmationGrantToken(grantID domain.ID, secret string) string {
	return confirmationGrantTokenVersion + "." + grantID.String() + "." + secret
}

func parseConfirmationGrantToken(raw string) (domain.ID, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 || parts[0] != confirmationGrantTokenVersion {
		return "", ErrConfirmationDenied
	}
	secret, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(secret) < 32 {
		return "", ErrConfirmationDenied
	}
	return domain.ParseID(parts[1], domain.MCPConfirmationGrantIDPrefix)
}

// HashMutationArguments returns the contract-defined digest after removing
// transport-only replay and confirmation secrets.
func HashMutationArguments(arguments any) (string, error) {
	encoded, err := json.Marshal(arguments)
	if err != nil {
		return "", err
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		return "", err
	}
	delete(object, "idempotencyKey")
	delete(object, "confirmationGrant")
	encoded, err = json.Marshal(object)
	if err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(encoded)
	if err != nil {
		return "", domain.NewValidationError("arguments", "canonical", "must contain canonicalizable JSON values")
	}
	hashInput := append([]byte("agentpay.mcp-mutation.v1\x00"), canonical...)
	digest := sha256.Sum256(hashInput)
	return hex.EncodeToString(digest[:]), nil
}
