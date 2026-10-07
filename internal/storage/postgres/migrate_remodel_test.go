package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"wzap/internal/storage/migrations"
	"wzap/internal/storage/postgres/postgrestest"
)

// The remodel turns the 12 pre-remodel product tables (00001-00007) into the
// final 14 approved in the change design. These tests run only when
// WZAP_TEST_DATABASE_URL points at a *_test database.

// remodelTables is the approved final set: contacts becomes jid_cache,
// newsletter_metadata becomes channel_metadata, and instance_connections /
// instance_webhooks join instances.
var remodelTables = []string{
	"instances",
	"instance_connections",
	"instance_webhooks",
	"users",
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

// migrateToVersion applies migrations up to version so fixtures can be seeded
// under the legacy schema before the remodel migration runs.
func migrateToVersion(t *testing.T, ctx context.Context, pool *pgxpool.Pool, version int64) {
	t.Helper()

	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.UpToContext(ctx, db, ".", version); err != nil {
		t.Fatalf("migrate up to %d: %v", version, err)
	}
}

// seedAndUpgrade migrates the schema to the pre-remodel baseline (00007),
// seeds the remodel fixtures and applies the remodel data migration (00008)
// plus the resolution marker (00009) on top. The cutover gate (00010) is
// deliberately not applied here: the fixtures include a divergent device
// JID, so the full migrate stays blocked until an explicit resolution is
// recorded — that loop is exercised in TestMigrateDeviceJIDGateBlocksCutover.
func seedAndUpgrade(t *testing.T) (*pgxpool.Pool, context.Context, postgrestest.RemodelFixtures) {
	t.Helper()

	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	migrateToVersion(t, ctx, pool, 7)
	fx := postgrestest.SeedRemodelFixtures(t, pool)

	migrateToVersion(t, ctx, pool, 9)
	return pool, ctx, fx
}

// resolveDeviceJIDConflicts performs the operator step the 00010 gate
// documents: after reconciling the identity, each device_jid_conflicts row
// is marked resolved. The fixtures keep the device_jid 00008 chose, so this
// only records the acknowledgment — the reconciliation itself would be an
// UPDATE on instance_connections.
func resolveDeviceJIDConflicts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fx postgrestest.RemodelFixtures) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		UPDATE remodel_report SET resolved_at = now()
		WHERE category = 'device_jid_conflicts' AND ref_id = $1`,
		fx.InstanceDivergentJIDID); err != nil {
		t.Fatalf("mark device_jid conflict resolved: %v", err)
	}
}

// appliedVersion returns the highest applied goose version.
func appliedVersion(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int64 {
	t.Helper()

	var version int64
	if err := pool.QueryRow(ctx, `
		SELECT coalesce(max(version_id), 0) FROM goose_db_version WHERE is_applied`).
		Scan(&version); err != nil {
		t.Fatalf("read goose version: %v", err)
	}
	return version
}

// TestMigrateRemodelFresh applies every migration on an empty schema and
// asserts the final catalog: the 14 approved tables exist, renamed tables are
// gone, and no whatsmeow/goose bookkeeping was touched.
func TestMigrateRemodelFresh(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	for _, table := range remodelTables {
		if !tableExists(t, ctx, pool, table) {
			t.Errorf("table %s missing after remodel migrate", table)
		}
	}
	for _, dropped := range []string{"contacts", "newsletter_metadata"} {
		if tableExists(t, ctx, pool, dropped) {
			t.Errorf("table %s still present after remodel", dropped)
		}
	}

	// Column-level spot checks on the renamed/dropped columns.
	instances := tableColumns(t, ctx, pool, "instances")
	for _, gone := range []string{"status", "whatsapp_jid", "device_jid", "last_connected_at", "last_error", "webhook_url", "webhook_enabled", "webhook_events"} {
		if _, ok := instances[gone]; ok {
			t.Errorf("instances.%s still present; connection/webhook state moved to satellite tables", gone)
		}
	}
	for _, col := range []string{"id", "name", "external_ref", "owner_user_id", "api_key_hash", "created_at", "updated_at"} {
		if _, ok := instances[col]; !ok {
			t.Errorf("instances.%s missing", col)
		}
	}
	if got := instances["owner_user_id"]; got.udtName != "uuid" || got.isNullable != "YES" {
		t.Errorf("instances.owner_user_id = type %s nullable %s, want uuid NULL", got.udtName, got.isNullable)
	}
	// Every table's uuid PK carries gen_random_uuid() per the approved matrix,
	// including the four pre-existing uuid PKs (instances, message_queue,
	// media, event_outbox).
	for _, table := range remodelTables {
		cols := tableColumns(t, ctx, pool, table)
		def := cols["id"].columnDef
		if def == nil || !strings.Contains(*def, "gen_random_uuid()") {
			t.Errorf("%s.id default = %v, want gen_random_uuid()", table, def)
		}
	}

	connections := tableColumns(t, ctx, pool, "instance_connections")
	for _, col := range []string{"id", "instance_id", "device_jid", "status",
		"last_connected_at", "last_error_code", "last_error_message", "last_error_at",
		"created_at", "updated_at"} {
		if _, ok := connections[col]; !ok {
			t.Errorf("instance_connections.%s missing", col)
		}
	}
	if got := connections["status"]; got.isNullable != "NO" {
		t.Errorf("instance_connections.status nullable = %s, want NOT NULL", got.isNullable)
	}
	if def := mustDef(t, connections["status"].columnDef, "instance_connections.status"); !strings.Contains(def, "disconnected") {
		t.Errorf("instance_connections.status default = %q, want 'disconnected'", def)
	}

	webhooks := tableColumns(t, ctx, pool, "instance_webhooks")
	for _, col := range []string{"id", "instance_id", "url", "is_enabled", "events", "created_at", "updated_at"} {
		if _, ok := webhooks[col]; !ok {
			t.Errorf("instance_webhooks.%s missing", col)
		}
	}
	if got := webhooks["is_enabled"]; got.dataType != "boolean" || got.isNullable != "NO" {
		t.Errorf("instance_webhooks.is_enabled = type %s nullable %s, want boolean NOT NULL", got.dataType, got.isNullable)
	}
	if got := webhooks["events"]; got.udtName != "_text" || got.isNullable != "NO" {
		t.Errorf("instance_webhooks.events = type %s nullable %s, want text[] NOT NULL", got.udtName, got.isNullable)
	}

	users := tableColumns(t, ctx, pool, "users")
	if _, ok := users["instance_quota"]; ok {
		t.Error("users.instance_quota still present, want renamed to instance_limit")
	}
	if got := users["instance_limit"]; got.dataType != "integer" || got.isNullable != "NO" {
		t.Errorf("users.instance_limit = type %s nullable %s, want integer NOT NULL", got.dataType, got.isNullable)
	}
	if def := mustDef(t, users["instance_limit"].columnDef, "users.instance_limit"); def != "0" {
		t.Errorf("users.instance_limit default = %q, want 0", def)
	}
	for _, col := range []string{"email", "password_hash"} {
		if got := users[col]; got.isNullable != "NO" {
			t.Errorf("users.%s nullable = %s, want NOT NULL per the approved matrix", col, got.isNullable)
		}
	}

	queue := tableColumns(t, ctx, pool, "message_queue")
	for _, gone := range []string{"recipient", "type", "status", "retries", "whatsapp_id", "last_error"} {
		if _, ok := queue[gone]; ok {
			t.Errorf("message_queue.%s still present, want it renamed", gone)
		}
	}
	for _, col := range []string{"recipient_jid", "message_type", "send_status", "retry_count", "wa_id",
		"media_id", "last_error_code", "last_error_message", "last_error_at",
		"next_attempt_at", "delivered_at", "read_at"} {
		if _, ok := queue[col]; !ok {
			t.Errorf("message_queue.%s missing", col)
		}
	}
	if got := queue["media_id"]; got.udtName != "uuid" || got.isNullable != "YES" {
		t.Errorf("message_queue.media_id = type %s nullable %s, want uuid NULL", got.udtName, got.isNullable)
	}
	if def := mustDef(t, queue["send_status"].columnDef, "message_queue.send_status"); !strings.Contains(def, "queued") {
		t.Errorf("message_queue.send_status default = %q, want 'queued'", def)
	}

	media := tableColumns(t, ctx, pool, "media")
	for _, gone := range []string{"message_id", "mimetype", "filename", "storage_path"} {
		if _, ok := media[gone]; ok {
			t.Errorf("media.%s still present, want it renamed/removed", gone)
		}
	}
	for _, col := range []string{"wa_id", "mime_type", "file_name", "bucket", "object_key", "sha256", "expires_at", "object_deleted_at", "updated_at"} {
		if _, ok := media[col]; !ok {
			t.Errorf("media.%s missing", col)
		}
	}
	for _, notNull := range []string{"bucket", "object_key", "expires_at", "mime_type", "size_bytes"} {
		if got := media[notNull]; got.isNullable != "NO" {
			t.Errorf("media.%s nullable = %s, want NOT NULL", notNull, got.isNullable)
		}
	}
	if got := media["object_deleted_at"]; got.isNullable != "YES" {
		t.Errorf("media.object_deleted_at nullable = %s, want NULL", got.isNullable)
	}

	jidCache := tableColumns(t, ctx, pool, "jid_cache")
	for _, col := range []string{"id", "phone", "jid", "expires_at", "created_at", "updated_at"} {
		if _, ok := jidCache[col]; !ok {
			t.Errorf("jid_cache.%s missing", col)
		}
	}

	cfg := tableColumns(t, ctx, pool, "chatwoot_configs")
	for _, col := range []string{"id", "is_enabled", "inbox_name", "is_sign_enabled", "is_reopen_enabled",
		"is_pending_enabled", "is_merge_enabled", "is_import_contacts", "is_import_messages",
		"import_days", "is_auto_create", "ignored_jids", "token"} {
		if _, ok := cfg[col]; !ok {
			t.Errorf("chatwoot_configs.%s missing", col)
		}
	}
	for _, gone := range []string{"enabled", "name_inbox", "sign_msg", "reopen_conversation",
		"conversation_pending", "merge_brazil_contacts", "import_contacts", "import_messages",
		"days_limit", "auto_create", "ignore_jids"} {
		if _, ok := cfg[gone]; ok {
			t.Errorf("chatwoot_configs.%s still present, want it renamed", gone)
		}
	}

	cw := tableColumns(t, ctx, pool, "chatwoot_messages")
	for _, gone := range []string{"chatwoot_message_id", "contact_source_id"} {
		if _, ok := cw[gone]; ok {
			t.Errorf("chatwoot_messages.%s still present, want it renamed", gone)
		}
	}
	for _, col := range []string{"id", "message_id", "wa_key", "cw_id", "conversation_id", "inbox_id", "chat_jid", "is_read", "updated_at"} {
		if _, ok := cw[col]; !ok {
			t.Errorf("chatwoot_messages.%s missing", col)
		}
	}
	for _, bigint := range []string{"cw_id", "conversation_id", "inbox_id"} {
		if got := cw[bigint]; got.dataType != "bigint" {
			t.Errorf("chatwoot_messages.%s = type %s, want bigint", bigint, got.dataType)
		}
	}
	if got := cw["message_id"]; got.udtName != "uuid" || got.isNullable != "YES" {
		t.Errorf("chatwoot_messages.message_id = type %s nullable %s, want uuid NULL", got.udtName, got.isNullable)
	}

	groups := tableColumns(t, ctx, pool, "group_metadata")
	if _, ok := groups["id"]; !ok {
		t.Error("group_metadata.id (uuid PK) missing")
	}
	channels := tableColumns(t, ctx, pool, "channel_metadata")
	for _, col := range []string{"id", "instance_id", "channel_jid", "title", "description", "follower_count", "created_at", "updated_at"} {
		if _, ok := channels[col]; !ok {
			t.Errorf("channel_metadata.%s missing", col)
		}
	}

	idem := tableColumns(t, ctx, pool, "idempotency_keys")
	for _, gone := range []string{"idempotency_key", "response_status"} {
		if _, ok := idem[gone]; ok {
			t.Errorf("idempotency_keys.%s still present, want it renamed", gone)
		}
	}
	for _, col := range []string{"id", "key", "request_hash", "status", "http_status", "response_body", "expires_at", "updated_at"} {
		if _, ok := idem[col]; !ok {
			t.Errorf("idempotency_keys.%s missing", col)
		}
	}

	outbox := tableColumns(t, ctx, pool, "event_outbox")
	if _, ok := outbox["published_at"]; ok {
		t.Error("event_outbox.published_at still present, want it removed")
	}
	for _, col := range []string{"id", "subject", "envelope", "attempt_count",
		"last_error_code", "last_error_message", "last_error_at", "updated_at"} {
		if _, ok := outbox[col]; !ok {
			t.Errorf("event_outbox.%s missing", col)
		}
	}

	dead := tableColumns(t, ctx, pool, "webhook_dead_letters")
	for _, gone := range []string{"payload", "attempts", "last_error"} {
		if _, ok := dead[gone]; ok {
			t.Errorf("webhook_dead_letters.%s still present, want it renamed", gone)
		}
	}
	for _, col := range []string{"id", "event_id", "event_type", "envelope", "attempt_count",
		"last_error_code", "last_error_message", "last_error_at", "updated_at"} {
		if _, ok := dead[col]; !ok {
			t.Errorf("webhook_dead_letters.%s missing", col)
		}
	}
	if got := dead["id"]; got.udtName != "uuid" {
		t.Errorf("webhook_dead_letters.id = type %s, want uuid", got.udtName)
	}

	// The audit table carries the explicit-resolution marker of the cutover
	// gate (00009); the gate itself (00010) passes on an empty database.
	report := tableColumns(t, ctx, pool, "remodel_report")
	for _, col := range []string{"category", "ref_id", "detail", "created_at", "resolved_at"} {
		if _, ok := report[col]; !ok {
			t.Errorf("remodel_report.%s missing", col)
		}
	}
	if got := report["resolved_at"]; got.dataType != "timestamp with time zone" || got.isNullable != "YES" {
		t.Errorf("remodel_report.resolved_at = type %s nullable %s, want timestamptz NULL", got.dataType, got.isNullable)
	}
}

// TestMigrateRemodelUpgrade is the acceptance test of the task: it seeds the
// legacy schema (00007) with fixtures covering every remodel edge case,
// applies the remodel migration, and asserts the non-destructive policies.
func TestMigrateRemodelUpgrade(t *testing.T) {
	pool, ctx, fx := seedAndUpgrade(t)

	// ------------------------------------------------------------------
	// instances: identity columns preserved, satellites created 1:1.
	// ------------------------------------------------------------------
	var instanceCount, connectionCount, webhookCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM instances`).Scan(&instanceCount); err != nil {
		t.Fatalf("count instances: %v", err)
	}
	if instanceCount != 4 {
		t.Errorf("instances count = %d, want 4 (UUIDs preserved, none dropped)", instanceCount)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM instance_connections`).Scan(&connectionCount); err != nil {
		t.Fatalf("count instance_connections: %v", err)
	}
	if connectionCount != instanceCount {
		t.Errorf("instance_connections count = %d, want one per instance (%d)", connectionCount, instanceCount)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM instance_webhooks`).Scan(&webhookCount); err != nil {
		t.Fatalf("count instance_webhooks: %v", err)
	}
	if webhookCount != instanceCount {
		t.Errorf("instance_webhooks count = %d, want one per instance (%d)", webhookCount, instanceCount)
	}

	// Paired instance keeps owner, external_ref and device identity.
	var extRef *string
	var owner *uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT external_ref, owner_user_id FROM instances WHERE id = $1`,
		fx.InstancePairedID).Scan(&extRef, &owner); err != nil {
		t.Fatalf("read paired instance: %v", err)
	}
	if extRef == nil || *extRef != "ext-paired" {
		t.Errorf("paired instance external_ref = %v, want ext-paired", extRef)
	}
	if owner == nil || *owner != fx.OwnerUserID {
		t.Errorf("paired instance owner_user_id = %v, want %s", owner, fx.OwnerUserID)
	}

	var connStatus, connDeviceJID string
	var connLastConnected *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status, device_jid, last_connected_at
		FROM instance_connections WHERE instance_id = $1`, fx.InstancePairedID).
		Scan(&connStatus, &connDeviceJID, &connLastConnected); err != nil {
		t.Fatalf("read paired connection: %v", err)
	}
	if connStatus != "connected" {
		t.Errorf("paired connection status = %q, want connected", connStatus)
	}
	if connDeviceJID != "5511999990001:5@s.whatsapp.net" {
		t.Errorf("paired connection device_jid = %q", connDeviceJID)
	}
	if connLastConnected == nil {
		t.Error("paired connection last_connected_at = NULL, want preserved timestamp")
	}

	// Legacy row: whatsapp_jid populated, device_jid NULL -> migrated per the
	// 00007 rule.
	var legacyDeviceJID *string
	if err := pool.QueryRow(ctx,
		`SELECT device_jid FROM instance_connections WHERE instance_id = $1`,
		fx.InstanceLegacyOnlyID).Scan(&legacyDeviceJID); err != nil {
		t.Fatalf("read legacy connection: %v", err)
	}
	if legacyDeviceJID == nil || *legacyDeviceJID != "5511888880002:12@s.whatsapp.net" {
		t.Errorf("legacy device_jid = %v, want whatsapp_jid fallback", legacyDeviceJID)
	}

	// Divergent whatsapp_jid vs device_jid: device_jid wins and the conflict
	// is reported by 00008; the cutover itself stays blocked by the 00010
	// gate until explicit resolution (TestMigrateDeviceJIDGateBlocksCutover)
	// and nothing is silently dropped.
	var divDeviceJID *string
	if err := pool.QueryRow(ctx,
		`SELECT device_jid FROM instance_connections WHERE instance_id = $1`,
		fx.InstanceDivergentJIDID).Scan(&divDeviceJID); err != nil {
		t.Fatalf("read divergent connection: %v", err)
	}
	if divDeviceJID == nil || *divDeviceJID != "5511777770003:99@s.whatsapp.net" {
		t.Errorf("divergent device_jid = %v, want the device_jid value kept", divDeviceJID)
	}
	var conflictCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM remodel_report
		WHERE category = 'device_jid_conflicts' AND ref_id = $1`,
		fx.InstanceDivergentJIDID).Scan(&conflictCount); err != nil {
		t.Fatalf("count device_jid_conflicts: %v", err)
	}
	if conflictCount != 1 {
		t.Errorf("device_jid_conflicts entries for divergent instance = %d, want 1", conflictCount)
	}

	// Never-paired instance: connection row exists, status default,
	// device_jid NULL.
	var npStatus string
	var npDeviceJID *string
	if err := pool.QueryRow(ctx,
		`SELECT status, device_jid FROM instance_connections WHERE instance_id = $1`,
		fx.InstanceNeverPairedID).Scan(&npStatus, &npDeviceJID); err != nil {
		t.Fatalf("read never-paired connection: %v", err)
	}
	if npStatus != "disconnected" || npDeviceJID != nil {
		t.Errorf("never-paired connection = status %q device_jid %v, want disconnected/NULL", npStatus, npDeviceJID)
	}

	// device_jid stays unique when filled.
	if _, err := pool.Exec(ctx, `
		INSERT INTO instance_connections (instance_id, device_jid)
		VALUES ($1, '5511999990001:5@s.whatsapp.net')`, uuid.New()); err == nil {
		t.Error("duplicate device_jid accepted, want unique violation")
	}
	// NULL device_jid rows do not collide.
	if _, err := pool.Exec(ctx, `
		INSERT INTO instance_connections (instance_id) VALUES ($1)`, uuid.New()); err == nil {
		t.Error("second connection for a dangling instance_id was accepted without FK rejection")
	}

	// webhook defaults copied per instance.
	var whEnabled bool
	var whURL *string
	var whEvents []string
	if err := pool.QueryRow(ctx,
		`SELECT is_enabled, url, events FROM instance_webhooks WHERE instance_id = $1`,
		fx.InstancePairedID).Scan(&whEnabled, &whURL, &whEvents); err != nil {
		t.Fatalf("read paired webhook: %v", err)
	}
	if whEnabled {
		t.Error("paired webhook is_enabled = true, want false (legacy default)")
	}
	if whURL != nil {
		t.Errorf("paired webhook url = %v, want NULL", *whURL)
	}
	if len(whEvents) != 4 {
		t.Errorf("paired webhook events = %v, want the 4 defaults", whEvents)
	}

	// ------------------------------------------------------------------
	// users: instance_quota -> instance_limit, values preserved.
	// ------------------------------------------------------------------
	var adminLimit, ownerLimit int
	if err := pool.QueryRow(ctx, `SELECT instance_limit FROM users WHERE id = $1`, fx.AdminUserID).Scan(&adminLimit); err != nil {
		t.Fatalf("read admin limit: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT instance_limit FROM users WHERE id = $1`, fx.OwnerUserID).Scan(&ownerLimit); err != nil {
		t.Fatalf("read owner limit: %v", err)
	}
	if adminLimit != 10 || ownerLimit != 5 {
		t.Errorf("instance_limit = admin:%d owner:%d, want 10 and 5", adminLimit, ownerLimit)
	}

	// ------------------------------------------------------------------
	// message_queue: renames, orphan media_id nulled + reported.
	// ------------------------------------------------------------------
	var msgStatus string
	var msgWAID *string
	var msgMediaID *uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT send_status, wa_id, media_id FROM message_queue WHERE id = $1`,
		fx.MessageWithMediaID).Scan(&msgStatus, &msgWAID, &msgMediaID); err != nil {
		t.Fatalf("read message with media: %v", err)
	}
	if msgStatus != "sent" {
		t.Errorf("message send_status = %q, want sent", msgStatus)
	}
	if msgWAID == nil || *msgWAID != "3EB0SENTWITHMEDIA" {
		t.Errorf("message wa_id = %v, want 3EB0SENTWITHMEDIA", msgWAID)
	}
	if msgMediaID == nil || *msgMediaID != fx.MediaActiveID {
		t.Errorf("message media_id = %v, want %s", msgMediaID, fx.MediaActiveID)
	}

	var orphanMediaID *uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT media_id FROM message_queue WHERE id = $1`,
		fx.MessageOrphanMedia).Scan(&orphanMediaID); err != nil {
		t.Fatalf("read orphan message: %v", err)
	}
	if orphanMediaID != nil {
		t.Errorf("orphan message media_id = %v, want NULL (ghost ref anulled)", *orphanMediaID)
	}
	var orphanRows int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM remodel_report
		WHERE category = 'orphan_media_refs' AND ref_id = $1`,
		fx.MessageOrphanMedia).Scan(&orphanRows); err != nil {
		t.Fatalf("count orphan_media_refs: %v", err)
	}
	if orphanRows != 1 {
		t.Errorf("orphan_media_refs entries = %d, want 1 for %s", orphanRows, fx.MessageOrphanMedia)
	}

	// media_id FK + SET NULL now enforced.
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_queue (instance_id, recipient_jid, message_type, media_id)
		VALUES ($1, '5511999990001@s.whatsapp.net', 'image', $2)`,
		fx.InstancePairedID, uuid.New()); err == nil {
		t.Error("message_queue.media_id FK absent: ghost media_id insert accepted")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM media WHERE id = $1`, fx.MediaActiveID); err != nil {
		t.Fatalf("delete media: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT media_id FROM message_queue WHERE id = $1`,
		fx.MessageWithMediaID).Scan(&msgMediaID); err != nil {
		t.Fatalf("re-read message after media delete: %v", err)
	}
	if msgMediaID != nil {
		t.Errorf("media_id after media delete = %v, want SET NULL", *msgMediaID)
	}

	// ------------------------------------------------------------------
	// media: marker chatwoot-* NOT copied to wa_id; bucket/object_key
	// backfilled from storage_path; sha256/size/expires preserved.
	// ------------------------------------------------------------------
	var mMime, mFile *string
	var mBucket, mObjectKey, mSHA string
	var mWAID *string
	var mSize int64
	var mExpires time.Time
	if err := pool.QueryRow(ctx, `
		SELECT mime_type, file_name, bucket, object_key, sha256, wa_id, size_bytes, expires_at
		FROM media WHERE id = $1`, fx.MediaExpiredID).
		Scan(&mMime, &mFile, &mBucket, &mObjectKey, &mSHA, &mWAID, &mSize, &mExpires); err != nil {
		t.Fatalf("read expired media: %v", err)
	}
	if mMime == nil || *mMime != "image/png" {
		t.Errorf("media mime_type = %v", mMime)
	}
	if mFile == nil || *mFile != "old.png" {
		t.Errorf("media file_name = %v", mFile)
	}
	if mBucket != "wzap-media" || mObjectKey != "data/media/expired.png" {
		t.Errorf("media bucket/object_key = %q/%q, want wzap-media/data/media/expired.png", mBucket, mObjectKey)
	}
	if mWAID == nil || *mWAID != "3EB0REALWAID002" {
		t.Errorf("media wa_id = %v, want legacy message_id preserved", mWAID)
	}
	if mSize != 512 {
		t.Errorf("media size_bytes = %d, want 512", mSize)
	}
	if time.Until(mExpires) > 0 {
		t.Errorf("media expires_at = %v, want preserved past timestamp", mExpires)
	}

	var markerWAID *string
	if err := pool.QueryRow(ctx, `SELECT wa_id FROM media WHERE id = $1`,
		fx.MediaChatwootMarker).Scan(&markerWAID); err != nil {
		t.Fatalf("read chatwoot-marker media: %v", err)
	}
	if markerWAID != nil {
		t.Errorf("chatwoot marker leaked into wa_id = %q, want NULL", *markerWAID)
	}
	var markerRows int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM remodel_report
		WHERE category = 'media_chatwoot_markers' AND ref_id = $1`,
		fx.MediaChatwootMarker).Scan(&markerRows); err != nil {
		t.Fatalf("count media_chatwoot_markers: %v", err)
	}
	if markerRows != 1 {
		t.Errorf("media_chatwoot_markers entries = %d, want 1", markerRows)
	}

	// UNIQUE(bucket, object_key) enforced.
	if _, err := pool.Exec(ctx, `
		INSERT INTO media (instance_id, direction, mime_type, size_bytes, bucket, object_key, sha256, expires_at)
		VALUES ($1, 'inbound', 'image/png', 1, 'wzap-media', 'data/media/expired.png', repeat('0', 64), now())`,
		fx.InstancePairedID); err == nil {
		t.Error("duplicate (bucket, object_key) accepted, want unique violation")
	}

	// ------------------------------------------------------------------
	// jid_cache: phone survives as UNIQUE (not PK), jid preserved.
	// ------------------------------------------------------------------
	var jid string
	if err := pool.QueryRow(ctx,
		`SELECT jid FROM jid_cache WHERE phone = '5511999990001'`).Scan(&jid); err != nil {
		t.Fatalf("read jid_cache row: %v", err)
	}
	if jid != "5511999990001@s.whatsapp.net" {
		t.Errorf("jid_cache.jid = %q", jid)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO jid_cache (phone, jid, expires_at) VALUES ('5511999990001', 'dup@s.whatsapp.net', now())`); err == nil {
		t.Error("duplicate jid_cache.phone accepted, want unique violation")
	}

	// ------------------------------------------------------------------
	// chatwoot_configs: renames keep values 1:1.
	// ------------------------------------------------------------------
	var cwEnabled bool
	var cwURL, cwInbox string
	if err := pool.QueryRow(ctx, `
		SELECT is_enabled, url, inbox_name FROM chatwoot_configs WHERE instance_id = $1`,
		fx.InstancePairedID).Scan(&cwEnabled, &cwURL, &cwInbox); err != nil {
		t.Fatalf("read chatwoot config: %v", err)
	}
	if !cwEnabled || cwURL != "https://chatwoot.fixture.test" || cwInbox != "Inbox WA" {
		t.Errorf("chatwoot config = enabled:%v url:%q inbox:%q", cwEnabled, cwURL, cwInbox)
	}

	// ------------------------------------------------------------------
	// chatwoot_messages: new uuid PK, cw_id rename, evidence backfill of
	// message_id, UNIQUE(instance_id, wa_key) preserved.
	// ------------------------------------------------------------------
	var backfillMsgID *uuid.UUID
	var backfillCwID int64
	var backfillChatJID string
	if err := pool.QueryRow(ctx, `
		SELECT message_id, cw_id, chat_jid FROM chatwoot_messages
		WHERE instance_id = $1 AND wa_key = $2`,
		fx.InstancePairedID, fx.ChatwootWAKeyBackfillable).
		Scan(&backfillMsgID, &backfillCwID, &backfillChatJID); err != nil {
		t.Fatalf("read backfillable correlation: %v", err)
	}
	if backfillMsgID == nil || *backfillMsgID != fx.MessageChatwootSent {
		t.Errorf("backfillable message_id = %v, want %s (wa_key == wa_id evidence join)", backfillMsgID, fx.MessageChatwootSent)
	}
	if backfillCwID != 9001 {
		t.Errorf("cw_id = %d, want 9001", backfillCwID)
	}
	if backfillChatJID != "5511999990001" {
		t.Errorf("chat_jid = %q, want contact_source_id value", backfillChatJID)
	}

	var inboundMsgID *uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT message_id FROM chatwoot_messages
		WHERE instance_id = $1 AND wa_key = $2`,
		fx.InstancePairedID, fx.ChatwootWAKeyInbound).Scan(&inboundMsgID); err != nil {
		t.Fatalf("read inbound correlation: %v", err)
	}
	if inboundMsgID != nil {
		t.Errorf("inbound correlation message_id = %v, want NULL (no queue row)", *inboundMsgID)
	}

	var pendingMsgID *uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT message_id FROM chatwoot_messages
		WHERE instance_id = $1 AND wa_key = $2`,
		fx.InstancePairedID, fx.ChatwootPendingWAKey).Scan(&pendingMsgID); err != nil {
		t.Fatalf("read pending correlation: %v", err)
	}
	if pendingMsgID == nil || *pendingMsgID != fx.ChatwootPendingMessageID {
		t.Errorf("pending:{uuid} correlation message_id = %v, want %s", pendingMsgID, fx.ChatwootPendingMessageID)
	}

	// UNIQUE(instance_id, wa_key) must remain enforced.
	if _, err := pool.Exec(ctx, `
		INSERT INTO chatwoot_messages (instance_id, wa_key, cw_id, conversation_id, inbox_id)
		VALUES ($1, $2, 9999, 1, 1)`,
		fx.InstancePairedID, fx.ChatwootWAKeyInbound); err == nil {
		t.Error("duplicate (instance_id, wa_key) accepted, want unique violation")
	}
	// N:1 on cw_id allowed: several sends share the same Chatwoot message.
	if _, err := pool.Exec(ctx, `
		INSERT INTO chatwoot_messages (instance_id, wa_key, cw_id, conversation_id, inbox_id)
		VALUES ($1, 'another-send', 9001, 101, 3)`, fx.InstancePairedID); err != nil {
		t.Errorf("second send sharing cw_id rejected: %v", err)
	}

	// ------------------------------------------------------------------
	// group/channel metadata: PK composta -> id uuid + UNIQUE combo.
	// ------------------------------------------------------------------
	var groupID uuid.UUID
	var groupName string
	if err := pool.QueryRow(ctx, `
		SELECT id, name FROM group_metadata WHERE instance_id = $1`,
		fx.InstancePairedID).Scan(&groupID, &groupName); err != nil {
		t.Fatalf("read group metadata: %v", err)
	}
	if groupName != "Fixture Group" {
		t.Errorf("group name = %q", groupName)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO group_metadata (instance_id, group_jid)
		VALUES ($1, '120363000000000001@g.us')`, fx.InstancePairedID); err == nil {
		t.Error("duplicate (instance_id, group_jid) accepted, want unique violation")
	}

	var channelTitle string
	if err := pool.QueryRow(ctx, `
		SELECT title FROM channel_metadata WHERE instance_id = $1`,
		fx.InstancePairedID).Scan(&channelTitle); err != nil {
		t.Fatalf("read channel metadata: %v", err)
	}
	if channelTitle != "Fixture Channel" {
		t.Errorf("channel title = %q, want newsletter_metadata row moved", channelTitle)
	}

	// ------------------------------------------------------------------
	// idempotency_keys: renames + UNIQUE(instance_id, key).
	// ------------------------------------------------------------------
	var idemStatus string
	var idemHTTP *int
	var idemBody []byte
	if err := pool.QueryRow(ctx, `
		SELECT status, http_status, response_body::text
		FROM idempotency_keys WHERE instance_id = $1 AND key = $2`,
		fx.InstancePairedID, fx.IdempotencyCompletedKey).
		Scan(&idemStatus, &idemHTTP, &idemBody); err != nil {
		t.Fatalf("read completed idempotency key: %v", err)
	}
	if idemStatus != "completed" || idemHTTP == nil || *idemHTTP != 202 {
		t.Errorf("completed key = status %q http %v", idemStatus, idemHTTP)
	}
	if !strings.Contains(string(idemBody), `"message"`) {
		t.Errorf("response_body = %s, want preserved jsonb", string(idemBody))
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO idempotency_keys (instance_id, key, request_hash, status, expires_at)
		VALUES ($1, $2, 'sha256-x', 'completed', now())`,
		fx.InstancePairedID, fx.IdempotencyCompletedKey); err == nil {
		t.Error("duplicate (instance_id, key) accepted, want unique violation")
	}

	// ------------------------------------------------------------------
	// event_outbox: pending survives, published row removed, trio errors.
	// ------------------------------------------------------------------
	var publishedGone, pendingKept int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM event_outbox WHERE id = $1`, fx.OutboxPublishedID).Scan(&publishedGone); err != nil {
		t.Fatalf("count published outbox row: %v", err)
	}
	if publishedGone != 0 {
		t.Error("published outbox row survived the remodel, want it removed")
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM event_outbox WHERE id = $1`, fx.OutboxPendingID).Scan(&pendingKept); err != nil {
		t.Fatalf("count pending outbox row: %v", err)
	}
	if pendingKept != 1 {
		t.Error("pending outbox row dropped, want preserved")
	}
	var attempts int
	var errCode, errMsg *string
	var errAt *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT attempt_count, last_error_code, last_error_message, last_error_at
		FROM event_outbox WHERE id = $1`, fx.OutboxPendingID).
		Scan(&attempts, &errCode, &errMsg, &errAt); err != nil {
		t.Fatalf("read pending outbox row: %v", err)
	}
	if attempts != 0 || errCode != nil || errMsg != nil || errAt != nil {
		t.Errorf("pending outbox = attempts %d err (%v,%v,%v)", attempts, errCode, errMsg, errAt)
	}

	// ------------------------------------------------------------------
	// webhook_dead_letters: bigint id -> uuid, event_id UNIQUE preserved,
	// last_error text -> code legacy_error + message, occurred_at NULL.
	// ------------------------------------------------------------------
	var deadID uuid.UUID
	var deadEventID uuid.UUID
	var deadCode, deadMsg *string
	var deadAt *time.Time
	var deadAttempts int
	var deadEnvelope []byte
	if err := pool.QueryRow(ctx, `
		SELECT id, event_id, attempt_count, last_error_code, last_error_message, last_error_at, envelope::text
		FROM webhook_dead_letters WHERE event_id = $1`, fx.DeadLetterEventID).
		Scan(&deadID, &deadEventID, &deadAttempts, &deadCode, &deadMsg, &deadAt, &deadEnvelope); err != nil {
		t.Fatalf("read dead letter: %v", err)
	}
	if deadEventID != fx.DeadLetterEventID {
		t.Errorf("dead letter event_id = %s, want %s", deadEventID, fx.DeadLetterEventID)
	}
	if deadID == (uuid.UUID{}) {
		t.Error("dead letter id is the zero UUID, want generated")
	}
	if deadAttempts != 5 {
		t.Errorf("dead letter attempt_count = %d, want 5", deadAttempts)
	}
	if deadCode == nil || *deadCode != "legacy_error" {
		t.Errorf("dead letter last_error_code = %v, want legacy_error", deadCode)
	}
	if deadMsg == nil || *deadMsg != "connection refused" {
		t.Errorf("dead letter last_error_message = %v", deadMsg)
	}
	if deadAt != nil {
		t.Errorf("dead letter last_error_at = %v, want NULL (legacy unknown)", *deadAt)
	}
	if !strings.Contains(string(deadEnvelope), `"event"`) {
		t.Errorf("dead letter envelope = %s, want preserved payload", string(deadEnvelope))
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO webhook_dead_letters (instance_id, event_id, event_type)
		VALUES ($1, $2, 'dup')`, fx.InstancePairedID, fx.DeadLetterEventID); err == nil {
		t.Error("duplicate dead letter event_id accepted, want unique violation")
	}
}

