package integrations

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
)

func TestSecureTokenGeneratorProduces256BitSecret(t *testing.T) {
	t.Parallel()
	generator := NewSecureTokenGenerator(strings.NewReader(strings.Repeat("x", 32)))
	secret, err := generator.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("secret length = %d, decode error = %v", len(decoded), err)
	}
}

func TestServiceCreatesAPC2CredentialWithKeyedDigest(t *testing.T) {
	t.Parallel()

	repository := newCredentialRepository()
	service := NewService(
		repository,
		&sellerAuthorizer{},
		&credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)},
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)},
		audit.NoopRecorder{},
		WithCredentialDigester(NewHMACCredentialDigester(StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))})),
	)
	created, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), CreateCredentialRequest{Label: "Production connector", Scopes: []Scope{ScopeRead, ScopeConfigure}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Token, "apc2."+testCredentialID+".") || strings.Contains(created.Token, testSellerID) {
		t.Fatalf("project key = %q", created.Token)
	}
	stored, err := repository.GetByID(t.Context(), domain.ID(testCredentialID))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored.TokenHash(), strings.Repeat("s", 43)) || len(stored.TokenHash()) != 64 {
		t.Fatalf("stored digest = %q", stored.TokenHash())
	}
	wrongPepper := NewHMACCredentialDigester(StaticCredentialPepperProvider{Value: []byte(strings.Repeat("q", 32))})
	wrongDigest, err := wrongPepper.Digest(t.Context(), created.Token)
	if err != nil {
		t.Fatal(err)
	}
	if wrongDigest == stored.TokenHash() {
		t.Fatal("credential digest did not depend on the pepper")
	}
}

func TestAuthorizeExchangeChecksRateLimitCredentialEntitlementQuotaAndLastUsed(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)
	repository := newCredentialRepository()
	limiter := &exchangeLimiter{}
	quota := &exchangeQuota{}
	entitlements := &exchangeEntitlementResolver{response: activeExchangeEntitlement(now.Add(time.Minute))}
	service := NewService(
		repository, &sellerAuthorizer{}, &credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)}, &credentialClock{now: now}, audit.NoopRecorder{},
		WithCredentialDigester(NewHMACCredentialDigester(StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))})),
		WithExchangeAuthorization(entitlements, limiter, quota),
	)
	created, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), CreateCredentialRequest{Label: "Connector", Scopes: []Scope{ScopeRead, ScopeConfigure}})
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := service.AuthorizeExchange(t.Context(), created.Token, []Scope{ScopeRead})
	if err != nil {
		t.Fatal(err)
	}
	if authorization.SellerID != domain.ID(testSellerID) || authorization.CredentialID != domain.ID(testCredentialID) || authorization.EntitlementEpoch != 4 {
		t.Fatalf("exchange authorization = %#v", authorization)
	}
	stored, _ := repository.GetByID(t.Context(), domain.ID(testCredentialID))
	if stored.LastUsedAt() == nil || !stored.LastUsedAt().Time().Equal(now) || limiter.calls != 1 || quota.calls != 1 {
		t.Fatalf("exchange effects: lastUsed=%v limiter=%d quota=%d", stored.LastUsedAt(), limiter.calls, quota.calls)
	}
}

func TestAuthorizeExchangeDeniesAtExactAccessBoundary(t *testing.T) {
	t.Parallel()

	boundary := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	repository := newCredentialRepository()
	quota := &exchangeQuota{}
	clock := &credentialClock{now: boundary.Add(-time.Minute)}
	service := NewService(
		repository, &sellerAuthorizer{}, &credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)}, clock, audit.NoopRecorder{},
		WithCredentialDigester(NewHMACCredentialDigester(StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))})),
		WithExchangeAuthorization(&exchangeEntitlementResolver{response: activeExchangeEntitlement(boundary)}, &exchangeLimiter{}, quota),
	)
	created, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), CreateCredentialRequest{Label: "Connector", Scopes: []Scope{ScopeRead}})
	if err != nil {
		t.Fatal(err)
	}
	clock.now = boundary
	_, err = service.AuthorizeExchange(t.Context(), created.Token, []Scope{ScopeRead})
	if !errors.Is(err, ErrSubscriptionInactive) || quota.calls != 0 {
		t.Fatalf("AuthorizeExchange() = %v, quota calls = %d", err, quota.calls)
	}
	stored, _ := repository.GetByID(t.Context(), domain.ID(testCredentialID))
	if stored.LastUsedAt() != nil {
		t.Fatalf("lastUsedAt = %v, want nil", stored.LastUsedAt())
	}
}

