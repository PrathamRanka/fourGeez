package authorization

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
)

const (
	testSellerID     = "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	testCredentialID = "key_01K5D09YJ0C0M7RJM4FWQ0K9H8"
	testCapabilityID = "cap_01K5D09YJ0C0M7RJM4FWQ0K9H9"
)

func TestAccessTokenServiceIssuesAndValidatesExactES256Capability(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	clock := &mutableClock{now: now}
	keys, err := NewLocalES256KeyRing(clock)
	if err != nil {
		t.Fatal(err)
	}
	credential := testCredential(now)
	service := NewAccessTokenService(AccessTokenConfig{
		Issuer: "https://api.agentpay.test", Audience: MCPAudience, Lifetime: 2 * time.Minute,
	}, &exchangeAuthorizer{authorization: integrations.ExchangeAuthorization{
		Principal:        integrations.Principal{SellerID: credential.SellerID(), CredentialID: credential.CredentialID(), Scopes: []integrations.Scope{integrations.ScopeRead, integrations.ScopeConfigure}},
		EntitlementEpoch: 7,
	}}, &credentialReader{credential: credential}, &entitlementReader{response: activeEntitlement(now.Add(time.Hour), 7)}, keys, keys, &fixedIDGenerator{id: domain.ID(testCapabilityID)}, clock)

	issued, err := service.Exchange(t.Context(), "apc2.project.secret", AccessTokenRequest{Audience: MCPAudience, Scopes: []integrations.Scope{integrations.ScopeConfigure, integrations.ScopeRead}})
	if err != nil {
		t.Fatal(err)
	}
	if issued.TokenType != "Bearer" || issued.ExpiresIn != 120 || issued.Scope != "configure read" {
		t.Fatalf("issued response = %#v", issued)
	}
	principal, err := service.AuthorizeAccessToken(t.Context(), issued.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if principal.SellerID != credential.SellerID() || principal.CredentialID != credential.CredentialID() || !principal.HasScope(integrations.ScopeConfigure) {
		t.Fatalf("principal = %#v", principal)
	}
}

func TestAccessTokenServicePublishesOverlappingJWKSOnRotation(t *testing.T) {
	t.Parallel()
	clock := &mutableClock{now: time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)}
	keys, err := NewLocalES256KeyRing(clock)
	if err != nil {
		t.Fatal(err)
	}
	before, err := keys.JWKS(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.Rotate(); err != nil {
		t.Fatal(err)
	}
	during, err := keys.JWKS(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Keys) != 1 || len(during.Keys) != 2 || before.Keys[0].KeyID == during.Keys[0].KeyID {
		t.Fatalf("JWKS before/after = %#v / %#v", before, during)
	}
	clock.now = clock.now.Add(MaximumAccessTokenLifetime)
	after, err := keys.JWKS(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Keys) != 1 {
		t.Fatalf("retired JWKS keys = %#v", after.Keys)
	}
}

func TestAccessTokenAuthorizationRechecksCredentialAndEntitlement(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	clock := &mutableClock{now: now}
	keys, err := NewLocalES256KeyRing(clock)
	if err != nil {
		t.Fatal(err)
	}
	credential := testCredential(now)
	credentials := &credentialReader{credential: credential}
	entitlements := &entitlementReader{response: activeEntitlement(now.Add(time.Hour), 7)}
	service := NewAccessTokenService(AccessTokenConfig{Issuer: "https://api.agentpay.test", Audience: MCPAudience, Lifetime: 2 * time.Minute},
		&exchangeAuthorizer{authorization: integrations.ExchangeAuthorization{Principal: integrations.Principal{SellerID: credential.SellerID(), CredentialID: credential.CredentialID(), Scopes: []integrations.Scope{integrations.ScopeRead}}, EntitlementEpoch: 7}},
		credentials, entitlements, keys, keys, &fixedIDGenerator{id: domain.ID(testCapabilityID)}, clock)
	issued, err := service.Exchange(t.Context(), "apc2.project.secret", AccessTokenRequest{Audience: MCPAudience, Scopes: []integrations.Scope{integrations.ScopeRead}})
	if err != nil {
		t.Fatal(err)
	}

	revoked := credential
	if err := revoked.Revoke(domain.NewTimestamp(now.Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	credentials.credential = revoked
	if _, err := service.AuthorizeAccessToken(t.Context(), issued.AccessToken); !errors.Is(err, ErrAccessTokenRevoked) {
		t.Fatalf("revoked credential error = %v", err)
	}

	credentials.credential = credential
	entitlements.response.Assignment.EntitlementEpoch = 8
	if _, err := service.AuthorizeAccessToken(t.Context(), issued.AccessToken); !errors.Is(err, ErrAccessTokenRevoked) {
		t.Fatalf("stale epoch error = %v", err)
	}
	entitlements.response.Assignment.EntitlementEpoch = 7
	entitlements.response.Assignment.AccessEndsAt = domain.NewTimestamp(now)
	if _, err := service.AuthorizeAccessToken(t.Context(), issued.AccessToken); !errors.Is(err, ErrSubscriptionInactive) {
		t.Fatalf("inactive entitlement error = %v", err)
	}
}

func TestAccessTokenAuthorizationRejectsProjectKeysAtMCPBoundary(t *testing.T) {
	t.Parallel()
	service := &AccessTokenService{}
	for _, token := range []string{"apc1.seller.key.secret", "apc2.key.secret"} {
		if _, err := service.AuthorizeAccessToken(t.Context(), token); !errors.Is(err, ErrInvalidAccessToken) {
			t.Fatalf("AuthorizeAccessToken(%q) error = %v", token, err)
		}
	}
}

func TestAccessTokenAuthorizationRejectsMalformedDomainIdentifiers(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	clock := &mutableClock{now: now}
	keys, err := NewLocalES256KeyRing(clock)
	if err != nil {
		t.Fatal(err)
	}
	service := NewAccessTokenService(
		AccessTokenConfig{Issuer: "https://api.agentpay.test", Audience: MCPAudience, Lifetime: 2 * time.Minute},
		nil, &credentialReader{}, &entitlementReader{}, keys, keys, &fixedIDGenerator{}, clock,
	)
	claims := accessTokenClaims{
		Issuer: "https://api.agentpay.test", Audience: MCPAudience,
		Subject: "key_not-a-ulid", SellerID: domain.ID("sel_not-a-ulid"), CredentialID: domain.ID("key_not-a-ulid"),
		Scope: string(integrations.ScopeRead), EntitlementEpoch: 7, JWTID: domain.ID("cap_not-a-ulid"),
		IssuedAt: now.Unix(), ExpiresAt: now.Add(2 * time.Minute).Unix(),
	}
	keyID, err := keys.CurrentKeyID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	token, err := service.sign(t.Context(), keyID, claims)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthorizeAccessToken(t.Context(), token); !errors.Is(err, ErrInvalidAccessToken) {
		t.Fatalf("malformed identifier error = %v", err)
	}
}

func TestAccessTokenAuthorizationRejectsMismatchedEntitlementSeller(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	clock := &mutableClock{now: now}
	keys, err := NewLocalES256KeyRing(clock)
	if err != nil {
		t.Fatal(err)
	}
	credential := testCredential(now)
	entitlement := activeEntitlement(now.Add(time.Hour), 7)
	entitlement.Assignment.SellerID = domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9HZ")
	service := NewAccessTokenService(
		AccessTokenConfig{Issuer: "https://api.agentpay.test", Audience: MCPAudience, Lifetime: 2 * time.Minute},
		&exchangeAuthorizer{authorization: integrations.ExchangeAuthorization{
			Principal:        integrations.Principal{SellerID: credential.SellerID(), CredentialID: credential.CredentialID(), Scopes: []integrations.Scope{integrations.ScopeRead}},
			EntitlementEpoch: 7,
		}},
		&credentialReader{credential: credential}, &entitlementReader{response: entitlement}, keys, keys,
		&fixedIDGenerator{id: domain.ID(testCapabilityID)}, clock,
	)
	issued, err := service.Exchange(t.Context(), "apc2.project.secret", AccessTokenRequest{Audience: MCPAudience, Scopes: []integrations.Scope{integrations.ScopeRead}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthorizeAccessToken(t.Context(), issued.AccessToken); !errors.Is(err, ErrAccessTokenRevoked) {
		t.Fatalf("mismatched entitlement seller error = %v", err)
	}
}

func testCredential(now time.Time) integrations.Credential {
	credential, err := integrations.NewCredential(integrations.CredentialParams{
		CredentialID: domain.ID(testCredentialID), SellerID: domain.ID(testSellerID), TokenHash: strings.Repeat("a", 64),
		Label: "Connector", Scopes: []integrations.Scope{integrations.ScopeRead, integrations.ScopeConfigure}, EntitlementEpoch: 7,
		CreatedAt: domain.NewTimestamp(now.Add(-time.Hour)),
	})
	if err != nil {
		panic(err)
	}
	return credential
}

func activeEntitlement(accessEndsAt time.Time, epoch uint64) billing.SellerPlanResponse {
	return billing.SellerPlanResponse{Assignment: billing.SellerEntitlementView{
		SellerID: domain.ID(testSellerID), Status: billing.EntitlementStatusActive,
		AccessEndsAt: domain.NewTimestamp(accessEndsAt), EntitlementEpoch: epoch,
	}}
}

type mutableClock struct{ now time.Time }

func (clock *mutableClock) Now() time.Time { return clock.now }

type fixedIDGenerator struct{ id domain.ID }

func (generator *fixedIDGenerator) New(domain.IDPrefix) (domain.ID, error) { return generator.id, nil }

type exchangeAuthorizer struct {
	authorization integrations.ExchangeAuthorization
	err           error
}

func (authorizer *exchangeAuthorizer) AuthorizeExchange(context.Context, string, []integrations.Scope) (integrations.ExchangeAuthorization, error) {
	return authorizer.authorization, authorizer.err
}

type credentialReader struct {
	credential integrations.Credential
	err        error
}

func (reader *credentialReader) GetByID(context.Context, domain.ID) (integrations.Credential, error) {
	if reader.err != nil {
		return integrations.Credential{}, reader.err
	}
	return reader.credential, nil
}

type entitlementReader struct {
	response billing.SellerPlanResponse
	err      error
}

func (reader *entitlementReader) ResolveSellerPlan(context.Context, domain.ID) (billing.SellerPlanResponse, error) {
	if reader.err != nil {
		return billing.SellerPlanResponse{}, reader.err
	}
	return reader.response, nil
}
