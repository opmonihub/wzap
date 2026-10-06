package postgrestest

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RemodelFixtures exposes the IDs seeded by SeedRemodelFixtures so upgrade
// tests can assert each scenario without hardcoding generated UUIDs.
type RemodelFixtures struct {
	// Users.
	AdminUserID uuid.UUID // role=admin, quota=10
	OwnerUserID uuid.UUID // role=user, quota=5, owns InstancePairedID

	// Instances: cover every device identity state of the baseline.
	InstanceNeverPairedID  uuid.UUID // no whatsapp_jid, no device_jid
	InstancePairedID       uuid.UUID // whatsapp_jid == device_jid (00007 backfill done)
	InstanceLegacyOnlyID   uuid.UUID // whatsapp_jid set, device_jid NULL (pre-00007 drift)
	InstanceDivergentJIDID uuid.UUID // whatsapp_jid != device_jid (device_jid_conflicts case)

	// Messages on InstancePairedID.
	MessageTextID       uuid.UUID // queued, no media_id
	MessageWithMediaID  uuid.UUID // sent, media_id -> MediaActiveID
	MessageOrphanMedia  uuid.UUID // media_id points to a deleted media (orphan_media_refs case)
	OrphanMediaGhostID  uuid.UUID // the media id MessageOrphanMedia references (row absent)
	MessageChatwootSent uuid.UUID // sent outbound that ChatwootMessageBackfillable can join on

	// Media on InstancePairedID.
	MediaActiveID        uuid.UUID // expires_at in the future, message_id is a real WA id
	MediaExpiredID       uuid.UUID // expires_at in the past
	MediaChatwootMarker  uuid.UUID // message_id is the synthetic "chatwoot-{id}-{idx}" marker
	ChatwootMarkerString string    // the marker value stored in MediaChatwootMarker.message_id

	// Chatwoot correlations on InstancePairedID (chatwoot_messages PK is the
	// composite (instance_id, wa_key), so wa_key strings identify the rows).
	ChatwootWAKeyBackfillable string    // equals MessageChatwootSent.whatsapp_id (evidence join)
	ChatwootWAKeyInbound      string    // inbound msg wa_key, no queue row (message_id stays NULL)
	ChatwootPendingMessageID  uuid.UUID // queue row awaiting wa_id, correlated by pending:{uuid} wa_key
	ChatwootPendingWAKey      string    // the synthetic "pending:<uuid>" wa_key used above

	// Idempotency on InstancePairedID.
	IdempotencyCompletedKey  string // status=completed, has response_status/body
	IdempotencyInProgressKey string // status=in_progress, no response

	// Dead letter on InstancePairedID.
	DeadLetterEventID uuid.UUID

	// Outbox.
	OutboxPublishedID uuid.UUID // published_at set (legacy retained row)
	OutboxPendingID   uuid.UUID // published_at NULL
}

