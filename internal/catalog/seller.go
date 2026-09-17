package catalog

import (
	"net"
	"net/url"
	"strings"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	maximumSellerNameLength    = 120
	maximumOwnerSubjectLength  = 256
	maximumSigningReferenceLen = 512
)

// SellerStatus represents whether a storefront may serve paid routes.
type SellerStatus string

const (
	SellerStatusDraft     SellerStatus = "draft"
	SellerStatusActive    SellerStatus = "active"
	SellerStatusSuspended SellerStatus = "suspended"
)

// SellerParams contains the immutable inputs required to create a seller.
type SellerParams struct {
	SellerID        domain.ID
	OwnerSubject    string
	Slug            string
	Name            string
	UpstreamBaseURL string
	CreatedAt       domain.Timestamp
}

// Seller is an API provider that publishes paid routes through AgentPay.
type Seller struct {
	SellerID         domain.ID        `json:"sellerId"`
	OwnerSubject     string           `json:"ownerSubject"`
	Slug             string           `json:"slug"`
	Name             string           `json:"name"`
	UpstreamBaseURL  string           `json:"upstreamBaseUrl"`
	SigningSecretRef string           `json:"signingSecretRef,omitempty"`
	Status           SellerStatus     `json:"status"`
	CreatedAt        domain.Timestamp `json:"createdAt"`
	UpdatedAt        domain.Timestamp `json:"updatedAt"`
	Version          uint64           `json:"version"`
}

// NewSeller validates and creates a draft seller.
func NewSeller(params SellerParams) (Seller, error) {
	validationErrors := validateSellerParams(params)
	if len(validationErrors) > 0 {
		return Seller{}, validationErrors
	}

	return Seller{
		SellerID:        params.SellerID,
		OwnerSubject:    strings.TrimSpace(params.OwnerSubject),
		Slug:            params.Slug,
		Name:            strings.TrimSpace(params.Name),
		UpstreamBaseURL: normalizeUpstreamBaseURL(params.UpstreamBaseURL),
		Status:          SellerStatusDraft,
		CreatedAt:       params.CreatedAt,
		UpdatedAt:       params.CreatedAt,
		Version:         1,
	}, nil
}

// Activate enables the seller after a signing secret has been provisioned.
func (seller *Seller) Activate(signingSecretRef string, changedAt domain.Timestamp) error {
	trimmedReference := strings.TrimSpace(signingSecretRef)
	if trimmedReference == "" || len(trimmedReference) > maximumSigningReferenceLen {
		return domain.NewValidationError("signingSecretRef", "required", "must reference a provisioned seller signing secret")
	}
	if changedAt.Before(seller.UpdatedAt) {
		return domain.NewValidationError("updatedAt", "chronology", "cannot occur before the previous update")
	}

	seller.SigningSecretRef = trimmedReference
	seller.Status = SellerStatusActive
	seller.UpdatedAt = changedAt
	seller.Version++
	return nil
}

// Suspend prevents the seller from issuing new payment challenges.
func (seller *Seller) Suspend(changedAt domain.Timestamp) error {
	if changedAt.Before(seller.UpdatedAt) {
		return domain.NewValidationError("updatedAt", "chronology", "cannot occur before the previous update")
	}

	seller.Status = SellerStatusSuspended
	seller.UpdatedAt = changedAt
	seller.Version++
	return nil
}

func validateSellerParams(params SellerParams) domain.ValidationErrors {
	var validationErrors domain.ValidationErrors
	if _, err := domain.ParseID(params.SellerID.String(), domain.SellerIDPrefix); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("sellerId", "format", "must be a seller identifier"))
	}

	ownerSubject := strings.TrimSpace(params.OwnerSubject)
	if ownerSubject == "" || len(ownerSubject) > maximumOwnerSubjectLength {
		validationErrors = append(validationErrors, domain.NewValidationError("ownerSubject", "length", "must contain 1-256 characters"))
	}

	name := strings.TrimSpace(params.Name)
	if name == "" || len(name) > maximumSellerNameLength {
		validationErrors = append(validationErrors, domain.NewValidationError("name", "length", "must contain 1-120 characters"))
	}

	if !sellerSlugPattern.MatchString(params.Slug) {
		validationErrors = append(validationErrors, domain.NewValidationError("slug", "format", "must contain 3-48 lowercase letters, digits, or hyphens"))
	}

	if err := validateUpstreamBaseURL(params.UpstreamBaseURL); err != nil {
		validationErrors = append(validationErrors, domain.NewValidationError("upstreamBaseUrl", "format", err.Error()))
	}

	if params.CreatedAt.Time().IsZero() {
		validationErrors = append(validationErrors, domain.NewValidationError("createdAt", "required", "is required"))
	}
	return validationErrors
}

func validateUpstreamBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return domain.NewValidationError("upstreamBaseUrl", "url", "must be an absolute URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return domain.NewValidationError("upstreamBaseUrl", "url", "must not contain credentials, query parameters, or fragments")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && localDevelopmentHost(parsed.Hostname()) {
		return nil
	}
	return domain.NewValidationError("upstreamBaseUrl", "scheme", "must use HTTPS except for a local development host")
}

func normalizeUpstreamBaseURL(raw string) string {
	return strings.TrimSuffix(raw, "/")
}

func localDevelopmentHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
