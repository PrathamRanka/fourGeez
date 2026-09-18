package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/sellerworkspace"
)

func TestSellerWorkspaceRepositoryUsesOptimisticWrites(t *testing.T) {
	t.Parallel()
	repository := NewSellerWorkspaceRepository()
	state := testSellerWorkspaceState(t)
	if err := repository.Put(context.Background(), state, 0); err != nil {
		t.Fatal(err)
	}
	state.Version++
	state.UpdatedAt = state.UpdatedAt.Add(time.Minute)
	if err := repository.Put(context.Background(), state, 1); err != nil {
		t.Fatal(err)
	}
	if err := repository.Put(context.Background(), state, 1); !errors.Is(err, persistence.ErrConditionFailed) {
		t.Fatalf("stale Put() error = %v, want condition failure", err)
	}
	stored, err := repository.Get(context.Background(), state.SellerID)
	if err != nil || stored.Version != 2 {
		t.Fatalf("Get() = %#v, %v", stored, err)
	}
}

func testSellerWorkspaceState(t *testing.T) sellerworkspace.WorkspaceState {
	t.Helper()
	sellerID, err := domain.ParseID("sel_01K5D09YJ0C0M7RJM4FWQ0K9H7", domain.SellerIDPrefix)
	if err != nil {
		t.Fatal(err)
	}
	now := domain.NewTimestamp(time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC))
	return sellerworkspace.WorkspaceState{SellerID: sellerID, OwnerSubjectHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CreatedAt: now, UpdatedAt: now, Version: 1, Settings: sellerworkspace.SellerSettings{Version: 1, UpdatedAt: now}}
}
