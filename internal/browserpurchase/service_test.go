package browserpurchase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestServiceCreateBindsProductRequestAndMaximum(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()

	created, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", CreateBrowserPurchaseSessionRequest{
		RequestBodyHash: fixture.requestBodyHash,
		MaximumAmount:   "150",
	})
	if err != nil {
		t.Fatal(err)
	}

	if created.Session.PurchaseSessionID != "bps_01K5D09YJ0C0M7RJM4FWQ0K9HA" ||
		created.Session.SellerID != fixture.product.SellerID ||
		created.Session.RouteID != fixture.product.RouteID ||
		created.Session.ProductSlug != fixture.product.ProductSlug ||
		created.Session.RequestBodyHash != fixture.requestBodyHash ||
		created.Session.MaximumAmount.String() != "150" ||
		created.Session.Status != StatusActive {
		t.Fatalf("unexpected session: %#v", created.Session)
	}
	if created.BrowserGrant != fixture.browserGrant || created.CSRFToken != fixture.csrfToken {
		t.Fatalf("unexpected raw credentials: %#v", created)
	}
	if created.Session.BrowserGrantHash == fixture.browserGrant || created.Session.CSRFTokenHash == fixture.csrfToken {
		t.Fatal("raw browser credentials were stored")
	}
	if got := created.Session.CommerceExpiresAt.Time(); !got.Equal(fixture.now.Add(MaximumCommerceLifetime)) {
		t.Fatalf("commerce expiry = %v", got)
	}
	if created.Session.AccessExpiresAt != created.Session.CommerceExpiresAt {
		t.Fatal("initial access expiry must equal commerce expiry")
	}

	stored, err := fixture.repository.Get(t.Context(), created.Session.PurchaseSessionID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.BrowserGrantHash != created.Session.BrowserGrantHash || stored.CSRFTokenHash != created.Session.CSRFTokenHash {
		t.Fatalf("stored session mismatch: %#v", stored)
	}
}

func TestServiceCreateRejectsInvalidOrUnsafeInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(*CreateBrowserPurchaseSessionRequest)
		wantErr error
	}{
		{name: "invalid request hash", mutate: func(request *CreateBrowserPurchaseSessionRequest) { request.RequestBodyHash = "ABC" }, wantErr: ErrValidation},
		{name: "invalid maximum", mutate: func(request *CreateBrowserPurchaseSessionRequest) { request.MaximumAmount = "1.5" }, wantErr: ErrValidation},
		{name: "maximum below quote", mutate: func(request *CreateBrowserPurchaseSessionRequest) { request.MaximumAmount = "99" }, wantErr: ErrMaximumBelowQuote},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newServiceFixture()
			request := CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"}
			test.mutate(&request)
			_, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", request)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Create() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestServiceCreateIsIdempotentAndRejectsChangedReplay(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	request := CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"}
	first, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", request)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replay || second.Session.PurchaseSessionID != first.Session.PurchaseSessionID {
		t.Fatalf("unexpected replay: %#v", second)
	}
	if second.BrowserGrant != "" || second.CSRFToken != "" {
		t.Fatal("idempotent replay must not expose or rotate credentials")
	}
	request.MaximumAmount = "151"
	if _, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", request); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed replay error = %v", err)
	}
}

