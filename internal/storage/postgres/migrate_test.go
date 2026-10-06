package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"wzap/internal/storage/migrations"
	"wzap/internal/storage/postgres/postgrestest"
)

func TestMigrate(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	for _, table := range []string{
		"instances",
		"instance_connections",
		"instance_webhooks",
		"message_queue",
		"idempotency_keys",
		"jid_cache",
		"media",
		"event_outbox",
	} {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = $1
			)`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s does not exist after Migrate", table)
		}
	}

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("second Migrate (idempotence): %v", err)
	}
}

type columnInfo struct {
	dataType   string
	udtName    string
	isNullable string
	columnDef  *string
}

func tableColumns(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) map[string]columnInfo {
	t.Helper()

	rows, err := pool.Query(ctx, `
		SELECT column_name, data_type, udt_name, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = $1`, table)
	if err != nil {
		t.Fatalf("query columns of %s: %v", table, err)
	}
	defer rows.Close()

	cols := map[string]columnInfo{}
	for rows.Next() {
		var name string
		var c columnInfo
		if err := rows.Scan(&name, &c.dataType, &c.udtName, &c.isNullable, &c.columnDef); err != nil {
			t.Fatalf("scan column of %s: %v", table, err)
		}
		cols[name] = c
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate columns of %s: %v", table, err)
	}
	return cols
}

func tableExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) bool {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_name = $1
		)`, table).Scan(&exists); err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	return exists
}

// TestMigrateProductFresh applies every migration on a clean database and
// checks the product schema: the users table and the new instances columns.
func TestMigrateProductFresh(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if !tableExists(t, ctx, pool, "users") {
		t.Fatal("table users does not exist after Migrate")
	}

	users := tableColumns(t, ctx, pool, "users")
	for _, col := range []string{"id", "email", "password_hash", "role", "instance_limit", "created_at", "updated_at"} {
		if _, ok := users[col]; !ok {
			t.Errorf("users.%s column is missing", col)
		}
	}
	if got := users["instance_limit"]; got.dataType != "integer" || got.isNullable != "NO" {
		t.Errorf("users.instance_limit = type %s nullable %s, want integer NOT NULL", got.dataType, got.isNullable)
	}
	if def := users["instance_limit"].columnDef; def == nil || *def != "0" {
		t.Errorf("users.instance_limit default = %v, want 0", def)
	}
	// owner stays logically required later; the migration keeps it out of users.

	var checkDef string
	if err := pool.QueryRow(ctx, `
		SELECT pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid = to_regclass(current_schema() || '.users')
		  AND contype = 'c'`).Scan(&checkDef); err != nil {
		t.Fatalf("read users CHECK constraint: %v", err)
	}
	if !strings.Contains(checkDef, "'admin'") || !strings.Contains(checkDef, "'user'") {
		t.Errorf("users role CHECK constraint = %q, want admin/user membership", checkDef)
	}

	var indexDef string
	if err := pool.QueryRow(ctx, `
		SELECT indexdef FROM pg_indexes
		WHERE schemaname = current_schema() AND tablename = 'users'
		  AND indexdef ILIKE '%lower(email)%'`).Scan(&indexDef); err != nil {
		t.Fatalf("unique index on lower(email) is missing: %v", err)
	}
	if !strings.Contains(strings.ToUpper(indexDef), "UNIQUE") {
		t.Errorf("index on lower(email) = %q, want UNIQUE", indexDef)
	}

	instances := tableColumns(t, ctx, pool, "instances")
	for _, col := range []string{"owner_user_id", "api_key_hash"} {
		if _, ok := instances[col]; !ok {
			t.Errorf("instances.%s column is missing", col)
		}
	}
	// The connection and webhook fields moved to the satellite tables.
	for _, dropped := range []string{"status", "whatsapp_jid", "device_jid", "last_connected_at", "last_error", "webhook_url", "webhook_enabled", "webhook_events"} {
		if _, ok := instances[dropped]; ok {
			t.Errorf("instances.%s still present, want it moved to its satellite", dropped)
		}
	}
	// R4: owner stays NULLABLE in this migration; NOT NULL comes later.
	if got := instances["owner_user_id"]; got.udtName != "uuid" || got.isNullable != "YES" {
		t.Errorf("instances.owner_user_id = type %s nullable %s, want uuid NULL", got.udtName, got.isNullable)
	}

	webhooks := tableColumns(t, ctx, pool, "instance_webhooks")
	if got := webhooks["is_enabled"]; got.dataType != "boolean" || got.isNullable != "NO" {
		t.Errorf("instance_webhooks.is_enabled = type %s nullable %s, want boolean NOT NULL", got.dataType, got.isNullable)
	}
	if def := mustDef(t, webhooks["is_enabled"].columnDef, "instance_webhooks.is_enabled"); def != "false" {
		t.Errorf("instance_webhooks.is_enabled default = %q, want false", def)
	}
	if got := webhooks["events"]; got.udtName != "_text" || got.isNullable != "NO" {
		t.Errorf("instance_webhooks.events = type %s nullable %s, want text[] NOT NULL", got.udtName, got.isNullable)
	}
	if def := mustDef(t, webhooks["events"].columnDef, "instance_webhooks.events"); !strings.Contains(def, "message") ||
		!strings.Contains(def, "receipt") || !strings.Contains(def, "connection") || !strings.Contains(def, "message.status") {
		t.Errorf("instance_webhooks.events default = %q, want message,receipt,connection,message.status", def)
	}

	connections := tableColumns(t, ctx, pool, "instance_connections")
	if got := connections["status"]; got.dataType != "text" || got.isNullable != "NO" {
		t.Errorf("instance_connections.status = type %s nullable %s, want text NOT NULL", got.dataType, got.isNullable)
	}
	if def := mustDef(t, connections["status"].columnDef, "instance_connections.status"); def != "'disconnected'::text" {
		t.Errorf("instance_connections.status default = %q, want 'disconnected'", def)
	}

	// Behavior: case-insensitive email uniqueness, role check, FK, defaults.
	adminID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, $3, 'admin')`,
		adminID, "Admin@Example.com", "hash"); err != nil {
		t.Fatalf("insert admin user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, $3, 'user')`,
		uuid.New(), "admin@example.com", "hash"); err == nil {
		t.Error("duplicate email with different case was accepted, want unique lower(email)")
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, $3, 'superadmin')`,
		uuid.New(), "other@example.com", "hash"); err == nil {
		t.Error("role 'superadmin' was accepted, want CHECK rejection")
	}

	instanceID := uuid.New()
	var owner *string
	var enabled bool
	var events []string
	if err := pool.QueryRow(ctx, `
		INSERT INTO instances (id, name, owner_user_id) VALUES ($1, 'loja', $2)
		RETURNING owner_user_id::text`,
		instanceID, adminID).Scan(&owner); err != nil {
		t.Fatalf("insert owned instance: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO instance_webhooks (instance_id) VALUES ($1)
		RETURNING is_enabled, events`, instanceID).Scan(&enabled, &events); err != nil {
		t.Fatalf("insert instance webhook: %v", err)
	}
	if enabled {
		t.Error("webhook is_enabled default = true, want false")
	}
	if len(events) != 4 || events[0] != "message" || events[1] != "receipt" || events[2] != "connection" || events[3] != "message.status" {
		t.Errorf("webhook events default = %v, want [message receipt connection message.status]", events)
	}
	if owner == nil || *owner != adminID.String() {
		t.Errorf("owner_user_id = %v, want %s", owner, adminID)
	}

	// Legacy-style row without an owner is still accepted here (R4).
	if _, err := pool.Exec(ctx,
		`INSERT INTO instances (id, name) VALUES ($1, 'legacy-style')`, uuid.New()); err != nil {
		t.Fatalf("insert ownerless instance: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO instances (id, name, owner_user_id) VALUES ($1, 'bad-owner', $2)`,
		uuid.New(), uuid.New()); err == nil {
		t.Error("unknown owner_user_id was accepted, want FK rejection")
	}
}

