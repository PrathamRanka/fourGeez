package integrations

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const (
	credentialTokenVersion                   = "apc2"
	minimumCredentialSecret                  = 32
	maximumCredentialLabel                   = 80
	credentialRotationReplayWindow           = 10 * time.Minute
	DefaultProjectKeyExchangeAttempts uint64 = 10
	DefaultProjectKeyExchangeWindow          = time.Minute
)

var (
	ErrCredentialInvalid                = errors.New("integration credential is invalid")
	ErrCredentialExpired                = errors.New("integration credential has expired")
	ErrCredentialRevoked                = errors.New("integration credential has been revoked")
	ErrScopeDenied                      = errors.New("integration credential scope denied")
	ErrSubscriptionInactive             = errors.New("seller subscription is inactive")
	ErrExchangeAuthorizationUnavailable = errors.New("project-key exchange authorization is unavailable")
	ErrRotationIdempotencyConflict      = errors.New("credential rotation idempotency key was reused with a different request")
)

// RotationRecoveryError reports a committed rotation whose secret can no
// longer be recovered. The successor must itself be rotated.
type RotationRecoveryError struct {
	SuccessorCredentialID domain.ID
}

func (err RotationRecoveryError) Error() string {
	return "credential rotation response is no longer recoverable"
}

// LocalRotationReplayProtector provides process-local authenticated encryption
// for disposable development. Production injects a KMS envelope protector.
type LocalRotationReplayProtector struct {
	aead cipher.AEAD
}

func NewLocalRotationReplayProtector(key []byte) (*LocalRotationReplayProtector, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &LocalRotationReplayProtector{aead: aead}, nil
}

func (protector *LocalRotationReplayProtector) Seal(_ context.Context, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, protector.aead.NonceSize())
	if _, err := io.ReadFull(cryptorand.Reader, nonce); err != nil {
		return nil, err
	}
	protected := make([]byte, 1, 1+len(nonce)+len(plaintext)+protector.aead.Overhead())
	protected[0] = 1
	protected = append(protected, nonce...)
	return protector.aead.Seal(protected, nonce, plaintext, nil), nil
}

func (protector *LocalRotationReplayProtector) Open(_ context.Context, protected []byte) ([]byte, error) {
	if len(protected) < 1+protector.aead.NonceSize()+protector.aead.Overhead() || protected[0] != 1 {
		return nil, errors.New("credential rotation replay material is invalid")
	}
	nonceEnd := 1 + protector.aead.NonceSize()
	return protector.aead.Open(nil, protected[1:nonceEnd], protected[nonceEnd:], nil)
}

type StaticCredentialPepperProvider struct{ Value []byte }

func (provider StaticCredentialPepperProvider) CredentialPepper(context.Context) ([]byte, error) {
	return append([]byte(nil), provider.Value...), nil
}

type HMACCredentialDigester struct{ pepperProvider CredentialPepperProvider }

func NewHMACCredentialDigester(provider CredentialPepperProvider) *HMACCredentialDigester {
	return &HMACCredentialDigester{pepperProvider: provider}
}

