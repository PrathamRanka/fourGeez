package integrations

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const (
	testSellerID     = "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"
	testCredentialID = "key_01K5D09YJ0C0M7RJM4FWQ0K9H8"
)

// TestServiceCreatesAndAuthenticatesScopedCredential verifies secure issuance.
func TestServiceCreatesAndAuthenticatesScopedCredential(t *testing.T) {
	t.Parallel()

	clock := domain.FixedClock{
		Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	repository := newCredentialRepository()
	service := NewService(
		repository,
		&sellerAuthorizer{},
		&credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)},
		clock,
	)
	expiresAt := domain.NewTimestamp(clock.Now().Add(time.Hour))

	created, err := service.Create(
		t.Context(),
		"owner-123",
		domain.ID(testSellerID),
		CreateCredentialRequest{
			Label:     "Claude Code laptop",
			Scopes:    []Scope{ScopeRead, ScopeConfigure},
			ExpiresAt: &expiresAt,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if created.Token == "" || !strings.HasPrefix(created.Token, "apc1.") {
		t.Fatalf("token = %q", created.Token)
	}
	stored, err := repository.Get(
		t.Context(),
		domain.ID(testSellerID),
		domain.ID(testCredentialID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored.TokenHash(), strings.Repeat("s", 43)) {
		t.Fatal("repository stored raw credential material")
	}

	principal, err := service.Authenticate(
		t.Context(),
		created.Token,
		ScopeConfigure,
	)
	if err != nil {
		t.Fatal(err)
	}
	if principal.SellerID.String() != testSellerID ||
		principal.CredentialID.String() != testCredentialID {
		t.Fatalf("principal = %#v", principal)
	}
	if _, err := service.Authenticate(
		t.Context(),
		created.Token,
		ScopePublish,
	); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("Authenticate() error = %v", err)
	}
}

// TestServiceRejectsInvalidCredentialRequests verifies domain validation.
func TestServiceRejectsInvalidCredentialRequests(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		request CreateCredentialRequest
	}{
		{name: "empty label", request: CreateCredentialRequest{Scopes: []Scope{ScopeRead}}},
		{name: "empty scopes", request: CreateCredentialRequest{Label: "Laptop"}},
		{name: "duplicate scope", request: CreateCredentialRequest{Label: "Laptop", Scopes: []Scope{ScopeRead, ScopeRead}}},
		{name: "unknown scope", request: CreateCredentialRequest{Label: "Laptop", Scopes: []Scope{"admin"}}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := testCredentialService()
			if _, err := service.Create(
				t.Context(),
				"owner-123",
				domain.ID(testSellerID),
				testCase.request,
			); err == nil {
				t.Fatal("Create() accepted invalid credential input")
			}
		})
	}
}

// TestServiceRevokesCredentialImmediately verifies revoked tokens cannot authenticate.
func TestServiceRevokesCredentialImmediately(t *testing.T) {
	t.Parallel()

	service := testCredentialService()
	created, err := service.Create(
		t.Context(),
		"owner-123",
		domain.ID(testSellerID),
		CreateCredentialRequest{
			Label:  "Codex workstation",
			Scopes: []Scope{ScopeRead},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := service.Revoke(
		t.Context(),
		"owner-123",
		domain.ID(testSellerID),
		created.CredentialID,
		created.Version,
	)
	if err != nil {
		t.Fatal(err)
	}
	if revoked.RevokedAt == nil || revoked.Version != created.Version+1 {
		t.Fatalf("revoked credential = %#v", revoked)
	}
	if _, err := service.Authenticate(
		t.Context(),
		created.Token,
		ScopeRead,
	); !errors.Is(err, ErrCredentialRevoked) {
		t.Fatalf("Authenticate() error = %v", err)
	}
}

// TestServiceRejectsModifiedAndExpiredTokens verifies token lifecycle checks.
func TestServiceRejectsModifiedAndExpiredTokens(t *testing.T) {
	t.Parallel()

	clock := &credentialClock{
		now: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
	}
	service := NewService(
		newCredentialRepository(),
		&sellerAuthorizer{},
		&credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)},
		clock,
	)
	expiresAt := domain.NewTimestamp(clock.now.Add(time.Minute))
	created, err := service.Create(
		t.Context(),
		"owner-123",
		domain.ID(testSellerID),
		CreateCredentialRequest{
			Label:     "Temporary setup",
			Scopes:    []Scope{ScopeRead},
			ExpiresAt: &expiresAt,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(
		t.Context(),
		created.Token+"modified",
		ScopeRead,
	); !errors.Is(err, ErrCredentialInvalid) {
		t.Fatalf("modified token error = %v", err)
	}

	clock.now = clock.now.Add(time.Minute)
	if _, err := service.Authenticate(
		t.Context(),
		created.Token,
		ScopeRead,
	); !errors.Is(err, ErrCredentialExpired) {
		t.Fatalf("expired token error = %v", err)
	}
}

// TestCredentialIdempotencyResultRedactsToken verifies replay storage is safe.
func TestCredentialIdempotencyResultRedactsToken(t *testing.T) {
	t.Parallel()

	created := CredentialCreated{
		CredentialView: CredentialView{
			CredentialID: domain.ID(testCredentialID),
			SellerID:     domain.ID(testSellerID),
		},
		Token: "one-time-token",
	}
	stored, err := credentialIdempotencyResult(
		created,
		[]byte(`{"token":"one-time-token"}`),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), created.Token) {
		t.Fatalf("stored idempotency response leaked token: %s", stored)
	}
}

// TestServiceRejectsCrossSellerManagement verifies credential ownership.
func TestServiceRejectsCrossSellerManagement(t *testing.T) {
	t.Parallel()

	service := NewService(
		newCredentialRepository(),
		&sellerAuthorizer{err: persistence.ErrNotFound},
		&credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)},
		domain.FixedClock{
			Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
		},
	)
	_, err := service.Create(
		t.Context(),
		"other-owner",
		domain.ID(testSellerID),
		CreateCredentialRequest{
			Label:  "Laptop",
			Scopes: []Scope{ScopeRead},
		},
	)
	if !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("Create() error = %v", err)
	}
}

