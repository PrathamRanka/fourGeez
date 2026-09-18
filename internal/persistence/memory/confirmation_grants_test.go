package memory

import (
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/authorization"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

func TestConfirmationGrantRepositoryReplacesBindingAndConsumesConditionally(t *testing.T) {
	t.Parallel()
	repository := NewConfirmationGrantRepository()
	first := testMemoryConfirmationGrant("mcg_01K5D09YJ0C0M7RJM4FWQ0K9HA", 1)
	second := testMemoryConfirmationGrant("mcg_01K5D09YJ0C0M7RJM4FWQ0K9HB", 1)
	if err := repository.CreateReplacing(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateReplacing(t.Context(), second); err != nil {
		t.Fatal(err)
	}
	consumedAt := domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 1, 0, 0, time.UTC))
	first.ConsumedAt = &consumedAt
	first.Version = 2
	if err := repository.Consume(t.Context(), first, 1); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("superseded Consume() error = %v", err)
	}
	second.ConsumedAt = &consumedAt
	second.Version = 2
	if err := repository.Consume(t.Context(), second, 1); err != nil {
		t.Fatal(err)
	}
}

func testMemoryConfirmationGrant(rawID string, version uint64) authorization.ConfirmationGrant {
	return authorization.ConfirmationGrant{
		ConfirmationGrantID: domain.ID(rawID), SellerID: domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		CredentialID: domain.ID("key_01K5D09YJ0C0M7RJM4FWQ0K9H8"), Tool: authorization.ToolConfigureStorefront,
		TargetType: authorization.ConfirmationTargetSeller, TargetID: domain.ID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7"),
		ArgumentsSHA256:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpectedResourceVersion: 1, TokenDigest: "digest", BindingHash: "binding",
		IssuedAt:  domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)),
		ExpiresAt: domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 5, 0, 0, time.UTC)), Version: version,
	}
}