func (digester *HMACCredentialDigester) Digest(ctx context.Context, rawToken string) (string, error) {
	pepper, err := digester.pepperProvider.CredentialPepper(ctx)
	if err != nil {
		return "", err
	}
	if len(pepper) < sha256.Size {
		return "", errors.New("credential pepper must contain at least 256 bits")
	}
	digest := hmac.New(sha256.New, pepper)
	_, _ = digest.Write([]byte(rawToken))
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// SecureTokenGenerator creates URL-safe credential secret material.
type SecureTokenGenerator struct {
	reader io.Reader
}

// NewSecureTokenGenerator uses crypto/rand when no reader is supplied.
func NewSecureTokenGenerator(reader io.Reader) *SecureTokenGenerator {
	if reader == nil {
		reader = cryptorand.Reader
	}
	return &SecureTokenGenerator{reader: reader}
}

// NewToken returns 256 bits of URL-safe random credential material.
func (generator *SecureTokenGenerator) NewToken() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := io.ReadFull(generator.reader, randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

// Service owns integration-credential issuance and authentication rules.
type Service struct {
	repository              Repository
	sellerAuthorizer        SellerAuthorizer
	idGenerator             domain.IDGenerator
	tokenGenerator          TokenGenerator
	clock                   domain.Clock
	auditRecorder           audit.Recorder
	credentialDigester      CredentialDigester
	entitlementResolver     EntitlementResolver
	exchangeRateLimiter     ExchangeRateLimiter
	exchangeQuotaEnforcer   ExchangeQuotaEnforcer
	rotationReplayProtector RotationReplayProtector
}

type ServiceOption func(*Service)

func WithCredentialDigester(digester CredentialDigester) ServiceOption {
	return func(service *Service) { service.credentialDigester = digester }
}

func WithExchangeAuthorization(resolver EntitlementResolver, limiter ExchangeRateLimiter, quota ExchangeQuotaEnforcer) ServiceOption {
	return func(service *Service) {
		service.entitlementResolver = resolver
		service.exchangeRateLimiter = limiter
		service.exchangeQuotaEnforcer = quota
	}
}

func WithRotationReplayProtector(protector RotationReplayProtector) ServiceOption {
	return func(service *Service) { service.rotationReplayProtector = protector }
}

// NewService creates the integration credential application service.
func NewService(
	repository Repository,
	sellerAuthorizer SellerAuthorizer,
	idGenerator domain.IDGenerator,
	tokenGenerator TokenGenerator,
	clock domain.Clock,
	auditRecorder audit.Recorder,
	options ...ServiceOption,
) *Service {
	localPepper := make([]byte, sha256.Size)
	_, _ = io.ReadFull(cryptorand.Reader, localPepper)
	localReplayKey := make([]byte, 32)
	_, _ = io.ReadFull(cryptorand.Reader, localReplayKey)
	localReplayProtector, _ := NewLocalRotationReplayProtector(localReplayKey)
	service := &Service{
		repository:              repository,
		sellerAuthorizer:        sellerAuthorizer,
		idGenerator:             idGenerator,
		tokenGenerator:          tokenGenerator,
		clock:                   clock,
		auditRecorder:           auditRecorder,
		credentialDigester:      NewHMACCredentialDigester(StaticCredentialPepperProvider{Value: localPepper}),
		rotationReplayProtector: localReplayProtector,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

// Create issues one seller-scoped credential and returns its token once.
func (service *Service) Create(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	request CreateCredentialRequest,
) (CredentialCreated, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(
		ctx,
		ownerSubject,
		sellerID,
	); err != nil {
		return CredentialCreated{}, err
	}

	credentialID, err := service.idGenerator.New(domain.CredentialIDPrefix)
	if err != nil {
		return CredentialCreated{}, err
	}
	secret, err := service.tokenGenerator.NewToken()
	if err != nil {
		return CredentialCreated{}, err
	}
	if !validCredentialSecret(secret) {
		return CredentialCreated{}, errors.New("credential token generator returned insufficient entropy")
	}

	rawToken := credentialToken(credentialID, secret)
	tokenDigest, err := service.credentialDigester.Digest(ctx, rawToken)
	if err != nil {
		return CredentialCreated{}, err
	}
	createdAt := domain.NewTimestamp(service.clock.Now())
	entitlementEpoch, err := service.credentialIssuanceEpoch(ctx, sellerID, createdAt)
	if err != nil {
		return CredentialCreated{}, err
	}
	credential, err := NewCredential(CredentialParams{
		CredentialID:     credentialID,
		SellerID:         sellerID,
		TokenHash:        tokenDigest,
		Label:            request.Label,
		Scopes:           request.Scopes,
		EntitlementEpoch: entitlementEpoch,
		ExpiresAt:        request.ExpiresAt,
		CreatedAt:        createdAt,
	})
	if err != nil {
		return CredentialCreated{}, err
	}
	if err := service.repository.Create(ctx, credential); err != nil {
		return CredentialCreated{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID:   sellerID,
		ActorType:  audit.ActorTypeSellerUser,
		ActorID:    ownerSubject,
		Action:     audit.ActionCredentialCreated,
		TargetType: audit.TargetTypeIntegrationCredential,
		TargetID:   credential.CredentialID().String(),
		Outcome:    audit.OutcomeSucceeded,
		ChangedFields: []string{
			"label",
			"scopes",
			"expiresAt",
		},
	}); err != nil {
		return CredentialCreated{}, err
	}

	return CredentialCreated{
		CredentialView: credentialView(credential),
		Token:          rawToken,
	}, nil
}

func (service *Service) Rotate(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	credentialID domain.ID,
	request RotateCredentialRequest,
	idempotency RotationIdempotency,
) (CredentialRotation, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(ctx, ownerSubject, sellerID); err != nil {
		return CredentialRotation{}, err
	}
	replayKey, err := domain.ParseIdempotencyKey(idempotency.Key)
	if err != nil {
		return CredentialRotation{}, err
	}
	requestDigest := sha256.Sum256(idempotency.RequestBody)
	requestHash := hex.EncodeToString(requestDigest[:])
	replayScope := credentialRotationReplayScope(ownerSubject, sellerID, credentialID)
	now := domain.NewTimestamp(service.clock.Now())
	if replay, found, loadErr := service.repository.LoadRotationReplay(ctx, replayScope, replayKey); loadErr != nil {
		return CredentialRotation{}, loadErr
	} else if found {
		return service.replayCredentialRotation(ctx, replay, requestHash, now)
	}
	predecessor, err := service.repository.Get(ctx, sellerID, credentialID)
	if err != nil {
		return CredentialRotation{}, err
	}
	if predecessor.Version() != request.ExpectedVersion {
		return CredentialRotation{}, persistence.ErrConditionFailed
	}
	if predecessor.RevokedAt() != nil {
		if successorID := predecessor.ReplacedByCredentialID(); successorID != nil {
			return CredentialRotation{}, RotationRecoveryError{SuccessorCredentialID: *successorID}
		}
		return CredentialRotation{}, ErrCredentialRevoked
	}
	successorID, err := service.idGenerator.New(domain.CredentialIDPrefix)
	if err != nil {
		return CredentialRotation{}, err
	}
	secret, err := service.tokenGenerator.NewToken()
	if err != nil {
		return CredentialRotation{}, err
	}
	if !validCredentialSecret(secret) {
		return CredentialRotation{}, errors.New("credential token generator returned insufficient entropy")
	}
	rawToken := credentialToken(successorID, secret)
	tokenDigest, err := service.credentialDigester.Digest(ctx, rawToken)
	if err != nil {
		return CredentialRotation{}, err
	}
	label := strings.TrimSpace(request.Label)
	if label == "" {
		label = predecessor.Label()
	}
	scopes := request.Scopes
	if len(scopes) == 0 {
		scopes = predecessor.Scopes()
	}
	expiresAt := request.ExpiresAt
	if expiresAt == nil {
		expiresAt = predecessor.ExpiresAt()
	}
	entitlementEpoch, err := service.credentialIssuanceEpoch(ctx, sellerID, now)
	if err != nil {
		return CredentialRotation{}, err
	}
	successor, err := NewCredential(CredentialParams{
		CredentialID: successorID, SellerID: sellerID, TokenHash: tokenDigest,
		Label: label, Scopes: scopes, EntitlementEpoch: entitlementEpoch, ExpiresAt: expiresAt, CreatedAt: now,
	})
	if err != nil {
		return CredentialRotation{}, err
	}
	if err := predecessor.Replace(successorID, now); err != nil {
		return CredentialRotation{}, err
	}
	rotation := CredentialRotation{
		Predecessor: credentialView(predecessor), Successor: credentialView(successor), Token: rawToken,
		SecretReplayExpiresAt: now.Add(credentialRotationReplayWindow),
	}
	encodedRotation, err := json.Marshal(rotation)
	if err != nil {
		return CredentialRotation{}, err
	}
	protectedRotation, err := service.rotationReplayProtector.Seal(ctx, encodedRotation)
	if err != nil {
		return CredentialRotation{}, err
	}
	replay := RotationReplay{
		Scope: replayScope, Key: replayKey, RequestHash: requestHash,
		ProtectedResponse: protectedRotation, SuccessorCredentialID: successorID,
		CreatedAt: now, ExpiresAt: rotation.SecretReplayExpiresAt,
	}
	if err := service.repository.Rotate(ctx, predecessor, successor, request.ExpectedVersion, replay); err != nil {
		if errors.Is(err, persistence.ErrConditionFailed) {
			if committed, found, loadErr := service.repository.LoadRotationReplay(ctx, replayScope, replayKey); loadErr != nil {
				return CredentialRotation{}, loadErr
			} else if found {
				return service.replayCredentialRotation(ctx, committed, requestHash, now)
			}
		}
		return CredentialRotation{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID: sellerID, ActorType: audit.ActorTypeSellerUser, ActorID: ownerSubject,
		Action: audit.ActionCredentialRotated, TargetType: audit.TargetTypeIntegrationCredential,
		TargetID: credentialID.String(), Outcome: audit.OutcomeSucceeded,
		ChangedFields: []string{"revokedAt", "replacedByCredentialId"},
	}); err != nil {
		return CredentialRotation{}, err
	}
	return rotation, nil
}

func (service *Service) replayCredentialRotation(
	ctx context.Context,
	replay RotationReplay,
	requestHash string,
	now domain.Timestamp,
) (CredentialRotation, error) {
	if replay.RequestHash != requestHash {
		return CredentialRotation{}, ErrRotationIdempotencyConflict
	}
	if !now.Time().Before(replay.ExpiresAt.Time()) {
		return CredentialRotation{}, RotationRecoveryError{SuccessorCredentialID: replay.SuccessorCredentialID}
	}
	encodedRotation, err := service.rotationReplayProtector.Open(ctx, replay.ProtectedResponse)
	if err != nil {
		return CredentialRotation{}, err
	}
	var rotation CredentialRotation
	if err := json.Unmarshal(encodedRotation, &rotation); err != nil {
		return CredentialRotation{}, err
	}
	return rotation, nil
}

func credentialRotationReplayScope(ownerSubject string, sellerID, credentialID domain.ID) string {
	return ownerSubject + ":rotateIntegrationCredential:" + sellerID.String() + ":" + credentialID.String()
}

// List returns redacted credentials after checking seller ownership.
func (service *Service) List(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
) ([]CredentialView, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(
		ctx,
		ownerSubject,
		sellerID,
	); err != nil {
		return nil, err
	}
	credentials, err := service.repository.ListBySeller(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	views := make([]CredentialView, len(credentials))
	for index, credential := range credentials {
		views[index] = credentialView(credential)
	}
	return views, nil
}

// Revoke immediately disables one credential using optimistic concurrency.
func (service *Service) Revoke(
	ctx context.Context,
	ownerSubject string,
	sellerID domain.ID,
	credentialID domain.ID,
	expectedVersion uint64,
) (CredentialView, error) {
	if err := service.sellerAuthorizer.AuthorizeSeller(
		ctx,
		ownerSubject,
		sellerID,
	); err != nil {
		return CredentialView{}, err
	}
	credential, err := service.repository.Get(ctx, sellerID, credentialID)
	if err != nil {
		return CredentialView{}, err
	}
	if err := credential.Revoke(
		domain.NewTimestamp(service.clock.Now()),
	); err != nil {
		return CredentialView{}, err
	}
	if err := service.repository.Update(
		ctx,
		credential,
		expectedVersion,
	); err != nil {
		return CredentialView{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID:   sellerID,
		ActorType:  audit.ActorTypeSellerUser,
		ActorID:    ownerSubject,
		Action:     audit.ActionCredentialRevoked,
		TargetType: audit.TargetTypeIntegrationCredential,
		TargetID:   credentialID.String(),
		Outcome:    audit.OutcomeSucceeded,
		ChangedFields: []string{
			"revokedAt",
		},
	}); err != nil {
		return CredentialView{}, err
	}
	return credentialView(credential), nil
}

// Authenticate verifies token binding, lifetime, revocation, and required scope.
func (service *Service) Authenticate(
	ctx context.Context,
	rawToken string,
	requiredScope Scope,
) (Principal, error) {
	principal, err := service.AuthenticateToken(ctx, rawToken)
	if err != nil {
		return Principal{}, err
	}
	if !principal.HasScope(requiredScope) {
		return Principal{}, ErrScopeDenied
	}
	return principal, nil
}

// AuthenticateToken verifies credential identity, lifetime, and revocation.
func (service *Service) AuthenticateToken(
	ctx context.Context,
	rawToken string,
) (Principal, error) {
	principal, _, err := service.authenticateCredential(ctx, rawToken)
	if err != nil {
		return Principal{}, err
	}
	if service.entitlementResolver != nil {
		credential, loadErr := service.repository.Get(ctx, principal.SellerID, principal.CredentialID)
		if loadErr != nil {
			return Principal{}, loadErr
		}
		if _, entitlementErr := service.authorizeCurrentEntitlement(ctx, credential, domain.NewTimestamp(service.clock.Now())); entitlementErr != nil {
			return Principal{}, entitlementErr
		}
	}
	return principal, nil
}

func (service *Service) authenticateCredential(ctx context.Context, rawToken string) (Principal, Credential, error) {
	credentialID, err := parseCredentialToken(rawToken)
	if err != nil {
		return Principal{}, Credential{}, ErrCredentialInvalid
	}
	credential, err := service.repository.GetByID(ctx, credentialID)
	if errors.Is(err, persistence.ErrNotFound) {
		return Principal{}, Credential{}, ErrCredentialInvalid
	}
	if err != nil {
		return Principal{}, Credential{}, err
	}
	providedDigest, err := service.credentialDigester.Digest(ctx, rawToken)
	if err != nil {
		return Principal{}, credential, err
	}
	storedDigest, storedErr := hex.DecodeString(credential.TokenHash())
	providedDigestBytes, providedErr := hex.DecodeString(providedDigest)
	if storedErr != nil || providedErr != nil || !hmac.Equal(storedDigest, providedDigestBytes) {
		return Principal{}, credential, ErrCredentialInvalid
	}
	if credential.RevokedAt() != nil {
		return Principal{}, credential, ErrCredentialRevoked
	}
	now := service.clock.Now()
	if expiresAt := credential.ExpiresAt(); expiresAt != nil && !now.Before(expiresAt.Time()) {
		return Principal{}, credential, ErrCredentialExpired
	}
	return Principal{
		SellerID:     credential.SellerID(),
		CredentialID: credentialID,
		Scopes:       credential.Scopes(),
	}, credential, nil
}

func (service *Service) AuthorizeExchange(ctx context.Context, rawToken string, requestedScopes []Scope) (ExchangeAuthorization, error) {
	credentialID, err := parseCredentialToken(rawToken)
	if err != nil {
		return ExchangeAuthorization{}, ErrCredentialInvalid
	}
	if service.exchangeRateLimiter == nil || service.entitlementResolver == nil || service.exchangeQuotaEnforcer == nil {
		return ExchangeAuthorization{}, ErrExchangeAuthorizationUnavailable
	}
	if err := service.exchangeRateLimiter.AllowProjectKeyExchange(ctx, credentialID); err != nil {
		return ExchangeAuthorization{}, err
	}
	principal, credential, err := service.authenticateCredential(ctx, rawToken)
	if err != nil {
		if credential.CredentialID() != "" {
			return ExchangeAuthorization{}, service.denyExchange(ctx, credential, err)
		}
		return ExchangeAuthorization{}, err
	}
	if len(requestedScopes) == 0 {
		return ExchangeAuthorization{}, service.denyExchange(ctx, credential, domain.NewValidationError("scopes", "required", "must contain at least one scope"))
	}
	seenScopes := make(map[Scope]struct{}, len(requestedScopes))
	for _, requestedScope := range requestedScopes {
		if !supportedExchangeScope(requestedScope) {
			return ExchangeAuthorization{}, service.denyExchange(ctx, credential, ErrScopeDenied)
		}
		if _, duplicate := seenScopes[requestedScope]; duplicate {
			return ExchangeAuthorization{}, service.denyExchange(ctx, credential, domain.NewValidationError("scopes", "unique", "must not contain duplicate scopes"))
		}
		seenScopes[requestedScope] = struct{}{}
		if !principal.HasScope(requestedScope) {
			return ExchangeAuthorization{}, service.denyExchange(ctx, credential, ErrScopeDenied)
		}
	}
	now := domain.NewTimestamp(service.clock.Now())
	entitlement, err := service.authorizeCurrentEntitlement(ctx, credential, now)
	if err != nil {
		return ExchangeAuthorization{}, service.denyExchange(ctx, credential, err)
	}
	if err := service.exchangeQuotaEnforcer.ConsumeAPIRequest(ctx, principal.SellerID); err != nil {
		return ExchangeAuthorization{}, err
	}
	expectedVersion := credential.Version()
	if err := credential.MarkUsed(now); err != nil {
		return ExchangeAuthorization{}, err
	}
	if err := service.repository.Update(ctx, credential, expectedVersion); err != nil {
		return ExchangeAuthorization{}, err
	}
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID: principal.SellerID, ActorType: audit.ActorTypeIntegrationCredential, ActorID: principal.CredentialID.String(),
		Action: audit.ActionCredentialExchangeSucceeded, TargetType: audit.TargetTypeIntegrationCredential,
		TargetID: principal.CredentialID.String(), Outcome: audit.OutcomeSucceeded, ChangedFields: []string{"lastUsedAt"},
	}); err != nil {
		return ExchangeAuthorization{}, err
	}
	principal.Scopes = append([]Scope(nil), requestedScopes...)
	return ExchangeAuthorization{Principal: principal, EntitlementEpoch: entitlement.Assignment.EntitlementEpoch}, nil
}

func (service *Service) authorizeCurrentEntitlement(ctx context.Context, credential Credential, now domain.Timestamp) (billing.SellerPlanResponse, error) {
	entitlement, err := service.entitlementResolver.ResolveSellerPlan(ctx, credential.SellerID())
	if errors.Is(err, billing.ErrSellerEntitlementNotFound) || errors.Is(err, persistence.ErrNotFound) {
		return billing.SellerPlanResponse{}, ErrSubscriptionInactive
	}
	if err != nil {
		return billing.SellerPlanResponse{}, err
	}
	if entitlement.Assignment.Status != billing.EntitlementStatusActive || !now.Before(entitlement.Assignment.AccessEndsAt) {
		return billing.SellerPlanResponse{}, ErrSubscriptionInactive
	}
	if credential.EntitlementEpoch() == 0 || credential.EntitlementEpoch() != entitlement.Assignment.EntitlementEpoch {
		return billing.SellerPlanResponse{}, ErrCredentialRevoked
	}
	return entitlement, nil
}

func (service *Service) denyExchange(ctx context.Context, credential Credential, denial error) error {
	if err := service.auditRecorder.Record(ctx, audit.RecordRequest{
		SellerID: credential.SellerID(), ActorType: audit.ActorTypeIntegrationCredential, ActorID: credential.CredentialID().String(),
		Action: audit.ActionCredentialExchangeDenied, TargetType: audit.TargetTypeIntegrationCredential,
		TargetID: credential.CredentialID().String(), Outcome: audit.OutcomeDenied, ChangedFields: []string{"authorization"},
	}); err != nil {
		return err
	}
	return denial
}

// NewCredential validates and creates persisted credential metadata.
func NewCredential(params CredentialParams) (Credential, error) {
	validationErrors := validateCredentialParams(params)
	if len(validationErrors) > 0 {
		return Credential{}, validationErrors
	}

	return Credential{
		credentialID:           params.CredentialID,
		sellerID:               params.SellerID,
		tokenHash:              params.TokenHash,
		label:                  strings.TrimSpace(params.Label),
		scopes:                 append([]Scope(nil), params.Scopes...),
		entitlementEpoch:       params.EntitlementEpoch,
		expiresAt:              copyTimestamp(params.ExpiresAt),
		lastUsedAt:             copyTimestamp(params.LastUsedAt),
		replacedByCredentialID: copyID(params.ReplacedByCredentialID),
		revokedAt:              copyTimestamp(params.RevokedAt),
		createdAt:              params.CreatedAt,
		updatedAt:              credentialUpdatedAt(params),
		version:                credentialVersion(params),
	}, nil
}

func (credential *Credential) MarkUsed(usedAt domain.Timestamp) error {
	if credential.revokedAt != nil {
		return ErrCredentialRevoked
	}
	if usedAt.Before(credential.updatedAt) {
		return domain.NewValidationError("lastUsedAt", "chronology", "cannot occur before the previous update")
	}
	usedAtCopy := usedAt
	credential.lastUsedAt = &usedAtCopy
	credential.updatedAt = usedAt
	credential.version++
	return nil
}

func (credential *Credential) Replace(successorID domain.ID, replacedAt domain.Timestamp) error {
	if successorID.Prefix() != domain.CredentialIDPrefix || successorID == credential.credentialID {
		return domain.NewValidationError("successorCredentialId", "format", "must identify a distinct integration credential")
	}
	if err := credential.Revoke(replacedAt); err != nil {
		return err
	}
	successorCopy := successorID
	credential.replacedByCredentialID = &successorCopy
	return nil
}

// Revoke marks the credential unusable without deleting its audit metadata.
func (credential *Credential) Revoke(revokedAt domain.Timestamp) error {
	if credential.revokedAt != nil {
		return ErrCredentialRevoked
	}
	if revokedAt.Before(credential.updatedAt) {
		return domain.NewValidationError(
			"revokedAt",
			"chronology",
			"cannot occur before the previous update",
		)
	}
	revokedAtCopy := revokedAt
	credential.revokedAt = &revokedAtCopy
	credential.updatedAt = revokedAt
	credential.version++
	return nil
}

// validateCredentialParams enforces identity, secret, scope, and time rules.
func validateCredentialParams(params CredentialParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(
		params.CredentialID.String(),
		domain.CredentialIDPrefix,
	); err != nil {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"credentialId",
				"format",
				"must be an integration credential identifier",
			),
		)
	}
	if _, err := domain.ParseID(
		params.SellerID.String(),
		domain.SellerIDPrefix,
	); err != nil {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"sellerId",
				"format",
				"must be a seller identifier",
			),
		)
	}
	label := strings.TrimSpace(params.Label)
	if label == "" || len(label) > maximumCredentialLabel {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"label",
				"length",
				"must contain 1-80 characters",
			),
		)
	}
	decodedHash, hashError := hex.DecodeString(params.TokenHash)
	if hashError != nil || len(decodedHash) != sha256.Size {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"tokenHash",
				"format",
				"must be a SHA-256 digest",
			),
		)
	}
	validationErrors = append(
		validationErrors,
		validateScopes(params.Scopes)...,
	)
	if params.EntitlementEpoch == 0 {
		validationErrors = append(validationErrors, domain.NewValidationError("entitlementEpoch", "minimum", "must be at least one"))
	}
	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"createdAt",
				"required",
				"is required",
			),
		)
	}
	if params.Version > 0 && (params.UpdatedAt.Time().IsZero() || params.UpdatedAt.Before(params.CreatedAt)) {
		validationErrors = append(validationErrors, domain.NewValidationError("updatedAt", "chronology", "must not precede creation"))
	}
	if params.ExpiresAt != nil &&
		!params.ExpiresAt.Time().After(params.CreatedAt.Time()) {
		validationErrors = append(
			validationErrors,
			domain.NewValidationError(
				"expiresAt",
				"chronology",
				"must occur after creation",
			),
		)
	}
	return validationErrors
}