func TestServiceAuthenticatesAndAuthorizesExactBindings(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	created, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"})
	if err != nil {
		t.Fatal(err)
	}

	authorization, err := fixture.service.Authorize(t.Context(), fixture.browserGrant, AuthorizationRequirement{
		Authority:       AuthorityCommerce,
		SellerID:        fixture.product.SellerID,
		RouteID:         fixture.product.RouteID,
		ProductSlug:     fixture.product.ProductSlug,
		RequestBodyHash: fixture.requestBodyHash,
		MaximumAmount:   "150",
	})
	if err != nil {
		t.Fatal(err)
	}
	if authorization.BuyerID != "browser:"+created.Session.PurchaseSessionID.String() {
		t.Fatalf("buyer ID = %q", authorization.BuyerID)
	}

	tests := []struct {
		name   string
		mutate func(*AuthorizationRequirement)
	}{
		{name: "seller", mutate: func(requirement *AuthorizationRequirement) {
			requirement.SellerID = domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9HB")
		}},
		{name: "route", mutate: func(requirement *AuthorizationRequirement) {
			requirement.RouteID = domain.ID("rte_01K5D09YJ0C0M7RJM4FWQ0K9HB")
		}},
		{name: "product", mutate: func(requirement *AuthorizationRequirement) { requirement.ProductSlug = "other-product" }},
		{name: "body", mutate: func(requirement *AuthorizationRequirement) {
			requirement.RequestBodyHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{name: "maximum", mutate: func(requirement *AuthorizationRequirement) { requirement.MaximumAmount = "151" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requirement := AuthorizationRequirement{Authority: AuthorityCommerce, SellerID: fixture.product.SellerID, RouteID: fixture.product.RouteID, ProductSlug: fixture.product.ProductSlug, RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"}
			test.mutate(&requirement)
			if _, err := fixture.service.Authorize(t.Context(), fixture.browserGrant, requirement); !errors.Is(err, ErrBindingMismatch) {
				t.Fatalf("Authorize() error = %v", err)
			}
		})
	}
}

func TestServiceAuthorizationHonorsExclusiveExpiry(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	_, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"})
	if err != nil {
		t.Fatal(err)
	}
	fixture.clock.Value = fixture.now.Add(MaximumCommerceLifetime)
	_, err = fixture.service.Authorize(t.Context(), fixture.browserGrant, AuthorizationRequirement{Authority: AuthorityCommerce})
	if !errors.Is(err, ErrCommerceExpired) {
		t.Fatalf("Authorize() error = %v", err)
	}
	_, err = fixture.service.Authorize(t.Context(), fixture.browserGrant, AuthorizationRequirement{Authority: AuthorityRead})
	if !errors.Is(err, ErrAccessExpired) {
		t.Fatalf("read Authorize() error = %v", err)
	}
}

func TestServiceClaimsExactlyOneTransactionDuringCommerceWindow(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	created, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"})
	if err != nil {
		t.Fatal(err)
	}
	transactionID := domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HA")
	claimed, err := fixture.service.ClaimTransaction(t.Context(), created.Session.PurchaseSessionID, transactionID)
	if err != nil || claimed.TransactionID == nil || *claimed.TransactionID != transactionID {
		t.Fatalf("ClaimTransaction() = %#v, %v", claimed, err)
	}
	if _, err := fixture.service.ClaimTransaction(t.Context(), created.Session.PurchaseSessionID, transactionID); err != nil {
		t.Fatalf("idempotent claim error = %v", err)
	}
	if _, err := fixture.service.ClaimTransaction(t.Context(), created.Session.PurchaseSessionID, domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HB")); !errors.Is(err, ErrCommerceConsumed) {
		t.Fatalf("second transaction claim error = %v", err)
	}
	fixture.clock.Value = fixture.now.Add(MaximumCommerceLifetime)
	if err := fixture.service.Complete(t.Context(), created.Session.PurchaseSessionID, "eip155:84532", "0x1111111111111111111111111111111111111111", transactionID, fixture.clock.Value); !errors.Is(err, ErrCommerceExpired) {
		t.Fatalf("expired Complete() error = %v", err)
	}
}

func TestServiceRecoveryRotatesGrantWithoutRestoringCommerce(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	created, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"})
	if err != nil {
		t.Fatal(err)
	}
	terminalAt := fixture.now
	if err := fixture.service.Complete(t.Context(), created.Session.PurchaseSessionID, "eip155:84532", "0x1111111111111111111111111111111111111111", domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HA"), terminalAt); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.Complete(t.Context(), created.Session.PurchaseSessionID, "eip155:84532", "0x1111111111111111111111111111111111111111", domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HA"), terminalAt); err != nil {
		t.Fatalf("idempotent Complete() error = %v", err)
	}
	if err := fixture.service.Complete(t.Context(), created.Session.PurchaseSessionID, "eip155:84532", "0x2222222222222222222222222222222222222222", domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HA"), terminalAt); !errors.Is(err, ErrCommerceConsumed) {
		t.Fatalf("changed Complete() error = %v", err)
	}

	challenge, err := fixture.service.CreateRecoveryChallenge(t.Context(), created.Session.PurchaseSessionID, BrowserPurchaseRecoveryChallengeRequest{Network: "eip155:84532", Address: "0x1111111111111111111111111111111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Message == "" || challenge.ChallengeID == "" || !challenge.ExpiresAt.Time().Equal(fixture.now.Add(RecoveryChallengeLifetime)) {
		t.Fatalf("unexpected challenge: %#v", challenge)
	}
	fixture.proofVerifier.expectedMessage = challenge.Message
	fixture.tokenGenerator.values = append(fixture.tokenGenerator.values, fixture.recoveredGrant, fixture.recoveredCSRF)
	recovered, err := fixture.service.Recover(t.Context(), created.Session.PurchaseSessionID, RecoverBrowserPurchaseRequest{
		ChallengeID: challenge.ChallengeID,
		Network:     challenge.Network,
		Address:     challenge.Address,
		Proof:       "0xsigned-proof",
	})
	if err != nil {
		t.Fatal(err)
	}
	if recovered.BrowserGrant != fixture.recoveredGrant || recovered.CSRFToken != fixture.recoveredCSRF {
		t.Fatalf("unexpected recovered credentials: %#v", recovered)
	}
	if _, err := fixture.service.Authorize(t.Context(), fixture.browserGrant, AuthorizationRequirement{Authority: AuthorityRead}); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("old grant error = %v", err)
	}
	if _, err := fixture.service.Authorize(t.Context(), fixture.recoveredGrant, AuthorizationRequirement{Authority: AuthorityRead, TransactionID: domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HA")}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Authorize(t.Context(), fixture.recoveredGrant, AuthorizationRequirement{Authority: AuthorityCommerce}); !errors.Is(err, ErrCommerceConsumed) {
		t.Fatalf("recovered commerce error = %v", err)
	}
	if _, err := fixture.service.Recover(t.Context(), created.Session.PurchaseSessionID, RecoverBrowserPurchaseRequest{ChallengeID: challenge.ChallengeID, Network: challenge.Network, Address: challenge.Address, Proof: "0xsigned-proof"}); !errors.Is(err, ErrChallengeConsumed) {
		t.Fatalf("replayed recovery error = %v", err)
	}
}

func TestServiceRecoveryRejectsWrongWalletAndExpiredAccess(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	created, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.Complete(t.Context(), created.Session.PurchaseSessionID, "eip155:84532", "0x1111111111111111111111111111111111111111", domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HA"), fixture.now); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.CreateRecoveryChallenge(t.Context(), created.Session.PurchaseSessionID, BrowserPurchaseRecoveryChallengeRequest{Network: "eip155:84532", Address: "0x2222222222222222222222222222222222222222"}); !errors.Is(err, ErrWalletMismatch) {
		t.Fatalf("wrong wallet error = %v", err)
	}
	fixture.clock.Value = fixture.now.Add(RemediationAccessLifetime)
	if _, err := fixture.service.CreateRecoveryChallenge(t.Context(), created.Session.PurchaseSessionID, BrowserPurchaseRecoveryChallengeRequest{Network: "eip155:84532", Address: "0x1111111111111111111111111111111111111111"}); !errors.Is(err, ErrAccessExpired) {
		t.Fatalf("expired access error = %v", err)
	}
}

func TestRecoveryChallengeDoesNotOutliveRemainingAccess(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture()
	created, err := fixture.service.Create(t.Context(), "demo-store", "research-report", "checkout-0001", CreateBrowserPurchaseSessionRequest{RequestBodyHash: fixture.requestBodyHash, MaximumAmount: "150"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.Complete(t.Context(), created.Session.PurchaseSessionID, "eip155:84532", "0x1111111111111111111111111111111111111111", domain.ID("txn_01K5D09YJ0C0M7RJM4FWQ0K9HA"), fixture.now); err != nil {
		t.Fatal(err)
	}
	fixture.clock.Value = fixture.now.Add(RemediationAccessLifetime - 2*time.Minute)
	challenge, err := fixture.service.CreateRecoveryChallenge(t.Context(), created.Session.PurchaseSessionID, BrowserPurchaseRecoveryChallengeRequest{Network: "eip155:84532", Address: "0x1111111111111111111111111111111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	if !challenge.ExpiresAt.Time().Equal(fixture.now.Add(RemediationAccessLifetime)) {
		t.Fatalf("challenge expiry = %v", challenge.ExpiresAt.Time())
	}
}

type serviceFixture struct {
	service         *Service
	repository      *testRepository
	clock           *mutableClock
	tokenGenerator  *sequenceTokenGenerator
	proofVerifier   *testProofVerifier
	now             time.Time
	product         Product
	requestBodyHash string
	browserGrant    string
	csrfToken       string
	recoveredGrant  string
	recoveredCSRF   string
}

func newServiceFixture() serviceFixture {
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	product := Product{SellerID: domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"), RouteID: domain.ID("rte_01K5D09YJ0C0M7RJM4FWQ0K9H8"), ProductSlug: "research-report", Amount: domain.MustParseAmount("100")}
	repository := newTestRepository()
	clock := &mutableClock{Value: now}
	tokens := &sequenceTokenGenerator{values: []string{"browser-grant-value-with-at-least-32-bytes", "csrf-token-value-with-at-least-32-bytes", "recovery-nonce-value-with-at-least-32-bytes"}}
	verifier := &testProofVerifier{valid: true}
	service := NewService(Dependencies{
		Products:       staticProductResolver{product: product},
		Repository:     repository,
		IDGenerator:    &sequenceIDGenerator{values: []string{"bps_01K5D09YJ0C0M7RJM4FWQ0K9HA", "bpr_01K5D09YJ0C0M7RJM4FWQ0K9HB"}},
		TokenGenerator: tokens,
		ProofVerifier:  verifier,
		Clock:          clock,
	})
	return serviceFixture{service: service, repository: repository, clock: clock, tokenGenerator: tokens, proofVerifier: verifier, now: now, product: product, requestBodyHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", browserGrant: "browser-grant-value-with-at-least-32-bytes", csrfToken: "csrf-token-value-with-at-least-32-bytes", recoveredGrant: "recovered-browser-grant-with-at-least-32-bytes", recoveredCSRF: "recovered-csrf-token-with-at-least-32-bytes"}
}

type mutableClock struct{ Value time.Time }

func (clock *mutableClock) Now() time.Time { return clock.Value }

type staticProductResolver struct {
	product Product
	err     error
}

func (resolver staticProductResolver) ResolveProduct(_ context.Context, sellerSlug, productSlug string) (Product, error) {
	if resolver.err != nil {
		return Product{}, resolver.err
	}
	if sellerSlug != "demo-store" || productSlug != resolver.product.ProductSlug {
		return Product{}, persistence.ErrNotFound
	}
	return resolver.product, nil
}

type sequenceIDGenerator struct {
	values []string
	index  int
}

func (generator *sequenceIDGenerator) New(prefix string) (string, error) {
	if generator.index >= len(generator.values) {
		return "", errors.New("no test IDs remain")
	}
	value := generator.values[generator.index]
	generator.index++
	if len(value) < len(prefix) || value[:len(prefix)] != prefix {
		return "", errors.New("unexpected test ID prefix")
	}
	return value, nil
}

type sequenceTokenGenerator struct{ values []string }

func (generator *sequenceTokenGenerator) NewToken() (string, error) {
	if len(generator.values) == 0 {
		return "", errors.New("no test tokens remain")
	}
	value := generator.values[0]
	generator.values = generator.values[1:]
	return value, nil
}

type testProofVerifier struct {
	valid           bool
	expectedMessage string
}

func (verifier *testProofVerifier) Verify(_ context.Context, network, address, message, proof string) (bool, error) {
	if network != "eip155:84532" || address != "0x1111111111111111111111111111111111111111" || proof != "0xsigned-proof" {
		return false, nil
	}
	return verifier.valid && message == verifier.expectedMessage, nil
}

type testRepository struct {
	sessions    map[PurchaseSessionID]BrowserPurchaseSession
	grantIndex  map[string]PurchaseSessionID
	idempotency map[string]creationRecord
	challenges  map[RecoveryChallengeID]BrowserPurchaseRecoveryChallengeRecord
}

type creationRecord struct {
	requestHash string
	sessionID   PurchaseSessionID
}

func newTestRepository() *testRepository {
	return &testRepository{sessions: map[PurchaseSessionID]BrowserPurchaseSession{}, grantIndex: map[string]PurchaseSessionID{}, idempotency: map[string]creationRecord{}, challenges: map[RecoveryChallengeID]BrowserPurchaseRecoveryChallengeRecord{}}
}

func (repository *testRepository) Create(_ context.Context, session BrowserPurchaseSession, scope string, key domain.IdempotencyKey, requestHash string) (BrowserPurchaseSession, bool, error) {
	indexKey := scope + "\x00" + string(key)
	if record, exists := repository.idempotency[indexKey]; exists {
		if record.requestHash != requestHash {
			return BrowserPurchaseSession{}, false, ErrIdempotencyConflict
		}
		return repository.sessions[record.sessionID], true, nil
	}
	if _, exists := repository.sessions[session.PurchaseSessionID]; exists {
		return BrowserPurchaseSession{}, false, persistence.ErrAlreadyExists
	}
	repository.sessions[session.PurchaseSessionID] = session
	repository.grantIndex[session.BrowserGrantHash] = session.PurchaseSessionID
	repository.idempotency[indexKey] = creationRecord{requestHash: requestHash, sessionID: session.PurchaseSessionID}
	return session, false, nil
}

func (repository *testRepository) FindCreation(_ context.Context, scope string, key domain.IdempotencyKey, requestHash string) (BrowserPurchaseSession, bool, error) {
	record, exists := repository.idempotency[scope+"\x00"+string(key)]
	if !exists {
		return BrowserPurchaseSession{}, false, nil
	}
	if record.requestHash != requestHash {
		return BrowserPurchaseSession{}, false, ErrIdempotencyConflict
	}
	return repository.sessions[record.sessionID], true, nil
}

func (repository *testRepository) Get(_ context.Context, sessionID PurchaseSessionID) (BrowserPurchaseSession, error) {
	session, exists := repository.sessions[sessionID]
	if !exists {
		return BrowserPurchaseSession{}, persistence.ErrNotFound
	}
	return session, nil
}

func (repository *testRepository) GetByGrantHash(_ context.Context, grantHash string) (BrowserPurchaseSession, error) {
	sessionID, exists := repository.grantIndex[grantHash]
	if !exists {
		return BrowserPurchaseSession{}, persistence.ErrNotFound
	}
	return repository.sessions[sessionID], nil
}

func (repository *testRepository) Complete(_ context.Context, session BrowserPurchaseSession) error {
	stored, exists := repository.sessions[session.PurchaseSessionID]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Status != StatusActive || stored.TransactionID != nil && (session.TransactionID == nil || *stored.TransactionID != *session.TransactionID) {
		return persistence.ErrConditionFailed
	}
	repository.sessions[session.PurchaseSessionID] = session
	return nil
}

func (repository *testRepository) ClaimTransaction(_ context.Context, purchaseSessionID PurchaseSessionID, transactionID domain.ID, updatedAt domain.Timestamp) (BrowserPurchaseSession, error) {
	session, exists := repository.sessions[purchaseSessionID]
	if !exists {
		return BrowserPurchaseSession{}, persistence.ErrNotFound
	}
	if session.TransactionID != nil {
		if *session.TransactionID == transactionID {
			return session, nil
		}
		return BrowserPurchaseSession{}, persistence.ErrConditionFailed
	}
	session.TransactionID = &transactionID
	session.UpdatedAt = updatedAt
	repository.sessions[purchaseSessionID] = session
	return session, nil
}

func (repository *testRepository) CreateChallenge(_ context.Context, challenge BrowserPurchaseRecoveryChallengeRecord) error {
	if _, exists := repository.challenges[challenge.ChallengeID]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.challenges[challenge.ChallengeID] = challenge
	return nil
}

func (repository *testRepository) GetChallenge(_ context.Context, purchaseSessionID PurchaseSessionID, challengeID RecoveryChallengeID) (BrowserPurchaseRecoveryChallengeRecord, error) {
	challenge, exists := repository.challenges[challengeID]
	if !exists || challenge.PurchaseSessionID != purchaseSessionID {
		return BrowserPurchaseRecoveryChallengeRecord{}, persistence.ErrNotFound
	}
	return challenge, nil
}

func (repository *testRepository) Recover(_ context.Context, session BrowserPurchaseSession, previousGrantHash string, challenge BrowserPurchaseRecoveryChallengeRecord) error {
	storedSession, sessionExists := repository.sessions[session.PurchaseSessionID]
	storedChallenge, challengeExists := repository.challenges[challenge.ChallengeID]
	if !sessionExists || !challengeExists {
		return persistence.ErrNotFound
	}
	if storedSession.BrowserGrantHash != previousGrantHash || storedChallenge.UsedAt != nil || challenge.UsedAt == nil {
		return persistence.ErrConditionFailed
	}
	delete(repository.grantIndex, previousGrantHash)
	repository.sessions[session.PurchaseSessionID] = session
	repository.grantIndex[session.BrowserGrantHash] = session.PurchaseSessionID
	repository.challenges[challenge.ChallengeID] = challenge
	return nil
}
