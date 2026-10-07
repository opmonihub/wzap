package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/storage"
)

func TestInstanceRepositoryNameClaims(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)
	ownerA, err := NewUserRepository(pool).Create(ctx, model.User{ID: uuid.New(), Email: "a@example.test", PasswordHash: "hash", Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	ownerB, err := NewUserRepository(pool).Create(ctx, model.User{ID: uuid.New(), Email: "b@example.test", PasswordHash: "hash", Role: "user"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.Create(ctx, model.Instance{ID: uuid.New(), Name: "global", OwnerUserID: &ownerA.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, model.Instance{ID: uuid.New(), Name: first.Name, OwnerUserID: &ownerB.ID}); !errors.Is(err, storage.ErrInstanceNameTaken) {
		t.Fatalf("cross-owner duplicate = %v", err)
	}
	second := createTestInstance(t, pool, repo, "other", "")
	if _, err := repo.UpdateIdentity(ctx, second.ID, first.Name, second.ExternalRef); !errors.Is(err, storage.ErrInstanceNameTaken) {
		t.Fatalf("rename occupied = %v", err)
	}
	if _, err := repo.UpdateIdentity(ctx, uuid.New(), first.Name, ""); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("unknown row = %v", err)
	}
}

func TestInstanceRepositoryNameConcurrentClaims(t *testing.T) {
	for _, kind := range []string{"create-create", "rename-rename", "create-rename"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			pool := newTestPool(t)
			repo := NewInstanceRepository(pool)
			owner := createTestOwner(t, pool)
			for i := 0; i < 8; i++ {
				target := fmt.Sprintf("target-%d", i)
				ops := []func() error{}
				for j := 0; j < 2; j++ {
					inst := model.Instance{ID: uuid.New(), Name: target, OwnerUserID: &owner.ID}
					rename := kind == "rename-rename" || kind == "create-rename" && j == 1
					if rename {
						inst = *createTestInstance(t, pool, repo, fmt.Sprintf("old-%d-%d", i, j), "")
						inst.Name = target
					}
					ops = append(ops, func() error {
						if rename {
							_, err := repo.UpdateIdentity(ctx, inst.ID, inst.Name, inst.ExternalRef)
							return err
						}
						_, err := repo.Create(ctx, inst)
						return err
					})
				}
				start := make(chan struct{})
				results := make(chan error, 2)
				for _, op := range ops {
					go func() { <-start; results <- op() }()
				}
				close(start)
				successes, conflicts := 0, 0
				for j := 0; j < 2; j++ {
					err := <-results
					if err == nil {
						successes++
					} else if errors.Is(err, storage.ErrInstanceNameTaken) {
						conflicts++
					} else {
						t.Fatalf("claim: %v", err)
					}
				}
				if successes != 1 || conflicts != 1 {
					t.Fatalf("%s: successes=%d conflicts=%d, want 1 each", kind, successes, conflicts)
				}
				got, err := repo.GetByName(ctx, target)
				if err != nil || got == nil {
					t.Fatalf("winner lookup = %+v, %v", got, err)
				}
			}
		})
	}
}

func TestInstanceRepositoryNameSameRowUsesCurrentDatabaseName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	stale := createTestInstance(t, pool, repo, "original", "")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var blocker int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blocker); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE instances SET name='new-current' WHERE id=$1`, stale.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO instances(id,name,owner_user_id) VALUES($1,'original',$2)`, uuid.New(), owner.ID); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, err := repo.UpdateIdentity(ctx, stale.ID, stale.Name, stale.ExternalRef); result <- err }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var blocked bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, blocker).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("update never waited for row lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, storage.ErrInstanceNameTaken) {
		t.Fatalf("stale unchanged name = %v, want occupied rename", err)
	}
	current, err := repo.Get(ctx, stale.ID)
	if err != nil || current.Name != "new-current" {
		t.Fatalf("current = %+v, %v", current, err)
	}
}