// TestMigrateProductLegacyInstances migrates only 00001, inserts legacy
// instance rows, then runs the full Migrate and checks the rows survive with
// NULL owner and product defaults.
func TestMigrateProductLegacyInstances(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.UpToContext(ctx, sqlDB, ".", 1); err != nil {
		t.Fatalf("migrate up to 00001: %v", err)
	}
	if tableExists(t, ctx, pool, "users") {
		t.Fatal("table users exists after 00001, want it created only by 00002")
	}

	legacyID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO instances (id, name, external_ref) VALUES ($1, 'legacy', 'ext-1')`,
		legacyID); err != nil {
		t.Fatalf("insert legacy instance: %v", err)
	}

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !tableExists(t, ctx, pool, "users") {
		t.Fatal("table users does not exist after Migrate")
	}

	var name string
	var extRef *string
	var owner *string
	var apiKeyHash *string
	if err := pool.QueryRow(ctx, `
		SELECT name, external_ref, owner_user_id::text, api_key_hash
		FROM instances WHERE id = $1`, legacyID).
		Scan(&name, &extRef, &owner, &apiKeyHash); err != nil {
		t.Fatalf("read legacy instance: %v", err)
	}
	if name != "legacy" || extRef == nil || *extRef != "ext-1" {
		t.Errorf("legacy row changed: name=%q external_ref=%v", name, extRef)
	}
	if owner != nil {
		t.Errorf("legacy owner_user_id = %v, want NULL", *owner)
	}
	if apiKeyHash != nil {
		t.Errorf("legacy api_key_hash=%v, want NULL", apiKeyHash)
	}

	// The migration splits connection/webhook into satellites with defaults.
	var status string
	var enabled bool
	var webhookURL *string
	var events []string
	if err := pool.QueryRow(ctx, `
		SELECT status FROM instance_connections WHERE instance_id = $1`, legacyID).Scan(&status); err != nil {
		t.Fatalf("read legacy connection satellite: %v", err)
	}
	if status != "disconnected" {
		t.Errorf("legacy connection status = %q, want disconnected", status)
	}
	if err := pool.QueryRow(ctx, `
		SELECT url, is_enabled, events FROM instance_webhooks WHERE instance_id = $1`, legacyID).
		Scan(&webhookURL, &enabled, &events); err != nil {
		t.Fatalf("read legacy webhook satellite: %v", err)
	}
	if webhookURL != nil {
		t.Errorf("legacy webhook url=%v, want NULL", webhookURL)
	}
	if enabled {
		t.Error("legacy webhook is_enabled = true, want false")
	}
	if len(events) != 4 {
		t.Errorf("legacy webhook events = %v, want 4 default entries", events)
	}
}

func mustDef(t *testing.T, def *string, col string) string {
	t.Helper()
	if def == nil {
		t.Fatalf("%s has no column default", col)
	}
	return *def
}