// TestServiceAuthenticatePropagatesRepositoryFailure preserves infrastructure errors.
func TestServiceAuthenticatePropagatesRepositoryFailure(t *testing.T) {
	t.Parallel()

	repositoryError := errors.New("credential repository unavailable")
	repository := newCredentialRepository()
	repository.getError = repositoryError
	service := NewService(
		repository,
		&sellerAuthorizer{},
		&credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)},
		domain.FixedClock{
			Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
		},
	)
	rawToken := credentialToken(
		domain.ID(testSellerID),
		domain.ID(testCredentialID),
		strings.Repeat("s", 43),
	)

	_, err := service.Authenticate(t.Context(), rawToken, ScopeRead)
	if !errors.Is(err, repositoryError) {
		t.Fatalf("Authenticate() error = %v, want %v", err, repositoryError)
	}
}

// TestServiceAuthenticatesTokenBeforeApplyingOperationScope separates identity from authorization.
func TestServiceAuthenticatesTokenBeforeApplyingOperationScope(t *testing.T) {
	t.Parallel()

	service := testCredentialService()
	created, err := service.Create(
		t.Context(),
		"owner-123",
		domain.ID(testSellerID),
		CreateCredentialRequest{
			Label:  "Configuration only",
			Scopes: []Scope{ScopeConfigure},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	principal, err := service.AuthenticateToken(t.Context(), created.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !principal.HasScope(ScopeConfigure) || principal.HasScope(ScopeRead) {
		t.Fatalf("principal scopes = %#v", principal.Scopes)
	}
	if _, err := service.Authenticate(
		t.Context(),
		created.Token,
		ScopeRead,
	); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("Authenticate() error = %v", err)
	}
}

// testCredentialService creates a deterministic credential service.
func testCredentialService() *Service {
	return NewService(
		newCredentialRepository(),
		&sellerAuthorizer{},
		&credentialIDGenerator{id: domain.ID(testCredentialID)},
		&credentialTokenGenerator{token: strings.Repeat("s", 43)},
		domain.FixedClock{
			Value: time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC),
		},
	)
}

type sellerAuthorizer struct {
	err error
}

// AuthorizeSeller returns the configured ownership result.
func (authorizer *sellerAuthorizer) AuthorizeSeller(
	_ context.Context,
	_ string,
	_ domain.ID,
) error {
	return authorizer.err
}

type credentialIDGenerator struct {
	id domain.ID
}

// New returns the configured credential identifier.
func (generator *credentialIDGenerator) New(
	_ domain.IDPrefix,
) (domain.ID, error) {
	return generator.id, nil
}

type credentialTokenGenerator struct {
	token string
}

type credentialClock struct {
	now time.Time
}

// Now returns the mutable test time.
func (clock *credentialClock) Now() time.Time {
	return clock.now
}

// NewToken returns deterministic secret material for service tests.
func (generator *credentialTokenGenerator) NewToken() (string, error) {
	return generator.token, nil
}

type credentialRepository struct {
	credentials map[domain.ID]Credential
	getError    error
}

// newCredentialRepository creates the service-test repository.
func newCredentialRepository() *credentialRepository {
	return &credentialRepository{
		credentials: make(map[domain.ID]Credential),
	}
}

// Create stores one credential without overwriting an existing identifier.
func (repository *credentialRepository) Create(
	_ context.Context,
	credential Credential,
) error {
	if _, exists := repository.credentials[credential.CredentialID()]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.credentials[credential.CredentialID()] = credential
	return nil
}

// Get loads one credential only from its owning seller scope.
func (repository *credentialRepository) Get(
	_ context.Context,
	sellerID domain.ID,
	credentialID domain.ID,
) (Credential, error) {
	if repository.getError != nil {
		return Credential{}, repository.getError
	}
	credential, exists := repository.credentials[credentialID]
	if !exists || credential.SellerID() != sellerID {
		return Credential{}, persistence.ErrNotFound
	}
	return credential, nil
}

// ListBySeller returns credentials owned by one seller.
func (repository *credentialRepository) ListBySeller(
	_ context.Context,
	sellerID domain.ID,
) ([]Credential, error) {
	credentials := make([]Credential, 0)
	for _, credential := range repository.credentials {
		if credential.SellerID() == sellerID {
			credentials = append(credentials, credential)
		}
	}
	return credentials, nil
}

// Update replaces a credential only at the expected version.
func (repository *credentialRepository) Update(
	_ context.Context,
	credential Credential,
	expectedVersion uint64,
) error {
	stored, exists := repository.credentials[credential.CredentialID()]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Version() != expectedVersion ||
		credential.Version() != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	repository.credentials[credential.CredentialID()] = credential
	return nil
}