func TestAuthorizeExchangeDeniesStaleEntitlementEpochBeforeQuota(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)
	repository := newCredentialRepository()
	resolver := &exchangeEntitlementResolver{response: activeExchangeEntitlement(now.Add(time.Minute))}
	quota := &exchangeQuota{}
	service := NewService(
		repository, &sellerAuthorizer{}, &credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)}, &credentialClock{now: now}, audit.NoopRecorder{},
		WithCredentialDigester(NewHMACCredentialDigester(StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))})),
		WithExchangeAuthorization(resolver, &exchangeLimiter{}, quota),
	)
	created, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), CreateCredentialRequest{Label: "Connector", Scopes: []Scope{ScopeRead}})
	if err != nil {
		t.Fatal(err)
	}
	resolver.response.Assignment.EntitlementEpoch++
	_, err = service.AuthorizeExchange(t.Context(), created.Token, []Scope{ScopeRead})
	if !errors.Is(err, ErrCredentialRevoked) || quota.calls != 0 {
		t.Fatalf("AuthorizeExchange() = %v, quota calls = %d", err, quota.calls)
	}
}

func TestServiceRotatesCredentialAtomically(t *testing.T) {
	t.Parallel()

	repository := newCredentialRepository()
	ids := &credentialIDSequence{ids: []domain.ID{domain.ID(testCredentialID), domain.ID("key_01K5D09YJ0C0M7RJM4FWQ0K9H9")}}
	service := NewService(
		repository, &sellerAuthorizer{}, ids, &credentialTokenGenerator{token: strings.Repeat("s", 43)},
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)}, audit.NoopRecorder{},
		WithCredentialDigester(NewHMACCredentialDigester(StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))})),
	)
	created, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), CreateCredentialRequest{Label: "Connector", Scopes: []Scope{ScopeRead, ScopeConfigure}})
	if err != nil {
		t.Fatal(err)
	}
	rotation, err := service.Rotate(
		t.Context(),
		"owner-123",
		domain.ID(testSellerID),
		created.CredentialID,
		RotateCredentialRequest{ExpectedVersion: created.Version},
		RotationIdempotency{Key: "credential-rotate-1", RequestBody: []byte(`{"expectedVersion":1}`)},
	)
	if err != nil {
		t.Fatal(err)
	}
	if rotation.Predecessor.RevokedAt == nil || rotation.Predecessor.ReplacedByCredentialID == nil || *rotation.Predecessor.ReplacedByCredentialID != rotation.Successor.CredentialID {
		t.Fatalf("rotation = %#v", rotation)
	}
	if !strings.HasPrefix(rotation.Token, "apc2."+rotation.Successor.CredentialID.String()+".") || repository.rotateCalls != 1 {
		t.Fatalf("rotation token/calls = %q/%d", rotation.Token, repository.rotateCalls)
	}
	if _, err := service.AuthenticateToken(t.Context(), created.Token); !errors.Is(err, ErrCredentialRevoked) {
		t.Fatalf("predecessor authentication error = %v", err)
	}
}

func TestServiceReplaysCommittedCredentialRotationExactly(t *testing.T) {
	t.Parallel()

	repository := newCredentialRepository()
	ids := &credentialIDSequence{ids: []domain.ID{domain.ID(testCredentialID), domain.ID("key_01K5D09YJ0C0M7RJM4FWQ0K9H9")}}
	service := NewService(
		repository, &sellerAuthorizer{}, ids, &credentialTokenGenerator{token: strings.Repeat("s", 43)},
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)}, audit.NoopRecorder{},
		WithCredentialDigester(NewHMACCredentialDigester(StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))})),
	)
	created, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), CreateCredentialRequest{Label: "Connector", Scopes: []Scope{ScopeRead}})
	if err != nil {
		t.Fatal(err)
	}
	idempotency := RotationIdempotency{Key: "credential-rotate-replay", RequestBody: []byte(`{"expectedVersion":1}`)}
	first, err := service.Rotate(t.Context(), "owner-123", domain.ID(testSellerID), created.CredentialID, RotateCredentialRequest{ExpectedVersion: 1}, idempotency)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.Rotate(t.Context(), "owner-123", domain.ID(testSellerID), created.CredentialID, RotateCredentialRequest{ExpectedVersion: 1}, idempotency)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Token != first.Token || replayed.Successor.CredentialID != first.Successor.CredentialID || repository.rotateCalls != 1 {
		t.Fatalf("replayed rotation = %#v, first = %#v, rotate calls = %d", replayed, first, repository.rotateCalls)
	}
}