// validateScopes rejects missing, duplicate, and unsupported capabilities.
func validateScopes(scopes []Scope) domain.ValidationErrors {
	if len(scopes) == 0 {
		return domain.ValidationErrors{
			domain.NewValidationError(
				"scopes",
				"required",
				"must contain at least one scope",
			),
		}

	}

	seen := make(map[Scope]struct{}, len(scopes))
	var validationErrors domain.ValidationErrors
	for _, scope := range scopes {
		if !supportedScope(scope) {
			validationErrors = append(
				validationErrors,
				domain.NewValidationError(
					"scopes",
					"supported",
					fmt.Sprintf("unsupported scope %q", scope),
				),
			)
			continue
		}
		if _, exists := seen[scope]; exists {
			validationErrors = append(
				validationErrors,
				domain.NewValidationError(
					"scopes",
					"unique",
					"must not contain duplicate scopes",
				),
			)
		}
		seen[scope] = struct{}{}
	}
	return validationErrors
}

// supportedScope reports whether a scope belongs to the fixed capability set.
func supportedScope(scope Scope) bool {
	switch scope {
	case ScopeRead, ScopeConfigure, ScopePublish, ScopeValidate, ScopeRotate:
		return true
	default:
		return false
	}
}

func validCredentialSecret(secret string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(secret)
	return err == nil && len(decoded) >= minimumCredentialSecret
}

