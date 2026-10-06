package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/instance"
	"wzap/internal/model"
	"wzap/internal/storage"
)

func TestInstanceRepositoryNameLookupLegacy(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)
	exact := createTestInstance(t, repo, "Loja_SP-1", "")
	lower := createTestInstance(t, repo, "loja_SP-1", "")
	for _, inst := range []*model.Instance{exact, lower} {
		got, err := repo.GetByName(ctx, inst.Name)
		if err != nil || got.ID != inst.ID {
			t.Fatalf("lookup %q = %+v, %v", inst.Name, got, err)
		}
	}
	if _, err := repo.GetByName(ctx, "missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
	legacyID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO instances(id,name) VALUES($1,'unsafe name!'),($2,'Loja_SP-1')`, legacyID, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByName(ctx, exact.Name); !errors.Is(err, storage.ErrInstanceNameAmbiguous) {
		t.Fatalf("ambiguous = %v", err)
	}
	legacy, err := repo.GetByName(ctx, "unsafe name!")
	if err != nil || legacy.ID != legacyID {
		t.Fatalf("legacy = %+v, %v", legacy, err)
	}
	for _, inst := range []*model.Instance{exact, legacy} {
		got, err := repo.UpdateIdentity(ctx, inst.ID, inst.Name, uuid.NewString())
		if err != nil || got.Name != inst.Name || got.ID != inst.ID {
			t.Fatalf("unchanged legacy = %+v, %v", got, err)
		}
	}
	renamed := "renamed"
	if _, err := repo.UpdateIdentity(ctx, exact.ID, renamed, exact.ExternalRef); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByName(ctx, renamed)
	if err != nil || got.ID != exact.ID {
		t.Fatalf("rename = %+v, %v", got, err)
	}
	if _, err := repo.Get(ctx, exact.ID); err != nil {
		t.Fatal(err)
	}
}

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
	second := createTestInstance(t, repo, "other", "")
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
			for i := 0; i < 8; i++ {
				target := fmt.Sprintf("target-%d", i)
				ops := []func() error{}
				for j := 0; j < 2; j++ {
					inst := model.Instance{ID: uuid.New(), Name: target}
					rename := kind == "rename-rename" || kind == "create-rename" && j == 1
					if rename {
						inst = *createTestInstance(t, repo, fmt.Sprintf("old-%d-%d", i, j), "")
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
	stale := createTestInstance(t, repo, "original", "")
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
	if _, err := tx.Exec(ctx, `INSERT INTO instances(id,name) VALUES($1,'original')`, uuid.New()); err != nil {
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

func TestInstanceRepositoryNameStaleLegacyRestoreRejected(t *testing.T) {
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)
	svc := instance.NewService(repo, nil, nil, nil, nil, zerolog.Nop())
	for i, legacy := range []string{"legacy name!", "stats", "550e8400e29b41d4a716446655440000"} {
		for _, explicit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/explicit-%t", legacy, explicit), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				id := uuid.New()
				t.Cleanup(func() {
					if _, err := pool.Exec(context.Background(), `DELETE FROM instances WHERE id=$1`, id); err != nil {
						t.Errorf("cleanup legacy row: %v", err)
					}
				})
				if _, err := pool.Exec(ctx, `INSERT INTO instances(id,name) VALUES($1,$2)`, id, legacy); err != nil {
					t.Fatal(err)
				}
				// Truly unchanged legacy values still allow unrelated updates.
				priorRef := uuid.NewString()
				if _, err := svc.Update(ctx, id, instance.UpdateInput{ExternalRef: &priorRef, Name: &legacy}); err != nil {
					t.Fatalf("unchanged legacy: %v", err)
				}
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = tx.Rollback(context.Background()) }()
				var blocker int
				if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blocker); err != nil {
					t.Fatal(err)
				}
				validName := fmt.Sprintf("valid-current-%d-%t", i, explicit)
				if _, err := tx.Exec(ctx, `UPDATE instances SET name=$2 WHERE id=$1`, id, validName); err != nil {
					t.Fatal(err)
				}
				nextRef := uuid.NewString()
				input := instance.UpdateInput{ExternalRef: &nextRef}
				if explicit {
					input.Name = &legacy
				}
				result := make(chan error, 1)
				go func() { _, err := svc.Update(ctx, id, input); result <- err }()
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
						t.Fatal("service update never waited for row lock")
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				if err := <-result; !errors.Is(err, instance.ErrInvalidInstanceName) {
					t.Fatalf("stale legacy restore = %v, want invalid name", err)
				}
				current, err := repo.Get(ctx, id)
				if err != nil || current.Name != validName || current.ExternalRef != priorRef {
					t.Fatalf("failed stale write mutated row: %+v, %v", current, err)
				}
			})
		}
	}
}