func TestServiceRejectsCredentialRotationIdempotencyConflict(t *testing.T) {
	t.Parallel()

	repository := newCredentialRepository()
	ids := &credentialIDSequence{ids: []domain.ID{domain.ID(testCredentialID), domain.ID("key_01K5D09YJ0C0M7RJM4FWQ0K9H9")}}
	service := NewService(
		repository, &sellerAuthorizer{}, ids, &credentialTokenGenerator{token: strings.Repeat("s", 43)},
		domain.FixedClock{Value: time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)}, audit.NoopRecorder{},
		WithCredentialDigester(NewHMACCredentialDigester(StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))})),
	)
	created, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), CreateCredentialRequest{Label: "Connector", Scopes: []Scope{ScopeRead}})
	if err != nil {
		t.Fatal(err)
	}
	first := RotationIdempotency{Key: "credential-rotate-conflict", RequestBody: []byte(`{"expectedVersion":1}`)}
	if _, err := service.Rotate(t.Context(), "owner-123", domain.ID(testSellerID), created.CredentialID, RotateCredentialRequest{ExpectedVersion: 1}, first); err != nil {
		t.Fatal(err)
	}
	conflict := RotationIdempotency{Key: first.Key, RequestBody: []byte(`{"expectedVersion":1,"label":"changed"}`)}
	_, err = service.Rotate(t.Context(), "owner-123", domain.ID(testSellerID), created.CredentialID, RotateCredentialRequest{ExpectedVersion: 1, Label: "changed"}, conflict)
	if !errors.Is(err, ErrRotationIdempotencyConflict) {
		t.Fatalf("Rotate() error = %v", err)
	}
}

func TestAuthorizeExchangeAppendsSuccessAndDenialAuditEvents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)
	recorder := &integrationAuditRecorder{}
	service := NewService(
		newCredentialRepository(), &sellerAuthorizer{}, &credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)}, &credentialClock{now: now}, recorder,
		WithCredentialDigester(NewHMACCredentialDigester(StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))})),
		WithExchangeAuthorization(&exchangeEntitlementResolver{response: activeExchangeEntitlement(now.Add(time.Minute))}, &exchangeLimiter{}, &exchangeQuota{}),
	)
	created, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), CreateCredentialRequest{Label: "Connector", Scopes: []Scope{ScopeRead}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthorizeExchange(t.Context(), created.Token, []Scope{ScopeRead}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthorizeExchange(t.Context(), created.Token, []Scope{ScopePublish}); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("denied exchange error = %v", err)
	}
	if len(recorder.requests) != 3 || recorder.requests[1].Action != audit.ActionCredentialExchangeSucceeded || recorder.requests[2].Action != audit.ActionCredentialExchangeDenied || recorder.requests[2].Outcome != audit.OutcomeDenied {
		t.Fatalf("audit requests = %#v", recorder.requests)
	}
}

func activeExchangeEntitlement(accessEndsAt time.Time) billing.SellerPlanResponse {
	return billing.SellerPlanResponse{Assignment: billing.SellerPlanView{
		SellerID: domain.ID(testSellerID), Status: billing.EntitlementStatusActive,
		AccessEndsAt: domain.NewTimestamp(accessEndsAt), EntitlementEpoch: 4,
	}}
}

type exchangeEntitlementResolver struct {
	response billing.SellerPlanResponse
	err      error
}

func (resolver *exchangeEntitlementResolver) ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error) {
	return resolver.response, resolver.err
}

type exchangeLimiter struct {
	calls int
	err   error
}

func (limiter *exchangeLimiter) AllowProjectKeyExchange(context.Context, domain.ID) error {
	limiter.calls++
	return limiter.err
}

type exchangeQuota struct {
	calls int
	err   error
}

func (quota *exchangeQuota) ConsumeAPIRequest(context.Context, domain.ID) error {
	quota.calls++
	return quota.err
}

type credentialIDSequence struct {
	ids []domain.ID
	pos int
}

func (generator *credentialIDSequence) New(domain.IDPrefix) (domain.ID, error) {
	id := generator.ids[generator.pos]
	generator.pos++
	return id, nil
}
