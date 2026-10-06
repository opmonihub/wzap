package postgrestest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"wzap/internal/storage/migrations"
)

// TestSeedRemodelFixtures exercises SeedRemodelFixtures against a real
// Postgres schema migrated to the current baseline. It runs only when
// WZAP_TEST_DATABASE_URL points at a *_test database.
func TestSeedRemodelFixtures(t *testing.T) {
	pool := NewPool(t)
	migrateFixtureSchema(t, pool)
	SeedRemodelFixtures(t, pool)
}

// migrateFixtureSchema applies the embedded goose migrations inside the
// isolated schema, mirroring postgres.Migrate without importing the postgres
// package (postgrestest must stay importable by postgres's own tests).
func migrateFixtureSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	goose.SetBaseFS(migrations.FS)

	if err := goose.UpContext(context.Background(), db, "."); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
}
