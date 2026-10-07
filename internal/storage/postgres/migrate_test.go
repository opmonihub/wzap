package postgres

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"wzap/internal/storage/postgres/postgrestest"
)

// baselineTables is the complete catalog the single 00001_init.sql baseline
// must install on an empty database. goose_db_version (goose's own) and the
// whatsmeow tables (created at runtime) are out of scope.
var baselineTables = []string{
	"users",
	"instances",
	"instance_connections",
	"instance_webhooks",
	"instance_chat_settings",
	"message_queue",
	"media",
	"jid_cache",
	"chatwoot_configs",
	"chatwoot_messages",
	"group_metadata",
	"channel_metadata",
	"idempotency_keys",
	"event_outbox",
	"webhook_dead_letters",
}

// obsoleteTables must NOT exist after the baseline: they belonged to the
// historical chain (pre-remodel shapes and the remodel audit trail).
var obsoleteTables = []string{
	"remodel_report",
	"contacts",
	"newsletter_metadata",
}

func TestMigrate(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	for _, table := range baselineTables {
		if !tableExists(t, ctx, pool, table) {
			t.Errorf("table %s does not exist after Migrate", table)
		}
	}
	for _, table := range obsoleteTables {
		if tableExists(t, ctx, pool, table) {
			t.Errorf("table %s still exists after Migrate, want it removed from the chain", table)
		}
	}

	got := userTables(t, ctx, pool)
	want := append([]string(nil), baselineTables...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("user tables after Migrate = %v, want exactly %v", got, want)
	}

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("second Migrate (idempotence): %v", err)
	}
	got = userTables(t, ctx, pool)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("user tables after re-run = %v, want unchanged %v", got, want)
	}
}

func TestMigrateOwnershipAndNames(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// users keeps the approved columns, defaults and role membership.
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

	// instances.owner_user_id is required and protected by a restrictive FK.
	instances := tableColumns(t, ctx, pool, "instances")
	if got := instances["owner_user_id"]; got.udtName != "uuid" || got.isNullable != "NO" {
		t.Errorf("instances.owner_user_id = type %s nullable %s, want uuid NOT NULL", got.udtName, got.isNullable)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO instances (id, name) VALUES ($1, 'ownerless')`, uuid.New()); err == nil {
		t.Error("instance without owner_user_id was accepted, want NOT NULL rejection")
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO instances (id, name, owner_user_id) VALUES ($1, 'bad-owner', $2)`,
		uuid.New(), uuid.New()); err == nil {
		t.Error("unknown owner_user_id was accepted, want FK rejection")
	}

	instanceID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO instances (id, name, owner_user_id) VALUES ($1, 'loja', $2)`,
		instanceID, adminID); err != nil {
		t.Fatalf("insert owned instance: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`DELETE FROM users WHERE id = $1`, adminID); err == nil {
		t.Error("deleting a user that owns instances was accepted, want restrictive FK rejection")
	}

	// instances.name is globally unique, case-sensitive: distinct cases coexist,
	// an exact duplicate is rejected.
	lowerID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO instances (id, name, owner_user_id) VALUES ($1, 'LOJA', $2)`,
		lowerID, adminID); err != nil {
		t.Fatalf("insert case-variant name: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO instances (id, name, owner_user_id) VALUES ($1, 'loja', $2)`,
		uuid.New(), adminID); err == nil {
		t.Error("duplicate exact instance name was accepted, want UNIQUE rejection")
	}

	// Deleting the owner instance cascades to its satellites.
	if _, err := pool.Exec(ctx,
		`INSERT INTO instance_connections (instance_id) VALUES ($1)`, instanceID); err != nil {
		t.Fatalf("insert connection satellite: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM instances WHERE id = $1`, instanceID); err != nil {
		t.Fatalf("delete owned instance: %v", err)
	}
	if tableExists(t, ctx, pool, "instance_connections") {
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM instance_connections`).Scan(&count); err != nil {
			t.Fatalf("count connections after cascade: %v", err)
		}
		if count != 0 { // the deleted instance's satellite must be gone
			t.Errorf("connection rows after cascade = %d, want 0", count)
		}
	}
}

func TestMigrateSatelliteDefaults(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	adminID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, $3, 'admin')`,
		adminID, "defaults@example.com", "hash"); err != nil {
		t.Fatalf("insert admin user: %v", err)
	}
	instanceID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO instances (id, name, owner_user_id) VALUES ($1, 'defaults', $2)`,
		instanceID, adminID); err != nil {
		t.Fatalf("insert owned instance: %v", err)
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

	var enabled bool
	var events []string
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

	var status string
	if err := pool.QueryRow(ctx, `
		INSERT INTO instance_connections (instance_id) VALUES ($1)
		RETURNING status`, instanceID).Scan(&status); err != nil {
		t.Fatalf("insert instance connection: %v", err)
	}
	if status != "disconnected" {
		t.Errorf("connection status default = %q, want disconnected", status)
	}

	var seconds *int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO instance_chat_settings (instance_id) VALUES ($1)
		RETURNING default_disappearing_seconds`, instanceID).Scan(&seconds); err != nil {
		t.Fatalf("insert instance chat settings: %v", err)
	}
	if seconds != nil {
		t.Errorf("default_disappearing_seconds default = %d, want NULL (never configured)", *seconds)
	}
}

// userTables returns the sorted names of every user table in the current
// schema (system and goose bookkeeping tables excluded).
func userTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []string {
	t.Helper()

	rows, err := pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'
		  AND table_name <> 'goose_db_version'`)
	if err != nil {
		t.Fatalf("list user tables: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table names: %v", err)
	}
	sort.Strings(names)
	return names
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

func mustDef(t *testing.T, def *string, col string) string {
	t.Helper()
	if def == nil {
		t.Fatalf("%s has no column default", col)
	}
	return *def
}
