package authorization

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/integrations"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestConfirmationGrantServiceReissuesAndConsumesExactBinding(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	repository := newConfirmationRepository()
	service := NewConfirmationGrantService(
		repository,
		&confirmationSellerAuthorizer{},
		&credentialReader{credential: testCredential(now)},
		&entitlementReader{response: activeEntitlement(now.Add(time.Hour), 7)},
		&confirmationTargetReader{seller: catalog.Seller{SellerID: domain.ID(testSellerID), Version: 3}},
		&sequenceIDGenerator{ids: []domain.ID{domain.ID("mcg_01K5D09YJ0C0M7RJM4FWQ0K9HA"), domain.ID("mcg_01K5D09YJ0C0M7RJM4FWQ0K9HB")}},
		&fixedTokenGenerator{token: strings.Repeat("s", 43)},
		integrations.NewHMACCredentialDigester(integrations.StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))}),
		&mutableClock{now: now},
		audit.NoopRecorder{},
	)
	request := CreateConfirmationGrantRequest{
		CredentialID: domain.ID(testCredentialID), Tool: ToolConfigureStorefront,
		TargetType: ConfirmationTargetSeller, TargetID: domain.ID(testSellerID),
		ArgumentsSHA256: strings.Repeat("a", 64), ExpectedResourceVersion: 3,
		Summary: "Update the reviewed seller storefront settings",
	}
	first, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfirmationGrant == second.ConfirmationGrant || first.ConfirmationGrantID == second.ConfirmationGrantID {
		t.Fatalf("reissued grants = %#v %#v", first, second)
	}
	principal := integrations.Principal{SellerID: domain.ID(testSellerID), CredentialID: domain.ID(testCredentialID), Scopes: []integrations.Scope{integrations.ScopeConfigure}}
	binding := ConfirmationConsumption{
		ConfirmationGrant: first.ConfirmationGrant, Tool: request.Tool, TargetType: request.TargetType,
		TargetID: request.TargetID, ArgumentsSHA256: request.ArgumentsSHA256,
		ExpectedResourceVersion: request.ExpectedResourceVersion, IdempotencyKey: "mutation-key-001",
	}
	if err := service.Consume(t.Context(), principal, binding); !errors.Is(err, ErrConfirmationDenied) {
		t.Fatalf("superseded grant error = %v", err)
	}
	binding.ConfirmationGrant = second.ConfirmationGrant
	if err := service.Consume(t.Context(), principal, binding); err != nil {
		t.Fatal(err)
	}
	if err := service.Consume(t.Context(), principal, binding); err != nil {
		t.Fatalf("exact retry error = %v", err)
	}
	binding.IdempotencyKey = "mutation-key-002"
	if err := service.Consume(t.Context(), principal, binding); !errors.Is(err, ErrConfirmationReplayed) {
		t.Fatalf("replayed grant error = %v", err)
	}
}

func TestConfirmationGrantServiceRejectsWrongBindingAndExpiry(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	clock := &mutableClock{now: now}
	service := NewConfirmationGrantService(
		newConfirmationRepository(), &confirmationSellerAuthorizer{},
		&credentialReader{credential: testCredential(now)}, &entitlementReader{response: activeEntitlement(now.Add(time.Hour), 7)},
		&confirmationTargetReader{route: catalog.PaidRoute{RouteID: domain.ID("rte_01K5D09YJ0C0M7RJM4FWQ0K9HC"), SellerID: domain.ID(testSellerID), Version: 2}},
		&fixedIDGenerator{id: domain.ID("mcg_01K5D09YJ0C0M7RJM4FWQ0K9HA")}, &fixedTokenGenerator{token: strings.Repeat("s", 43)},
		integrations.NewHMACCredentialDigester(integrations.StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))}),
		clock, audit.NoopRecorder{},
	)
	request := CreateConfirmationGrantRequest{
		CredentialID: domain.ID(testCredentialID), Tool: ToolChangeRoutePrice,
		TargetType: ConfirmationTargetPaidRoute, TargetID: domain.ID("rte_01K5D09YJ0C0M7RJM4FWQ0K9HC"),
		ArgumentsSHA256: strings.Repeat("a", 64), ExpectedResourceVersion: 2,
		Summary: "Publish the reviewed paid route after validation",
	}
	created, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), request)
	if err != nil {
		t.Fatal(err)
	}
	principal := integrations.Principal{SellerID: domain.ID(testSellerID), CredentialID: domain.ID(testCredentialID)}
	binding := ConfirmationConsumption{
		ConfirmationGrant: created.ConfirmationGrant, Tool: request.Tool, TargetType: request.TargetType,
		TargetID: request.TargetID, ArgumentsSHA256: strings.Repeat("b", 64), ExpectedResourceVersion: 2,
		IdempotencyKey: "mutation-key-001",
	}
	if err := service.Consume(t.Context(), principal, binding); !errors.Is(err, ErrConfirmationDenied) {
		t.Fatalf("wrong binding error = %v", err)
	}
	binding.ArgumentsSHA256 = request.ArgumentsSHA256
	clock.now = now.Add(MaximumConfirmationGrantLifetime)
	if err := service.Consume(t.Context(), principal, binding); !errors.Is(err, ErrConfirmationDenied) {
		t.Fatalf("expired grant error = %v", err)
	}
}