// SeedRemodelFixtures populates the isolated schema of pool (from NewPool +
// postgres.Migrate) with rows that exercise every remodel edge case listed in
// the change design: device identity states, media_id presence/absence,
// orphaned media references, chatwoot-* media markers, chatwoot_messages with
// and without a queue message to correlate, dead letters, completed
// idempotency keys, expired media, and published/pending outbox rows.
//
// All inserts use the CURRENT (pre-remodel) schema; upgrade tests run the new
// migrations on top and assert preservation/transformation of these rows.
func SeedRemodelFixtures(t testing.TB, pool *pgxpool.Pool) RemodelFixtures {
	t.Helper()
	ctx := context.Background()
	f := RemodelFixtures{
		AdminUserID:               uuid.New(),
		OwnerUserID:               uuid.New(),
		InstanceNeverPairedID:     uuid.New(),
		InstancePairedID:          uuid.New(),
		InstanceLegacyOnlyID:      uuid.New(),
		InstanceDivergentJIDID:    uuid.New(),
		MessageTextID:             uuid.New(),
		MessageWithMediaID:        uuid.New(),
		MessageOrphanMedia:        uuid.New(),
		OrphanMediaGhostID:        uuid.New(),
		MessageChatwootSent:       uuid.New(),
		MediaActiveID:             uuid.New(),
		MediaExpiredID:            uuid.New(),
		MediaChatwootMarker:       uuid.New(),
		ChatwootWAKeyBackfillable: "3EB0CWSENT001",
		ChatwootWAKeyInbound:      "3EB0INBOUND001",
		ChatwootPendingMessageID:  uuid.New(),
		DeadLetterEventID:         uuid.New(),
		OutboxPublishedID:         uuid.New(),
		OutboxPendingID:           uuid.New(),
		IdempotencyCompletedKey:   "fixture-key-completed",
		IdempotencyInProgressKey:  "fixture-key-in-progress",
	}
	f.ChatwootMarkerString = "chatwoot-4242-1"
	f.ChatwootPendingWAKey = "pending:" + f.ChatwootPendingMessageID.String()

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("seed fixture: %v\nquery: %s", err, query)
		}
	}

	// --- users ---
	exec(`INSERT INTO users (id, email, password_hash, role, instance_quota)
	      VALUES ($1, 'admin@fixture.test', 'bcrypt-admin', 'admin', 10)`, f.AdminUserID)
	exec(`INSERT INTO users (id, email, password_hash, role, instance_quota)
	      VALUES ($1, 'owner@fixture.test', 'bcrypt-owner', 'user', 5)`, f.OwnerUserID)

	// --- instances ---
	exec(`INSERT INTO instances (id, name, status) VALUES ($1, 'fx-never-paired', 'disconnected')`,
		f.InstanceNeverPairedID)
	exec(`INSERT INTO instances (id, name, external_ref, status, whatsapp_jid, device_jid,
	                             last_connected_at, owner_user_id)
	      VALUES ($1, 'fx-paired', 'ext-paired', 'connected', '5511999990001:5@s.whatsapp.net',
	              '5511999990001:5@s.whatsapp.net', now(), $2)`,
		f.InstancePairedID, f.OwnerUserID)
	// Legacy row: whatsapp_jid present, device_jid still NULL (00007 not yet run/drifted).
	exec(`INSERT INTO instances (id, name, status, whatsapp_jid)
	      VALUES ($1, 'fx-legacy-jid', 'disconnected', '5511888880002:12@s.whatsapp.net')`,
		f.InstanceLegacyOnlyID)
	// Divergent: both set but different -> device_jid_conflicts report entry.
	exec(`INSERT INTO instances (id, name, status, whatsapp_jid, device_jid)
	      VALUES ($1, 'fx-divergent-jid', 'error', '5511777770003:7@s.whatsapp.net',
	              '5511777770003:99@s.whatsapp.net')`,
		f.InstanceDivergentJIDID)

	// --- media on the paired instance ---
	exec(`INSERT INTO media (id, instance_id, direction, message_id, mimetype, filename,
	                         size_bytes, storage_path, sha256, expires_at)
	      VALUES ($1, $2, 'inbound', '3EB0REALWAID001', 'image/jpeg', 'photo.jpg',
	              1024, 'data/media/active.jpg', repeat('a', 64), now() + interval '24 hours')`,
		f.MediaActiveID, f.InstancePairedID)
	exec(`INSERT INTO media (id, instance_id, direction, message_id, mimetype, filename,
	                         size_bytes, storage_path, sha256, expires_at)
	      VALUES ($1, $2, 'inbound', '3EB0REALWAID002', 'image/png', 'old.png',
	              512, 'data/media/expired.png', repeat('b', 64), now() - interval '1 hour')`,
		f.MediaExpiredID, f.InstancePairedID)
	// Synthetic chatwoot marker must NOT become wa_id after remodel.
	exec(`INSERT INTO media (id, instance_id, direction, message_id, mimetype, size_bytes,
	                         storage_path, sha256, expires_at)
	      VALUES ($1, $2, 'inbound', $3, 'image/webp', 256, 'data/media/cw-marker.webp',
	              repeat('c', 64), now() + interval '24 hours')`,
		f.MediaChatwootMarker, f.InstancePairedID, f.ChatwootMarkerString)

	// --- message_queue ---
	exec(`INSERT INTO message_queue (id, instance_id, recipient, type, payload, status)
	      VALUES ($1, $2, '5511999990001@s.whatsapp.net', 'text', '{"text":"queued"}', 'queued')`,
		f.MessageTextID, f.InstancePairedID)
	exec(`INSERT INTO message_queue (id, instance_id, recipient, type, payload, status,
	                                 whatsapp_id, media_id, delivered_at)
	      VALUES ($1, $2, '5511999990001@s.whatsapp.net', 'image',
	              '{"caption":"with media"}', 'sent', '3EB0SENTWITHMEDIA', $3, now())`,
		f.MessageWithMediaID, f.InstancePairedID, f.MediaActiveID)
	// media_id pointing at a media row that no longer exists (pre-FK baseline
	// allows it; remodel must null it instead of failing the migration).
	exec(`INSERT INTO message_queue (id, instance_id, recipient, type, payload, status,
	                                 media_id)
	      VALUES ($1, $2, '5511999990001@s.whatsapp.net', 'video', '{}', 'failed', $3)`,
		f.MessageOrphanMedia, f.InstancePairedID, f.OrphanMediaGhostID)
	// Sent outbound whose whatsapp_id a chatwoot_messages.wa_key can join on.
	exec(`INSERT INTO message_queue (id, instance_id, recipient, type, payload, status,
	                                 whatsapp_id, delivered_at)
	      VALUES ($1, $2, '5511999990001@s.whatsapp.net', 'text', '{"text":"cw"}',
	              'sent', '3EB0CWSENT001', now())`,
		f.MessageChatwootSent, f.InstancePairedID)
	// Queue row still awaiting wa_id (pre-WA-ID window for the cw webhook).
	exec(`INSERT INTO message_queue (id, instance_id, recipient, type, payload, status)
	      VALUES ($1, $2, '5511999990001@s.whatsapp.net', 'text', '{"text":"pending"}',
	              'queued')`,
		f.ChatwootPendingMessageID, f.InstancePairedID)

	// --- chatwoot_messages ---
	exec(`INSERT INTO chatwoot_configs (instance_id, enabled, url, account_id, name_inbox)
	      VALUES ($1, true, 'https://chatwoot.fixture.test', '7', 'Inbox WA')`,
		f.InstancePairedID)
	// Backfillable: wa_key equals a real queue whatsapp_id on the same instance.
	exec(`INSERT INTO chatwoot_messages (instance_id, wa_key, chatwoot_message_id,
	                                     conversation_id, inbox_id, contact_source_id)
	      VALUES ($1, $2, 9001, 101, 3, '5511999990001')`,
		f.InstancePairedID, f.ChatwootWAKeyBackfillable)
	// Inbound-only correlation: no queue row exists -> message_id stays NULL.
	exec(`INSERT INTO chatwoot_messages (instance_id, wa_key, chatwoot_message_id,
	                                     conversation_id, inbox_id, contact_source_id, is_read)
	      VALUES ($1, $2, 9002, 101, 3, '5511888880002', true)`,
		f.InstancePairedID, f.ChatwootWAKeyInbound)
	// Pending window marker: synthetic wa_key reserved prefix.
	exec(`INSERT INTO chatwoot_messages (instance_id, wa_key, chatwoot_message_id,
	                                     conversation_id, inbox_id, contact_source_id)
	      VALUES ($1, $2, 9003, 101, 3, '5511999990001')`,
		f.InstancePairedID, f.ChatwootPendingWAKey)

	// --- idempotency_keys ---
	exec(`INSERT INTO idempotency_keys (instance_id, idempotency_key, request_hash, status,
	                                    response_status, response_body, expires_at)
	      VALUES ($1, $2, 'sha256-completed', 'completed', 202,
	              '{"data":{"message":{"id":"00000000-0000-0000-0000-0000000000aa"}}}'::jsonb,
	              now() + interval '24 hours')`,
		f.InstancePairedID, f.IdempotencyCompletedKey)
	exec(`INSERT INTO idempotency_keys (instance_id, idempotency_key, request_hash, status,
	                                    expires_at)
	      VALUES ($1, $2, 'sha256-inprogress', 'in_progress', now() + interval '24 hours')`,
		f.InstancePairedID, f.IdempotencyInProgressKey)

	// --- webhook_dead_letters ---
	exec(`INSERT INTO webhook_dead_letters (instance_id, event_id, event_type, payload,
	                                        attempts, last_error)
	      VALUES ($1, $2, 'message', '{"event":"message"}'::jsonb, 5, 'connection refused')`,
		f.InstancePairedID, f.DeadLetterEventID)

	// --- event_outbox: one retained published row + one pending ---
	exec(`INSERT INTO event_outbox (id, subject, envelope, attempts, published_at)
	      VALUES ($1, 'wzap.instances.fx-paired.message',
	              '{"event_version":1,"event_id":"00000000-0000-0000-0000-0000000000e1"}'::jsonb,
	              1, now())`,
		f.OutboxPublishedID)
	exec(`INSERT INTO event_outbox (id, subject, envelope)
	      VALUES ($1, 'wzap.instances.fx-paired.connection',
	              '{"event_version":1,"event_id":"00000000-0000-0000-0000-0000000000e2"}'::jsonb)`,
		f.OutboxPendingID)

	// --- contacts (future jid_cache) ---
	exec(`INSERT INTO contacts (phone, jid, expires_at)
	      VALUES ('5511999990001', '5511999990001@s.whatsapp.net', now() + interval '7 days')`)

	// --- group/newsletter metadata ---
	exec(`INSERT INTO group_metadata (instance_id, group_jid, name, participant_count)
	      VALUES ($1, '120363000000000001@g.us', 'Fixture Group', 12)`, f.InstancePairedID)
	exec(`INSERT INTO newsletter_metadata (instance_id, channel_jid, title, follower_count)
	      VALUES ($1, '120363000000000002@newsletter', 'Fixture Channel', 34)`, f.InstancePairedID)

	if err := assertSeeded(ctx, pool); err != nil {
		t.Fatalf("fixture self-check: %v", err)
	}
	return f
}

// assertSeeded is a cheap smoke check that the seed produced the expected
// table cardinalities; it catches silent skips (e.g. missing NOT NULL) before
// the upgrade tests run.
func assertSeeded(ctx context.Context, pool *pgxpool.Pool) error {
	checks := []struct {
		table string
		want  int
	}{
		{"users", 2},
		{"instances", 4},
		{"media", 3},
		{"message_queue", 5},
		{"chatwoot_configs", 1},
		{"chatwoot_messages", 3},
		{"idempotency_keys", 2},
		{"webhook_dead_letters", 1},
		{"event_outbox", 2},
		{"contacts", 1},
		{"group_metadata", 1},
		{"newsletter_metadata", 1},
	}
	for _, c := range checks {
		var got int
		if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s", c.table)).Scan(&got); err != nil {
			return fmt.Errorf("count %s: %w", c.table, err)
		}
		if got != c.want {
			return fmt.Errorf("count %s: got %d, want %d", c.table, got, c.want)
		}
	}
	return nil
}
