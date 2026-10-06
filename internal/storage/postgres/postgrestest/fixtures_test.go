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
// Postgres schema migrated to the pre-remodel baseline (00007): the fixtures
// deliberately use the legacy column/table names so upgrade tests can apply
// the remodel migration on top. It runs only when WZAP_TEST_DATABASE_URL
// points at a *_test database.
func TestSeedRemodelFixtures(t *testing.T) {
	pool := NewPool(t)
	migrateFixtureSchema(t, pool, 7)
	SeedRemodelFixtures(t, pool)
}

// migrateFixtureSchema applies the embedded goose migrations up to version
// inside the isolated schema, mirroring postgres.Migrate without importing
// the postgres package (postgrestest must stay importable by postgres's own
// tests).
func migrateFixtureSchema(t *testing.T, pool *pgxpool.Pool, version int64) {
	t.Helper()

	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	goose.SetBaseFS(migrations.FS)

	if err := goose.UpToContext(context.Background(), db, ".", version); err != nil {
		t.Fatalf("apply migrations up to %d: %v", version, err)
	}
}