func TestConfirmationGrantServiceRejectsWrongTargetOwnershipOrVersion(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	targets := &confirmationTargetReader{route: catalog.PaidRoute{
		RouteID:  domain.ID("rte_01K5D09YJ0C0M7RJM4FWQ0K9HC"),
		SellerID: domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9HZ"), Version: 4,
	}}
	service := NewConfirmationGrantService(
		newConfirmationRepository(), &confirmationSellerAuthorizer{},
		&credentialReader{credential: testCredential(now)}, &entitlementReader{response: activeEntitlement(now.Add(time.Hour), 7)}, targets,
		&fixedIDGenerator{id: domain.ID("mcg_01K5D09YJ0C0M7RJM4FWQ0K9HA")}, &fixedTokenGenerator{token: strings.Repeat("s", 43)},
		integrations.NewHMACCredentialDigester(integrations.StaticCredentialPepperProvider{Value: []byte(strings.Repeat("p", 32))}),
		&mutableClock{now: now}, audit.NoopRecorder{},
	)
	request := CreateConfirmationGrantRequest{
		CredentialID: domain.ID(testCredentialID), Tool: ToolChangeRoutePrice,
		TargetType: ConfirmationTargetPaidRoute, TargetID: targets.route.RouteID,
		ArgumentsSHA256: strings.Repeat("a", 64), ExpectedResourceVersion: 4,
		Summary: "Change the reviewed paid route price",
	}
	if _, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), request); !errors.Is(err, ErrConfirmationDenied) {
		t.Fatalf("cross-seller target error = %v", err)
	}
	targets.route.SellerID = domain.ID(testSellerID)
	request.ExpectedResourceVersion = 3
	if _, err := service.Create(t.Context(), "owner-123", domain.ID(testSellerID), request); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("stale target version error = %v", err)
	}
}

func TestHashMutationArgumentsExcludesGrantAndIdempotency(t *testing.T) {
	t.Parallel()
	first, err := HashMutationArguments(map[string]any{
		"idempotencyKey": "request-one", "confirmationGrant": "secret-one",
		"route": map[string]any{"amount": "100", "method": "POST"},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashMutationArguments(map[string]any{
		"confirmationGrant": "secret-two", "route": map[string]any{"method": "POST", "amount": "100"},
		"idempotencyKey": "request-two",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("hashes differ: %s != %s", first, second)
	}
}

type confirmationSellerAuthorizer struct{}

func (*confirmationSellerAuthorizer) AuthorizeSeller(context.Context, string, domain.ID) error {
	return nil
}

type fixedTokenGenerator struct{ token string }

func (generator *fixedTokenGenerator) NewToken() (string, error) { return generator.token, nil }

type sequenceIDGenerator struct {
	ids   []domain.ID
	index int
}

func (generator *sequenceIDGenerator) New(domain.IDPrefix) (domain.ID, error) {
	identifier := generator.ids[generator.index]
	generator.index++
	return identifier, nil
}

type confirmationRepository struct {
	grants   map[domain.ID]ConfirmationGrant
	bindings map[string]domain.ID
}

type confirmationTargetReader struct {
	seller catalog.Seller
	route  catalog.PaidRoute
	err    error
}

func (reader *confirmationTargetReader) GetSeller(context.Context, domain.ID) (catalog.Seller, error) {
	if reader.err != nil {
		return catalog.Seller{}, reader.err
	}
	return reader.seller, nil
}

func (reader *confirmationTargetReader) GetRoute(context.Context, domain.ID) (catalog.PaidRoute, error) {
	if reader.err != nil {
		return catalog.PaidRoute{}, reader.err
	}
	return reader.route, nil
}

func newConfirmationRepository() *confirmationRepository {
	return &confirmationRepository{grants: make(map[domain.ID]ConfirmationGrant), bindings: make(map[string]domain.ID)}
}

func (repository *confirmationRepository) CreateReplacing(_ context.Context, grant ConfirmationGrant) error {
	if _, exists := repository.grants[grant.ConfirmationGrantID]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.grants[grant.ConfirmationGrantID] = grant
	repository.bindings[grant.BindingHash] = grant.ConfirmationGrantID
	return nil
}

func (repository *confirmationRepository) Get(_ context.Context, grantID domain.ID) (ConfirmationGrant, error) {
	grant, exists := repository.grants[grantID]
	if !exists {
		return ConfirmationGrant{}, persistence.ErrNotFound
	}
	return grant, nil
}

func (repository *confirmationRepository) Consume(_ context.Context, grant ConfirmationGrant, expectedVersion uint64) error {
	stored, exists := repository.grants[grant.ConfirmationGrantID]
	if !exists || stored.Version != expectedVersion || repository.bindings[stored.BindingHash] != stored.ConfirmationGrantID {
		return persistence.ErrConditionFailed
	}
	repository.grants[grant.ConfirmationGrantID] = grant
	return nil
}
