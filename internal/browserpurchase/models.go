package browserpurchase

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/oklog/ulid/v2"
)

const (
	PurchaseSessionIDPrefix   = "bps_"
	RecoveryChallengeIDPrefix = "bpr_"
	PurchaseChannelBrowser    = "browser"
	MaximumCommerceLifetime   = 10 * time.Minute
	RecoveryChallengeLifetime = 5 * time.Minute
	RemediationAccessLifetime = 30 * 24 * time.Hour
)

var (
	sha256Pattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	sellerSlugPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	productSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

	ErrValidation           = errors.New("browser purchase validation failed")
	ErrMaximumBelowQuote    = errors.New("maximum amount is below the seller quote")
	ErrIdempotencyConflict  = errors.New("idempotency key was reused with different purchase parameters")
	ErrInvalidGrant         = errors.New("browser purchase grant is invalid")
	ErrGrantRevoked         = errors.New("browser purchase grant is revoked")
	ErrCommerceExpired      = errors.New("browser purchase commerce authority expired")
	ErrAccessExpired        = errors.New("browser purchase access expired")
	ErrCommerceConsumed     = errors.New("browser purchase commerce authority was consumed")
	ErrBindingMismatch      = errors.New("browser purchase binding does not match")
	ErrOriginDenied         = errors.New("browser purchase origin is denied")
	ErrCSRFInvalid          = errors.New("browser purchase CSRF token is invalid")
	ErrWalletNotBound       = errors.New("browser purchase has no finalized payer wallet")
	ErrWalletMismatch       = errors.New("wallet does not match the finalized payer")
	ErrChallengeExpired     = errors.New("browser purchase recovery challenge expired")
	ErrChallengeConsumed    = errors.New("browser purchase recovery challenge was already consumed")
	ErrRecoveryProofInvalid = errors.New("browser purchase recovery proof is invalid")
)

type PurchaseSessionID string

func (identifier PurchaseSessionID) String() string { return string(identifier) }

func ParsePurchaseSessionID(raw string) (PurchaseSessionID, error) {
	if !strings.HasPrefix(raw, PurchaseSessionIDPrefix) {
		return "", validation("purchaseSessionId", "must use the bps_ prefix")
	}
	if _, err := ulid.ParseStrict(strings.TrimPrefix(raw, PurchaseSessionIDPrefix)); err != nil {
		return "", validation("purchaseSessionId", "must contain a canonical ULID")
	}
	return PurchaseSessionID(raw), nil
}

type RecoveryChallengeID string

func (identifier RecoveryChallengeID) String() string { return string(identifier) }

func ParseRecoveryChallengeID(raw string) (RecoveryChallengeID, error) {
	if !strings.HasPrefix(raw, RecoveryChallengeIDPrefix) {
		return "", validation("challengeId", "must use the bpr_ prefix")
	}
	if _, err := ulid.ParseStrict(strings.TrimPrefix(raw, RecoveryChallengeIDPrefix)); err != nil {
		return "", validation("challengeId", "must contain a canonical ULID")
	}
	return RecoveryChallengeID(raw), nil
}

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusExpired   Status = "expired"
	StatusRevoked   Status = "revoked"
)

type BrowserPurchaseSession struct {
	PurchaseSessionID PurchaseSessionID
	SellerID          domain.ID
	RouteID           domain.ID
	ProductSlug       string
	RequestBodyHash   string
	MaximumAmount     domain.Amount
	BrowserGrantHash  string
	CSRFTokenHash     string
	WalletBindingHash *string
	TransactionID     *domain.ID
	Status            Status
	CommerceExpiresAt domain.Timestamp
	AccessExpiresAt   domain.Timestamp
	CreatedAt         domain.Timestamp
	UpdatedAt         domain.Timestamp
}

type BrowserPurchaseRecoveryChallengeRecord struct {
	ChallengeID               RecoveryChallengeID
	PurchaseSessionID         PurchaseSessionID
	ExpectedWalletBindingHash string
	Network                   string
	Address                   string
	Nonce                     string
	MessageHash               string
	ExpiresAt                 domain.Timestamp
	UsedAt                    *domain.Timestamp
}

type Product struct {
	SellerID    domain.ID
	RouteID     domain.ID
	ProductSlug string
	Amount      domain.Amount
}

type CreateBrowserPurchaseSessionRequest struct {
	RequestBodyHash string `json:"requestBodyHash"`
	MaximumAmount   string `json:"maximumAmount"`
}

type BrowserPurchaseSessionCreated struct {
	PurchaseSessionID PurchaseSessionID `json:"purchaseSessionId"`
	SellerID          domain.ID         `json:"sellerId"`
	RouteID           domain.ID         `json:"routeId"`
	ProductSlug       string            `json:"productSlug"`
	PurchaseChannel   string            `json:"purchaseChannel"`
	CommerceExpiresAt domain.Timestamp  `json:"commerceExpiresAt"`
	AccessExpiresAt   domain.Timestamp  `json:"accessExpiresAt"`
}

type BrowserPurchaseRecoveryChallengeRequest struct {
	Network string `json:"network"`
	Address string `json:"address"`
}

type BrowserPurchaseRecoveryChallenge struct {
	ChallengeID RecoveryChallengeID `json:"challengeId"`
	Network     string              `json:"network"`
	Address     string              `json:"address"`
	Message     string              `json:"message"`
	ExpiresAt   domain.Timestamp    `json:"expiresAt"`
}

type RecoverBrowserPurchaseRequest struct {
	ChallengeID RecoveryChallengeID `json:"challengeId"`
	Network     string              `json:"network"`
	Address     string              `json:"address"`
	Proof       string              `json:"proof"`
}

type Creation struct {
	Session      BrowserPurchaseSession
	BrowserGrant string
	CSRFToken    string
	Replay       bool
}

func (creation Creation) Response() BrowserPurchaseSessionCreated {
	return BrowserPurchaseSessionCreated{
		PurchaseSessionID: creation.Session.PurchaseSessionID,
		SellerID:          creation.Session.SellerID,
		RouteID:           creation.Session.RouteID,
		ProductSlug:       creation.Session.ProductSlug,
		PurchaseChannel:   PurchaseChannelBrowser,
		CommerceExpiresAt: creation.Session.CommerceExpiresAt,
		AccessExpiresAt:   creation.Session.AccessExpiresAt,
	}
}

type Recovery struct {
	Session      BrowserPurchaseSession
	BrowserGrant string
	CSRFToken    string
}

type Authority string

const (
	AuthorityCommerce    Authority = "commerce"
	AuthorityRead        Authority = "read"
	AuthorityRemediation Authority = "remediation"
)

type AuthorizationRequirement struct {
	Authority       Authority
	Mutation        bool
	SellerID        domain.ID
	RouteID         domain.ID
	ProductSlug     string
	RequestBodyHash string
	MaximumAmount   string
	TransactionID   domain.ID
}

type Authorization struct {
	PurchaseSession BrowserPurchaseSession
	BuyerID         string
}

func validation(field, message string) error {
	return &browserValidationError{field: field, message: message}
}

type browserValidationError struct {
	field   string
	message string
}

func (validationError *browserValidationError) Error() string {
	return validationError.field + ": " + validationError.message
}

func (validationError *browserValidationError) Unwrap() error { return ErrValidation }
