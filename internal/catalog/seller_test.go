package catalog

import (
	"strings"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
)

func TestNewSeller(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.FixedZone("IST", 5*60*60+30*60)))
	seller, err := NewSeller(SellerParams{
		SellerID:        mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		OwnerSubject:    "cognito-user-123",
		Slug:            "acme-research",
		Name:            "Acme Research",
		UpstreamBaseURL: "https://seller.example/api/",
		CreatedAt:       createdAt,
	})
	if err != nil {
		t.Fatalf("NewSeller() error = %v", err)
	}

	if seller.Status != SellerStatusDraft {
		t.Fatalf("Status = %q, want %q", seller.Status, SellerStatusDraft)
	}
	if seller.UpstreamBaseURL != "https://seller.example/api" {
		t.Fatalf("UpstreamBaseURL = %q", seller.UpstreamBaseURL)
	}
	if seller.CreatedAt.Time().Location() != time.UTC || seller.UpdatedAt != seller.CreatedAt {
		t.Fatalf("timestamps were not initialized in UTC: created=%s updated=%s", seller.CreatedAt, seller.UpdatedAt)
	}
	if seller.Version != 1 {
		t.Fatalf("Version = %d, want 1", seller.Version)
	}
}

func TestNewSellerValidation(t *testing.T) {
	t.Parallel()

	valid := SellerParams{
		SellerID:        mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		OwnerSubject:    "cognito-user-123",
		Slug:            "acme-research",
		Name:            "Acme Research",
		UpstreamBaseURL: "https://seller.example/api",
		CreatedAt:       domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC)),
	}

	tests := []struct {
		name      string
		mutate    func(*SellerParams)
		wantField string
	}{
		{name: "seller ID prefix", mutate: func(params *SellerParams) {
			params.SellerID = mustCatalogID(t, "txn_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.TransactionIDPrefix)
		}, wantField: "sellerId"},
		{name: "owner required", mutate: func(params *SellerParams) { params.OwnerSubject = "" }, wantField: "ownerSubject"},
		{name: "name required", mutate: func(params *SellerParams) { params.Name = "" }, wantField: "name"},
		{name: "name too long", mutate: func(params *SellerParams) { params.Name = strings.Repeat("n", 121) }, wantField: "name"},
		{name: "slug format", mutate: func(params *SellerParams) { params.Slug = "Acme Research" }, wantField: "slug"},
		{name: "invalid URL", mutate: func(params *SellerParams) { params.UpstreamBaseURL = "://bad" }, wantField: "upstreamBaseUrl"},
		{name: "external HTTP rejected", mutate: func(params *SellerParams) { params.UpstreamBaseURL = "http://seller.example/api" }, wantField: "upstreamBaseUrl"},
		{name: "URL credentials rejected", mutate: func(params *SellerParams) { params.UpstreamBaseURL = "https://user:pass@seller.example/api" }, wantField: "upstreamBaseUrl"},
		{name: "URL query rejected", mutate: func(params *SellerParams) { params.UpstreamBaseURL = "https://seller.example/api?token=secret" }, wantField: "upstreamBaseUrl"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			params := valid
			test.mutate(&params)
			_, err := NewSeller(params)
			assertCatalogValidationField(t, err, test.wantField)
		})
	}
}

func TestNewSellerAllowsLocalHTTPForDevelopment(t *testing.T) {
	t.Parallel()

	_, err := NewSeller(SellerParams{
		SellerID:        mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		OwnerSubject:    "cognito-user-123",
		Slug:            "local-seller",
		Name:            "Local Seller",
		UpstreamBaseURL: "http://localhost:8090",
		CreatedAt:       domain.NewTimestamp(time.Now()),
	})
	if err != nil {
		t.Fatalf("NewSeller() rejected local development URL: %v", err)
	}
}

func TestSellerActivationAndSuspension(t *testing.T) {
	t.Parallel()

	createdAt := domain.NewTimestamp(time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC))
	seller, err := NewSeller(SellerParams{
		SellerID:        mustCatalogID(t, "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix),
		OwnerSubject:    "cognito-user-123",
		Slug:            "acme-research",
		Name:            "Acme Research",
		UpstreamBaseURL: "https://seller.example/api",
		CreatedAt:       createdAt,
	})
	if err != nil {
		t.Fatalf("NewSeller() error = %v", err)
	}

	if err := seller.Activate("", createdAt.Add(time.Minute)); err == nil {
		t.Fatal("Activate() accepted an empty signing secret reference")
	}
	if seller.Status != SellerStatusDraft || seller.Version != 1 {
		t.Fatal("failed activation mutated seller")
	}

	activatedAt := createdAt.Add(time.Minute)
	if err := seller.Activate("arn:aws:secretsmanager:us-east-1:123456789012:secret:seller/acme", activatedAt); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if seller.Status != SellerStatusActive || seller.SigningSecretRef == "" || seller.UpdatedAt != activatedAt || seller.Version != 2 {
		t.Fatalf("activated seller = %#v", seller)
	}

	suspendedAt := activatedAt.Add(time.Minute)
	if err := seller.Suspend(suspendedAt); err != nil {
		t.Fatalf("Suspend() error = %v", err)
	}
	if seller.Status != SellerStatusSuspended || seller.UpdatedAt != suspendedAt || seller.Version != 3 {
		t.Fatalf("suspended seller = %#v", seller)
	}
}
