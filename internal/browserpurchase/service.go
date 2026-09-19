package browserpurchase

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/oklog/ulid/v2"
)

const secureTokenBytes = 32

type ProductResolver interface {
	ResolveProduct(context.Context, string, string) (Product, error)
}

type IDGenerator interface {
	New(string) (string, error)
}

type TokenGenerator interface {
	NewToken() (string, error)
}

type WalletProofVerifier interface {
	Verify(context.Context, string, string, string, string) (bool, error)
}

type Dependencies struct {
	Products       ProductResolver
	Repository     Repository
	IDGenerator    IDGenerator
	TokenGenerator TokenGenerator
	ProofVerifier  WalletProofVerifier
	Clock          domain.Clock
}

type Service struct{ dependencies Dependencies }

func NewService(dependencies Dependencies) *Service {
	if dependencies.Clock == nil {
		dependencies.Clock = domain.SystemClock{}
	}
	if dependencies.IDGenerator == nil {
		dependencies.IDGenerator = newSecureIDGenerator(dependencies.Clock, nil)
	}
	if dependencies.TokenGenerator == nil {
		dependencies.TokenGenerator = NewSecureTokenGenerator(nil)
	}
	return &Service{dependencies: dependencies}
}

func (service *Service) Create(ctx context.Context, sellerSlug, productSlug, rawIdempotencyKey string, request CreateBrowserPurchaseSessionRequest) (Creation, error) {
	if service.dependencies.Products == nil || service.dependencies.Repository == nil {
		return Creation{}, errors.New("browser purchase dependencies are unavailable")
	}
	if !validSellerSlug(sellerSlug) {
		return Creation{}, validation("sellerSlug", "must be 3-48 lowercase letters, digits, or single hyphens")
	}
	if !validProductSlug(productSlug) {
		return Creation{}, validation("productSlug", "must be 3-80 lowercase letters, digits, or single hyphens")
	}
	requestHash, maximumAmount, err := validateCreateRequest(request)
	if err != nil {
		return Creation{}, err
	}
	idempotencyKey, err := domain.ParseIdempotencyKey(rawIdempotencyKey)
	if err != nil {
		return Creation{}, validation("idempotencyKey", "must match the public idempotency-key contract")
	}
	product, err := service.dependencies.Products.ResolveProduct(ctx, sellerSlug, productSlug)
	if err != nil {
		return Creation{}, err
	}
	if product.ProductSlug != productSlug || product.SellerID == "" || product.RouteID == "" {
		return Creation{}, ErrBindingMismatch
	}
	if maximumAmount.Compare(product.Amount) < 0 {
		return Creation{}, ErrMaximumBelowQuote
	}
	creationRequestHash := hashCreationRequest(sellerSlug, productSlug, requestHash, maximumAmount.String())
	scope := sellerSlug + "/" + productSlug
	existing, found, err := service.dependencies.Repository.FindCreation(ctx, scope, idempotencyKey, creationRequestHash)
	if err != nil {
		return Creation{}, err
	}
	if found {
		return Creation{Session: existing, Replay: true}, nil
	}
	purchaseSessionID, err := service.newPurchaseSessionID()
	if err != nil {
		return Creation{}, err
	}
	browserGrant, err := service.dependencies.TokenGenerator.NewToken()
	if err != nil {
		return Creation{}, err
	}
	csrfToken, err := service.dependencies.TokenGenerator.NewToken()
	if err != nil {
		return Creation{}, err
	}
	now := domain.NewTimestamp(service.dependencies.Clock.Now())
	expiresAt := now.Add(MaximumCommerceLifetime)
	session := BrowserPurchaseSession{
		PurchaseSessionID: purchaseSessionID,
		SellerID:          product.SellerID,
		RouteID:           product.RouteID,
		ProductSlug:       product.ProductSlug,
		RequestBodyHash:   requestHash,
		MaximumAmount:     maximumAmount,
		BrowserGrantHash:  hashSecret(browserGrant),
		CSRFTokenHash:     hashSecret(csrfToken),
		Status:            StatusActive,
		CommerceExpiresAt: expiresAt,
		AccessExpiresAt:   expiresAt,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	stored, replay, err := service.dependencies.Repository.Create(ctx, session, scope, idempotencyKey, creationRequestHash)
	if err != nil {
		return Creation{}, err
	}
	if replay {
		return Creation{Session: stored, Replay: true}, nil
	}
	return Creation{Session: stored, BrowserGrant: browserGrant, CSRFToken: csrfToken}, nil
}

func (service *Service) Authorize(ctx context.Context, browserGrant string, requirement AuthorizationRequirement) (Authorization, error) {
	if len(browserGrant) < 32 || len(browserGrant) > 256 {
		return Authorization{}, ErrInvalidGrant
	}
	session, err := service.dependencies.Repository.GetByGrantHash(ctx, hashSecret(browserGrant))
	if errors.Is(err, persistence.ErrNotFound) {
		return Authorization{}, ErrInvalidGrant
	}
	if err != nil {
		return Authorization{}, err
	}
	if !secretMatches(browserGrant, session.BrowserGrantHash) {
		return Authorization{}, ErrInvalidGrant
	}
	now := service.dependencies.Clock.Now()
	switch session.Status {
	case StatusRevoked:
		return Authorization{}, ErrGrantRevoked
	case StatusExpired:
		return Authorization{}, ErrAccessExpired
	}
	switch requirement.Authority {
	case AuthorityCommerce:
		if session.Status != StatusActive {
			return Authorization{}, ErrCommerceConsumed
		}
		if !now.Before(session.CommerceExpiresAt.Time()) {
			return Authorization{}, ErrCommerceExpired
		}
	case AuthorityRead, AuthorityRemediation:
		if !now.Before(session.AccessExpiresAt.Time()) {
			return Authorization{}, ErrAccessExpired
		}
	default:
		return Authorization{}, validation("authority", "is unsupported")
	}
	if err := validateAuthorizationBinding(session, requirement); err != nil {
		return Authorization{}, err
	}
	return Authorization{PurchaseSession: session, BuyerID: "browser:" + session.PurchaseSessionID.String()}, nil
}

func (service *Service) ValidateCSRF(session BrowserPurchaseSession, csrfToken string) error {
	if len(csrfToken) < 32 || len(csrfToken) > 256 || !secretMatches(csrfToken, session.CSRFTokenHash) {
		return ErrCSRFInvalid
	}
	return nil
}

// ClaimTransaction atomically binds the one transaction created from a browser session.
func (service *Service) ClaimTransaction(ctx context.Context, purchaseSessionID PurchaseSessionID, transactionID domain.ID) (BrowserPurchaseSession, error) {
	if _, err := ParsePurchaseSessionID(purchaseSessionID.String()); err != nil {
		return BrowserPurchaseSession{}, err
	}
	if _, err := domain.ParseID(transactionID.String(), domain.TransactionIDPrefix); err != nil {
		return BrowserPurchaseSession{}, validation("transactionId", "must be a transaction ID")
	}
	session, err := service.dependencies.Repository.Get(ctx, purchaseSessionID)
	if err != nil {
		return BrowserPurchaseSession{}, err
	}
	if session.Status != StatusActive {
		return BrowserPurchaseSession{}, ErrCommerceConsumed
	}
	if !service.dependencies.Clock.Now().Before(session.CommerceExpiresAt.Time()) {
		return BrowserPurchaseSession{}, ErrCommerceExpired
	}
	claimed, err := service.dependencies.Repository.ClaimTransaction(ctx, purchaseSessionID, transactionID, domain.NewTimestamp(service.dependencies.Clock.Now()))
	if errors.Is(err, persistence.ErrConditionFailed) {
		return BrowserPurchaseSession{}, ErrCommerceConsumed
	}
	return claimed, err
}

func (service *Service) Complete(ctx context.Context, purchaseSessionID PurchaseSessionID, network, address string, transactionID domain.ID, terminalAt time.Time) error {
	if _, err := ParsePurchaseSessionID(purchaseSessionID.String()); err != nil {
		return err
	}
	if _, err := domain.ParseID(transactionID.String(), domain.TransactionIDPrefix); err != nil {
		return validation("transactionId", "must be a transaction ID")
	}
	network, address, err := normalizeWallet(network, address)
	if err != nil {
		return err
	}
	session, err := service.dependencies.Repository.Get(ctx, purchaseSessionID)
	if err != nil {
		return err
	}
	walletHash := walletBindingHash(network, address)
	if session.Status == StatusCompleted {
		if session.TransactionID != nil && *session.TransactionID == transactionID &&
			session.WalletBindingHash != nil && constantStringEqual(*session.WalletBindingHash, walletHash) {
			return nil
		}
		return ErrCommerceConsumed
	}
	if session.Status != StatusActive || session.TransactionID != nil && *session.TransactionID != transactionID {
		return ErrCommerceConsumed
	}
	if !service.dependencies.Clock.Now().Before(session.CommerceExpiresAt.Time()) {
		return ErrCommerceExpired
	}
	if terminalAt.IsZero() || terminalAt.Before(session.CreatedAt.Time()) || terminalAt.After(service.dependencies.Clock.Now()) {
		return validation("terminalAt", "must be between session creation and the current time")
	}
	session.WalletBindingHash = &walletHash
	session.TransactionID = &transactionID
	session.Status = StatusCompleted
	session.AccessExpiresAt = domain.NewTimestamp(terminalAt).Add(RemediationAccessLifetime)
	session.UpdatedAt = domain.NewTimestamp(service.dependencies.Clock.Now())
	if err := service.dependencies.Repository.Complete(ctx, session); errors.Is(err, persistence.ErrConditionFailed) {
		return ErrCommerceConsumed
	} else {
		return err
	}
}

func (service *Service) CreateRecoveryChallenge(ctx context.Context, purchaseSessionID PurchaseSessionID, request BrowserPurchaseRecoveryChallengeRequest) (BrowserPurchaseRecoveryChallenge, error) {
	if _, err := ParsePurchaseSessionID(purchaseSessionID.String()); err != nil {
		return BrowserPurchaseRecoveryChallenge{}, err
	}
	network, address, err := normalizeWallet(request.Network, request.Address)
	if err != nil {
		return BrowserPurchaseRecoveryChallenge{}, err
	}
	session, err := service.dependencies.Repository.Get(ctx, purchaseSessionID)
	if err != nil {
		return BrowserPurchaseRecoveryChallenge{}, err
	}
	if session.Status != StatusCompleted || session.WalletBindingHash == nil || session.TransactionID == nil {
		return BrowserPurchaseRecoveryChallenge{}, ErrWalletNotBound
	}
	if !service.dependencies.Clock.Now().Before(session.AccessExpiresAt.Time()) {
		return BrowserPurchaseRecoveryChallenge{}, ErrAccessExpired
	}
	if !constantStringEqual(walletBindingHash(network, address), *session.WalletBindingHash) {
		return BrowserPurchaseRecoveryChallenge{}, ErrWalletMismatch
	}
	challengeID, err := service.newRecoveryChallengeID()
	if err != nil {
		return BrowserPurchaseRecoveryChallenge{}, err
	}
	nonce, err := service.dependencies.TokenGenerator.NewToken()
	if err != nil {
		return BrowserPurchaseRecoveryChallenge{}, err
	}
	expiresAt := domain.NewTimestamp(service.dependencies.Clock.Now().Add(RecoveryChallengeLifetime))
	if session.AccessExpiresAt.Before(expiresAt) {
		expiresAt = session.AccessExpiresAt
	}
	message := recoveryMessage(purchaseSessionID, challengeID, network, address, nonce, expiresAt.Time())
	record := BrowserPurchaseRecoveryChallengeRecord{
		ChallengeID:               challengeID,
		PurchaseSessionID:         purchaseSessionID,
		ExpectedWalletBindingHash: *session.WalletBindingHash,
		Network:                   network,
		Address:                   address,
		Nonce:                     nonce,
		MessageHash:               hashSecret(message),
		ExpiresAt:                 expiresAt,
	}
	if err := service.dependencies.Repository.CreateChallenge(ctx, record); err != nil {
		return BrowserPurchaseRecoveryChallenge{}, err
	}
	return BrowserPurchaseRecoveryChallenge{ChallengeID: challengeID, Network: network, Address: address, Message: message, ExpiresAt: expiresAt}, nil
}

func (service *Service) Recover(ctx context.Context, purchaseSessionID PurchaseSessionID, request RecoverBrowserPurchaseRequest) (Recovery, error) {
	if service.dependencies.ProofVerifier == nil {
		return Recovery{}, errors.New("wallet proof verifier is unavailable")
	}
	if _, err := ParsePurchaseSessionID(purchaseSessionID.String()); err != nil {
		return Recovery{}, err
	}
	challengeID, err := ParseRecoveryChallengeID(request.ChallengeID.String())
	if err != nil {
		return Recovery{}, err
	}
	network, address, err := normalizeWallet(request.Network, request.Address)
	if err != nil {
		return Recovery{}, err
	}
	if strings.TrimSpace(request.Proof) == "" || len(request.Proof) > 4096 {
		return Recovery{}, validation("proof", "must contain at most 4096 characters")
	}
	session, err := service.dependencies.Repository.Get(ctx, purchaseSessionID)
	if err != nil {
		return Recovery{}, err
	}
	challenge, err := service.dependencies.Repository.GetChallenge(ctx, purchaseSessionID, challengeID)
	if err != nil {
		return Recovery{}, err
	}
	if challenge.PurchaseSessionID != purchaseSessionID {
		return Recovery{}, ErrBindingMismatch
	}
	if challenge.UsedAt != nil {
		return Recovery{}, ErrChallengeConsumed
	}
	now := service.dependencies.Clock.Now()
	if !now.Before(challenge.ExpiresAt.Time()) {
		return Recovery{}, ErrChallengeExpired
	}
	if !now.Before(session.AccessExpiresAt.Time()) {
		return Recovery{}, ErrAccessExpired
	}
	if session.Status != StatusCompleted || session.WalletBindingHash == nil || session.TransactionID == nil {
		return Recovery{}, ErrWalletNotBound
	}
	requestedWalletHash := walletBindingHash(network, address)
	if !constantStringEqual(requestedWalletHash, challenge.ExpectedWalletBindingHash) ||
		!constantStringEqual(requestedWalletHash, *session.WalletBindingHash) ||
		challenge.Network != network || challenge.Address != address {
		return Recovery{}, ErrWalletMismatch
	}
	message := recoveryMessage(purchaseSessionID, challengeID, network, address, challenge.Nonce, challenge.ExpiresAt.Time())
	if !secretMatches(message, challenge.MessageHash) {
		return Recovery{}, ErrBindingMismatch
	}
	verified, err := service.dependencies.ProofVerifier.Verify(ctx, network, address, message, request.Proof)
	if err != nil {
		return Recovery{}, err
	}
	if !verified {
		return Recovery{}, ErrRecoveryProofInvalid
	}
	browserGrant, err := service.dependencies.TokenGenerator.NewToken()
	if err != nil {
		return Recovery{}, err
	}
	csrfToken, err := service.dependencies.TokenGenerator.NewToken()
	if err != nil {
		return Recovery{}, err
	}
	previousGrantHash := session.BrowserGrantHash
	session.BrowserGrantHash = hashSecret(browserGrant)
	session.CSRFTokenHash = hashSecret(csrfToken)
	session.UpdatedAt = domain.NewTimestamp(now)
	usedAt := domain.NewTimestamp(now)
	challenge.UsedAt = &usedAt
	if err := service.dependencies.Repository.Recover(ctx, session, previousGrantHash, challenge); errors.Is(err, persistence.ErrConditionFailed) {
		return Recovery{}, ErrChallengeConsumed
	} else if err != nil {
		return Recovery{}, err
	}
	return Recovery{Session: session, BrowserGrant: browserGrant, CSRFToken: csrfToken}, nil
}

func validateCreateRequest(request CreateBrowserPurchaseSessionRequest) (string, domain.Amount, error) {
	if !sha256Pattern.MatchString(request.RequestBodyHash) {
		return "", domain.Amount{}, validation("requestBodyHash", "must be a lowercase SHA-256 digest")
	}
	maximumAmount, err := domain.ParseAmount(request.MaximumAmount)
	if err != nil || maximumAmount.String() != request.MaximumAmount {
		return "", domain.Amount{}, validation("maximumAmount", "must be canonical atomic units")
	}
	return request.RequestBodyHash, maximumAmount, nil
}

func validateAuthorizationBinding(session BrowserPurchaseSession, requirement AuthorizationRequirement) error {
	if requirement.SellerID != "" && requirement.SellerID != session.SellerID ||
		requirement.RouteID != "" && requirement.RouteID != session.RouteID ||
		requirement.ProductSlug != "" && requirement.ProductSlug != session.ProductSlug ||
		requirement.RequestBodyHash != "" && !constantStringEqual(requirement.RequestBodyHash, session.RequestBodyHash) ||
		requirement.TransactionID != "" && (session.TransactionID == nil || requirement.TransactionID != *session.TransactionID) {
		return ErrBindingMismatch
	}
	if requirement.MaximumAmount != "" {
		maximumAmount, err := domain.ParseAmount(requirement.MaximumAmount)
		if err != nil || maximumAmount.String() != requirement.MaximumAmount || maximumAmount.Compare(session.MaximumAmount) != 0 {
			return ErrBindingMismatch
		}
	}
	return nil
}

func normalizeWallet(network, address string) (string, string, error) {
	network = strings.TrimSpace(network)
	address = strings.TrimSpace(address)
	if network == "" || len(network) > 80 {
		return "", "", validation("network", "must contain at most 80 characters")
	}
	if address == "" || len(address) > 256 {
		return "", "", validation("address", "must contain at most 256 characters")
	}
	if strings.HasPrefix(network, "eip155:") {
		address = strings.ToLower(address)
	}
	return network, address, nil
}

func validSellerSlug(slug string) bool {
	return len(slug) >= 3 && len(slug) <= 48 && sellerSlugPattern.MatchString(slug)
}

func validProductSlug(slug string) bool {
	return len(slug) >= 3 && len(slug) <= 80 && productSlugPattern.MatchString(slug)
}

func hashCreationRequest(sellerSlug, productSlug, requestBodyHash, maximumAmount string) string {
	encoded, _ := json.Marshal(struct {
		SellerSlug      string `json:"sellerSlug"`
		ProductSlug     string `json:"productSlug"`
		RequestBodyHash string `json:"requestBodyHash"`
		MaximumAmount   string `json:"maximumAmount"`
	}{sellerSlug, productSlug, requestBodyHash, maximumAmount})
	digest := sha256.Sum256(append([]byte("agentpay.browser-purchase-create.v1\x00"), encoded...))
	return hex.EncodeToString(digest[:])
}

func walletBindingHash(network, address string) string {
	digest := sha256.Sum256([]byte("agentpay.browser-wallet.v1\x00" + network + "\x00" + address))
	return hex.EncodeToString(digest[:])
}

func recoveryMessage(sessionID PurchaseSessionID, challengeID RecoveryChallengeID, network, address, nonce string, expiresAt time.Time) string {
	return fmt.Sprintf("AgentPay browser purchase recovery\nVersion: 1\nPurchase Session: %s\nChallenge: %s\nNetwork: %s\nAddress: %s\nNonce: %s\nExpires At: %s", sessionID, challengeID, network, address, nonce, expiresAt.UTC().Format(time.RFC3339Nano))
}

func hashSecret(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

func secretMatches(raw, expectedHash string) bool {
	expected, err := hex.DecodeString(expectedHash)
	if err != nil || len(expected) != sha256.Size {
		return false
	}
	actual := sha256.Sum256([]byte(raw))
	return subtle.ConstantTimeCompare(expected, actual[:]) == 1
}

func constantStringEqual(left, right string) bool {
	if len(left) != len(right) || len(left) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func (service *Service) newPurchaseSessionID() (PurchaseSessionID, error) {
	raw, err := service.dependencies.IDGenerator.New(PurchaseSessionIDPrefix)
	if err != nil {
		return "", err
	}
	return ParsePurchaseSessionID(raw)
}

func (service *Service) newRecoveryChallengeID() (RecoveryChallengeID, error) {
	raw, err := service.dependencies.IDGenerator.New(RecoveryChallengeIDPrefix)
	if err != nil {
		return "", err
	}
	return ParseRecoveryChallengeID(raw)
}

type SecureTokenGenerator struct{ reader io.Reader }

func NewSecureTokenGenerator(reader io.Reader) *SecureTokenGenerator {
	if reader == nil {
		reader = cryptorand.Reader
	}
	return &SecureTokenGenerator{reader: reader}
}

func (generator *SecureTokenGenerator) NewToken() (string, error) {
	randomBytes := make([]byte, secureTokenBytes)
	if _, err := io.ReadFull(generator.reader, randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

type secureIDGenerator struct {
	clock   domain.Clock
	entropy io.Reader
	mutex   sync.Mutex
}

func newSecureIDGenerator(clock domain.Clock, entropy io.Reader) *secureIDGenerator {
	if entropy == nil {
		entropy = ulid.Monotonic(cryptorand.Reader, 0)
	}
	return &secureIDGenerator{clock: clock, entropy: entropy}
}

func (generator *secureIDGenerator) New(prefix string) (string, error) {
	if prefix != PurchaseSessionIDPrefix && prefix != RecoveryChallengeIDPrefix {
		return "", validation("prefix", "is unsupported")
	}
	generator.mutex.Lock()
	defer generator.mutex.Unlock()
	identifier, err := ulid.New(ulid.Timestamp(generator.clock.Now()), generator.entropy)
	if err != nil {
		return "", err
	}
	return prefix + identifier.String(), nil
}