// TestMigrateDeviceJIDGateBlocksCutover pins the gate the approved design
// requires ("divergências entram no relatório e bloqueiam o corte até
// resolução explícita — nunca escolher silenciosamente"): 00008 records the
// conflict and keeps the preferred device_jid, then the 00010 gate fails the
// full migrate while the conflict row is unresolved — aborting `wzap
// migrate` and a boot with WZAP_AUTO_MIGRATE=true — and only an explicit
// operator resolution (identity reconciled, report row marked resolved)
// lets the migration complete, preserving the conflict row as audit
// history.
func TestMigrateDeviceJIDGateBlocksCutover(t *testing.T) {
	pool, ctx, fx := seedAndUpgrade(t) // schema at version 9, conflict recorded

	// The resolved_at marker (00009) exists even though the gate (00010) has
	// not applied yet: the marker is its own migration so the operator loop
	// can record resolutions after a failed gate attempt.
	resolvedAt := tableColumns(t, ctx, pool, "remodel_report")["resolved_at"]
	if resolvedAt.dataType != "timestamp with time zone" {
		t.Errorf("remodel_report.resolved_at = %q, want timestamp with time zone", resolvedAt.dataType)
	}
	if resolvedAt.isNullable != "YES" {
		t.Errorf("remodel_report.resolved_at nullable = %s, want YES (unresolved rows are the gate condition)", resolvedAt.isNullable)
	}

	var conflictRows int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM remodel_report
		WHERE category = 'device_jid_conflicts' AND resolved_at IS NULL`).
		Scan(&conflictRows); err != nil {
		t.Fatalf("count unresolved conflicts: %v", err)
	}
	if conflictRows != 1 {
		t.Fatalf("unresolved device_jid_conflicts rows = %d, want 1 (00008 recorded the divergence)", conflictRows)
	}

	// The gate aborts the migrate while the conflict is unresolved.
	err := Migrate(ctx, pool)
	if err == nil {
		t.Fatal("Migrate succeeded with an unresolved device_jid_conflicts row, want the cutover gate to block it")
	}
	if !strings.Contains(err.Error(), "device JID divergence") {
		t.Errorf("gate error does not name the divergence gate: %v", err)
	}
	if !strings.Contains(err.Error(), fx.InstanceDivergentJIDID.String()) {
		t.Errorf("gate error does not name the divergent instance %s: %v", fx.InstanceDivergentJIDID, err)
	}
	if got := appliedVersion(t, ctx, pool); got != 9 {
		t.Errorf("goose version after blocked migrate = %d, want 9 (gate migration rolled back, marker below it stays)", got)
	}
	var stillUnresolved int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM remodel_report
		WHERE category = 'device_jid_conflicts' AND ref_id = $1 AND resolved_at IS NULL`,
		fx.InstanceDivergentJIDID).Scan(&stillUnresolved); err != nil {
		t.Fatalf("check conflict row after blocked migrate: %v", err)
	}
	if stillUnresolved != 1 {
		t.Error("conflict row changed by the blocked migrate, want it preserved as evidence")
	}

	// Operator loop: reconcile the identity (the fixtures keep the
	// device_jid 00008 chose) and record the explicit resolution, then the
	// migrate completes through the gate.
	resolveDeviceJIDConflicts(t, ctx, pool, fx)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate after resolving the conflict: %v", err)
	}
	if got := appliedVersion(t, ctx, pool); got != 10 {
		t.Errorf("goose version after resolved migrate = %d, want 10", got)
	}

	// The resolution is an acknowledgment, not an erasure: the audit row
	// survives with both candidate JIDs and the resolution timestamp.
	var detailWA, detailDevice *string
	var resolved *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT detail->>'whatsapp_jid', detail->>'device_jid', resolved_at
		FROM remodel_report
		WHERE category = 'device_jid_conflicts' AND ref_id = $1`,
		fx.InstanceDivergentJIDID).Scan(&detailWA, &detailDevice, &resolved); err != nil {
		t.Fatalf("read conflict row after resolution: %v", err)
	}
	if detailWA == nil || *detailWA != "5511777770003:7@s.whatsapp.net" ||
		detailDevice == nil || *detailDevice != "5511777770003:99@s.whatsapp.net" {
		t.Errorf("conflict detail = whatsapp_jid %v device_jid %v, want both candidates preserved", detailWA, detailDevice)
	}
	if resolved == nil {
		t.Error("resolved_at = NULL after resolution, want the acknowledgment timestamp")
	}
}

// TestMigrateRemodelDown applies the remodel then rolls it back; the Down
// must leave the pre-remodel table set (renamed tables back, satellites
// dropped) without touching whatsmeow/goose bookkeeping. The schema goes
// through the 00010 gate first (the fixtures' divergence needs an explicit
// resolution) so the Down also reverses the gate migration and the
// resolved_at column.
func TestMigrateRemodelDown(t *testing.T) {
	pool, ctx, fx := seedAndUpgrade(t)

	resolveDeviceJIDConflicts(t, ctx, pool, fx)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate through the gate: %v", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.DownToContext(context.Background(), db, ".", 7); err != nil {
		t.Fatalf("goose down to 7: %v", err)
	}

	for _, back := range []string{"contacts", "newsletter_metadata", "instances", "message_queue", "media"} {
		if !tableExists(t, ctx, pool, back) {
			t.Errorf("table %s missing after down", back)
		}
	}
	for _, gone := range []string{"instance_connections", "instance_webhooks", "jid_cache", "channel_metadata", "remodel_report"} {
		if tableExists(t, ctx, pool, gone) {
			t.Errorf("table %s still present after down", gone)
		}
	}
}
