package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/model"
)

// TestInstanceRepositoryBackfillOwner pins the backfill against the fresh
// baseline: ownership is NOT NULL, so ownerless rows cannot exist and
// BackfillOwner claims nothing, leaving owned instances untouched. (The
// backfill itself goes away with the historical-tolerance cleanup.)
func TestInstanceRepositoryBackfillOwner(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	users := NewUserRepository(pool)
	repo := NewInstanceRepository(pool)

	admin := createTestUser(t, users, "admin@example.com", "admin", 0)
	other := createTestUser(t, users, "other@example.com", "user", 0)

	created := []*model.Instance{
		createTestInstance(t, pool, repo, "owned-one", "owned-one"),
		createTestInstance(t, pool, repo, "owned-two", "owned-two"),
	}

	claimed, err := repo.BackfillOwner(ctx, admin.ID)
	if err != nil {
		t.Fatalf("BackfillOwner: %v", err)
	}
	if claimed != 0 {
		t.Errorf("BackfillOwner claimed = %d, want 0 (no ownerless rows exist)", claimed)
	}

	for _, instance := range created {
		got, err := repo.Get(ctx, instance.ID)
		if err != nil {
			t.Fatalf("Get(%s) after backfill: %v", instance.ID, err)
		}
		if got.OwnerUserID == nil || instance.OwnerUserID == nil || *got.OwnerUserID != *instance.OwnerUserID {
			t.Errorf("Get(%s) OwnerUserID = %v, want untouched", instance.ID, instance.OwnerUserID)
		}
	}

	// The backfill is a no-op even on repeat: never touching owners.
	claimed, err = repo.BackfillOwner(ctx, other.ID)
	if err != nil {
		t.Fatalf("BackfillOwner(other): %v", err)
	}
	if claimed != 0 {
		t.Errorf("BackfillOwner(other) claimed = %d, want 0", claimed)
	}
}

func TestInstanceRepositoryBackfillOwnerUnknownOwner(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	createTestInstance(t, pool, repo, "owned", "owned-ref")

	// With no ownerless rows to match, an unknown owner updates nothing and
	// never reaches the FK.
	claimed, err := repo.BackfillOwner(ctx, uuid.New())
	if err != nil {
		t.Fatalf("BackfillOwner(unknown owner) = %v, want a no-op", err)
	}
	if claimed != 0 {
		t.Errorf("BackfillOwner(unknown owner) claimed = %d, want 0", claimed)
	}
}