func supportedExchangeScope(scope Scope) bool {
	return scope == ScopeRead || scope == ScopeConfigure || scope == ScopePublish || scope == ScopeValidate
}

// credentialToken creates the versioned point-addressable credential value.
func credentialToken(credentialID domain.ID, secret string) string {
	return strings.Join(
		[]string{
			credentialTokenVersion,
			credentialID.String(),
			secret,
		},
		".",
	)
}

// parseCredentialToken validates the non-secret routing segments.
func parseCredentialToken(rawToken string) (domain.ID, error) {
	segments := strings.Split(rawToken, ".")
	if len(segments) != 3 ||
		segments[0] != credentialTokenVersion ||
		!validCredentialSecret(segments[2]) {
		return "", ErrCredentialInvalid
	}
	credentialID, err := domain.ParseID(
		segments[1],
		domain.CredentialIDPrefix,
	)
	if err != nil {
		return "", ErrCredentialInvalid
	}
	return credentialID, nil
}

// credentialView removes the token hash from service responses.
func credentialView(credential Credential) CredentialView {
	return CredentialView{
		CredentialID:           credential.CredentialID(),
		SellerID:               credential.SellerID(),
		Label:                  credential.Label(),
		Scopes:                 credential.Scopes(),
		ExpiresAt:              credential.ExpiresAt(),
		RevokedAt:              credential.RevokedAt(),
		LastUsedAt:             credential.LastUsedAt(),
		ReplacedByCredentialID: credential.ReplacedByCredentialID(),
		CreatedAt:              credential.CreatedAt(),
		UpdatedAt:              credential.UpdatedAt(),
		Version:                credential.Version(),
	}
}

func credentialUpdatedAt(params CredentialParams) domain.Timestamp {
	if params.UpdatedAt.Time().IsZero() {
		return params.CreatedAt
	}
	return params.UpdatedAt
}

func credentialVersion(params CredentialParams) uint64 {
	if params.Version == 0 {
		return 1
	}
	return params.Version
}

func (service *Service) credentialIssuanceEpoch(ctx context.Context, sellerID domain.ID, now domain.Timestamp) (uint64, error) {
	if service.entitlementResolver == nil {
		return 1, nil
	}
	entitlement, err := service.entitlementResolver.ResolveSellerPlan(ctx, sellerID)
	if err != nil {
		return 0, err
	}
	if entitlement.Assignment.Status != billing.EntitlementStatusActive || !now.Before(entitlement.Assignment.AccessEndsAt) {
		return 0, ErrSubscriptionInactive
	}
	return entitlement.Assignment.EntitlementEpoch, nil
}
