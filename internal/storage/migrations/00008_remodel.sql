-- +goose Up
-- Remodel of the 12 product tables into the 14 approved in
-- openspec/changes/remodel-storage-and-json-responses/design.md
-- ("Decisões fechadas na task 1.1"). Non-destructive policy: rows are moved
-- or renamed, never silently dropped; orphans/divergences are anulled and
-- recorded in remodel_report for operator review. whatsmeow_* and
-- goose_db_version are out of scope.

-- ---------------------------------------------------------------------------
-- remodel_report: migration audit trail. Every non-destructive adjustment
-- (anulled orphan FK, skipped chatwoot marker, divergent device JID) lands a
-- row here so the preflight/post-migration review can reconcile them.
-- ---------------------------------------------------------------------------
CREATE TABLE remodel_report (
  category text NOT NULL,
  ref_id uuid,
  detail jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- instances: split connection state and webhook config into satellites.
-- ---------------------------------------------------------------------------
CREATE TABLE instance_connections (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL UNIQUE REFERENCES instances(id) ON DELETE CASCADE,
  device_jid text,
  status text NOT NULL DEFAULT 'disconnected'
    CHECK (status IN ('disconnected', 'pairing', 'connected', 'error')),
  last_connected_at timestamptz,
  last_error_code text,
  last_error_message text,
  last_error_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
-- device_jid is unique only when filled; preserves the 00007 partial index.
CREATE UNIQUE INDEX instance_connections_device_jid_uidx
  ON instance_connections (device_jid)
  WHERE device_jid IS NOT NULL AND device_jid <> '';

-- Divergence audit before moving: device_jid wins, divergence is reported.
INSERT INTO remodel_report (category, ref_id, detail)
SELECT 'device_jid_conflicts', id,
       jsonb_build_object('name', name, 'whatsapp_jid', whatsapp_jid, 'device_jid', device_jid)
FROM instances
WHERE whatsapp_jid IS NOT NULL AND whatsapp_jid <> ''
  AND device_jid IS NOT NULL AND device_jid <> ''
  AND whatsapp_jid <> device_jid;

INSERT INTO instance_connections
  (instance_id, device_jid, status, last_connected_at, last_error_message,
   created_at, updated_at)
SELECT id,
       -- device_jid is authoritative (00007 rule); fall back to whatsapp_jid
       -- for rows that predate or drifted from the backfill.
       NULLIF(COALESCE(device_jid, whatsapp_jid), ''),
       CASE WHEN status IN ('disconnected', 'pairing', 'connected', 'error')
            THEN status ELSE 'disconnected' END,
       last_connected_at,
       CASE WHEN last_error IS NOT NULL AND last_error <> ''
            THEN last_error END,
       created_at, updated_at
FROM instances;

-- Legacy free-text errors keep the message; the code is filled next and the
-- occurred-at instant stays NULL (unknown, never inferred from updated_at).
UPDATE instance_connections
SET last_error_code = 'legacy_error'
WHERE last_error_message IS NOT NULL;

INSERT INTO remodel_report (category, ref_id, detail)
SELECT 'legacy_connection_errors', instance_id,
       jsonb_build_object('last_error', last_error_message)
FROM instance_connections
WHERE last_error_message IS NOT NULL;

ALTER TABLE instances
  DROP COLUMN status,
  DROP COLUMN whatsapp_jid,
  DROP COLUMN device_jid,
  DROP COLUMN last_connected_at,
  DROP COLUMN last_error;

CREATE TABLE instance_webhooks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL UNIQUE REFERENCES instances(id) ON DELETE CASCADE,
  url text,
  is_enabled bool NOT NULL DEFAULT false,
  events text[] NOT NULL DEFAULT '{message,receipt,connection,message.status}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO instance_webhooks (instance_id, url, is_enabled, events, created_at, updated_at)
SELECT id, webhook_url, webhook_enabled, webhook_events, created_at, updated_at
FROM instances;

ALTER TABLE instances
  DROP COLUMN webhook_url,
  DROP COLUMN webhook_enabled,
  DROP COLUMN webhook_events;

-- owner_user_id keeps NULL for legacy ownerless rows but now protects them:
-- deleting an operator account sets NULL instead of cascading the instances.
ALTER TABLE instances DROP CONSTRAINT instances_owner_user_id_fkey;
ALTER TABLE instances ADD CONSTRAINT instances_owner_user_id_fkey
  FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE instances ALTER COLUMN id SET DEFAULT gen_random_uuid();
CREATE INDEX instances_owner_idx ON instances (owner_user_id);
CREATE INDEX instances_name_idx ON instances (name);

-- ---------------------------------------------------------------------------
-- users: instance_quota -> instance_limit; email/password_hash become NOT NULL
-- per the approved matrix. 00002 left them NULLable, so legacy NULL rows are
-- defensively coalesced and reported before the constraint lands.
-- ---------------------------------------------------------------------------
ALTER TABLE users RENAME COLUMN instance_quota TO instance_limit;
ALTER TABLE users ALTER COLUMN id SET DEFAULT gen_random_uuid();

INSERT INTO remodel_report (category, ref_id, detail)
SELECT 'users_null_email', id, jsonb_build_object('email', email)
FROM users WHERE email IS NULL;
-- The lower(email) UNIQUE index requires distinct values; the row id keeps the
-- placeholder unique and greppable as deliberately invalid.
UPDATE users SET email = 'legacy-' || id::text || '@invalid.remodel'
WHERE email IS NULL;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

INSERT INTO remodel_report (category, ref_id, detail)
SELECT 'users_null_password_hash', id, jsonb_build_object('email', email)
FROM users WHERE password_hash IS NULL;
-- Empty string is not a valid bcrypt hash, so the account stays locked out
-- until an admin resets it; the row is preserved, never deleted.
UPDATE users SET password_hash = '' WHERE password_hash IS NULL;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;

-- ---------------------------------------------------------------------------
-- message_queue: approved names, error trio, media_id FK.
-- ---------------------------------------------------------------------------
ALTER TABLE message_queue RENAME COLUMN recipient TO recipient_jid;
ALTER TABLE message_queue RENAME COLUMN type TO message_type;
ALTER TABLE message_queue RENAME COLUMN status TO send_status;
ALTER TABLE message_queue RENAME COLUMN retries TO retry_count;
ALTER TABLE message_queue RENAME COLUMN whatsapp_id TO wa_id;
ALTER TABLE message_queue RENAME COLUMN last_error TO last_error_message;
ALTER TABLE message_queue ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE message_queue ADD COLUMN last_error_code text;
ALTER TABLE message_queue ADD COLUMN last_error_at timestamptz;
UPDATE message_queue
SET last_error_code = 'legacy_error'
WHERE last_error_message IS NOT NULL AND last_error_message <> '';

ALTER TABLE message_queue ALTER COLUMN send_status SET DEFAULT 'queued';
ALTER TABLE message_queue ADD CONSTRAINT message_queue_send_status_check
  CHECK (send_status IN ('queued', 'sending', 'sent', 'failed'));

-- Orphan media_id (points at a media row that no longer exists): preserve the
-- message, annul the dangling reference, record it in the report.
INSERT INTO remodel_report (category, ref_id, detail)
SELECT 'orphan_media_refs', mq.id,
       jsonb_build_object('instance_id', mq.instance_id, 'media_id', mq.media_id)
FROM message_queue mq
WHERE mq.media_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM media m WHERE m.id = mq.media_id);

UPDATE message_queue mq
SET media_id = NULL
WHERE mq.media_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM media m WHERE m.id = mq.media_id);

ALTER TABLE message_queue ADD CONSTRAINT message_queue_media_id_fkey
  FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE SET NULL;

DROP INDEX message_queue_instance_status_idx;
CREATE INDEX message_queue_instance_status_idx ON message_queue (instance_id, send_status);
DROP INDEX message_queue_wa_id_idx;
CREATE INDEX message_queue_wa_id_idx ON message_queue (wa_id);
CREATE INDEX message_queue_media_idx ON message_queue (media_id) WHERE media_id IS NOT NULL;
CREATE INDEX message_queue_next_attempt_idx ON message_queue (next_attempt_at)
  WHERE send_status = 'queued';

-- ---------------------------------------------------------------------------
-- media: rename, MinIO addressing columns, orphan object bookkeeping.
-- Local files keep their storage_path as object_key so the transition window
-- stays reversible; bucket defaults to the configured WZAP_S3_BUCKET.
-- ---------------------------------------------------------------------------
ALTER TABLE media RENAME COLUMN message_id TO wa_id;
ALTER TABLE media RENAME COLUMN mimetype TO mime_type;
ALTER TABLE media RENAME COLUMN filename TO file_name;
ALTER TABLE media RENAME COLUMN storage_path TO object_key;

ALTER TABLE media ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE media ADD COLUMN bucket text;
ALTER TABLE media ADD COLUMN object_deleted_at timestamptz;
ALTER TABLE media ADD COLUMN updated_at timestamptz;

UPDATE media SET bucket = 'wzap-media' WHERE bucket IS NULL;
ALTER TABLE media ALTER COLUMN bucket SET NOT NULL;
ALTER TABLE media ALTER COLUMN object_key SET NOT NULL;

-- Media whose legacy message_id is a synthetic chatwoot-{id}-{idx} marker:
-- the marker is NOT a WA ID and must not circulate as one. Report + NULL.
INSERT INTO remodel_report (category, ref_id, detail)
SELECT 'media_chatwoot_markers', id,
       jsonb_build_object('marker', wa_id, 'instance_id', instance_id)
FROM media
WHERE wa_id ~ '^chatwoot-\d+-\d+$';

UPDATE media SET wa_id = NULL WHERE wa_id ~ '^chatwoot-\d+-\d+$';

ALTER TABLE media ADD CONSTRAINT media_bucket_object_key_key UNIQUE (bucket, object_key);
DROP INDEX media_expires_idx;
CREATE INDEX media_expires_idx ON media (expires_at) WHERE object_deleted_at IS NULL;
CREATE INDEX media_instance_idx ON media (instance_id);
CREATE INDEX media_wa_id_idx ON media (wa_id) WHERE wa_id IS NOT NULL;
ALTER TABLE media ALTER COLUMN updated_at SET DEFAULT now();
UPDATE media SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE media ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE media ADD CONSTRAINT media_direction_check CHECK (direction IN ('inbound', 'outbound'));

-- ---------------------------------------------------------------------------
-- contacts -> jid_cache: PK natural -> uuid + UNIQUE(phone).
-- ---------------------------------------------------------------------------
ALTER TABLE contacts RENAME TO jid_cache;
ALTER TABLE jid_cache ADD COLUMN id uuid DEFAULT gen_random_uuid();
ALTER TABLE jid_cache ADD COLUMN updated_at timestamptz;
UPDATE jid_cache SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE jid_cache ALTER COLUMN id SET NOT NULL;
ALTER TABLE jid_cache ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE jid_cache ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE jid_cache DROP CONSTRAINT contacts_pkey;
ALTER TABLE jid_cache ADD PRIMARY KEY (id);
ALTER TABLE jid_cache ADD CONSTRAINT jid_cache_phone_key UNIQUE (phone);
ALTER TABLE contacts_expires_idx RENAME TO jid_cache_expires_idx;

-- ---------------------------------------------------------------------------
-- chatwoot_configs: approved flag/field names.
-- ---------------------------------------------------------------------------
ALTER TABLE chatwoot_configs RENAME COLUMN enabled TO is_enabled;
ALTER TABLE chatwoot_configs RENAME COLUMN name_inbox TO inbox_name;
ALTER TABLE chatwoot_configs RENAME COLUMN sign_msg TO is_sign_enabled;
ALTER TABLE chatwoot_configs RENAME COLUMN reopen_conversation TO is_reopen_enabled;
ALTER TABLE chatwoot_configs RENAME COLUMN conversation_pending TO is_pending_enabled;
ALTER TABLE chatwoot_configs RENAME COLUMN merge_brazil_contacts TO is_merge_enabled;
ALTER TABLE chatwoot_configs RENAME COLUMN import_contacts TO is_import_contacts;
ALTER TABLE chatwoot_configs RENAME COLUMN import_messages TO is_import_messages;
ALTER TABLE chatwoot_configs RENAME COLUMN days_limit TO import_days;
ALTER TABLE chatwoot_configs RENAME COLUMN auto_create TO is_auto_create;
ALTER TABLE chatwoot_configs RENAME COLUMN ignore_jids TO ignored_jids;
ALTER TABLE chatwoot_configs ADD COLUMN id uuid DEFAULT gen_random_uuid();
UPDATE chatwoot_configs SET id = gen_random_uuid() WHERE id IS NULL;
ALTER TABLE chatwoot_configs ALTER COLUMN id SET NOT NULL;
ALTER TABLE chatwoot_configs DROP CONSTRAINT chatwoot_configs_pkey;
ALTER TABLE chatwoot_configs ADD PRIMARY KEY (id);
ALTER TABLE chatwoot_configs ADD CONSTRAINT chatwoot_configs_instance_id_key UNIQUE (instance_id);

-- ---------------------------------------------------------------------------
-- chatwoot_messages: composite PK -> uuid + UNIQUE(instance_id, wa_key),
-- cw_id/chat_jid renames, optional message_id FK with evidence-only backfill.
-- ---------------------------------------------------------------------------
ALTER TABLE chatwoot_messages RENAME COLUMN chatwoot_message_id TO cw_id;
ALTER TABLE chatwoot_messages RENAME COLUMN contact_source_id TO chat_jid;
ALTER TABLE chatwoot_messages ADD COLUMN id uuid DEFAULT gen_random_uuid();
ALTER TABLE chatwoot_messages ADD COLUMN message_id uuid;
ALTER TABLE chatwoot_messages ADD COLUMN updated_at timestamptz;
UPDATE chatwoot_messages SET id = gen_random_uuid() WHERE id IS NULL;
UPDATE chatwoot_messages SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE chatwoot_messages ALTER COLUMN id SET NOT NULL;
ALTER TABLE chatwoot_messages ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE chatwoot_messages ALTER COLUMN updated_at SET DEFAULT now();

-- Backfill message_id only with evidence: wa_key equals a real queue wa_id on
-- the same instance, or the reserved pending:{uuid} marker of a queue row
-- still awaiting its WA ID. Inbound/edit correlations stay NULL.
UPDATE chatwoot_messages cm
SET message_id = mq.id
FROM message_queue mq
WHERE cm.instance_id = mq.instance_id
  AND mq.wa_id IS NOT NULL
  AND cm.wa_key = mq.wa_id;

UPDATE chatwoot_messages cm
SET message_id = mq.id
FROM message_queue mq
WHERE cm.instance_id = mq.instance_id
  AND cm.message_id IS NULL
  AND cm.wa_key = 'pending:' || mq.id::text;

ALTER TABLE chatwoot_messages DROP CONSTRAINT chatwoot_messages_pkey;
ALTER TABLE chatwoot_messages ADD PRIMARY KEY (id);
ALTER TABLE chatwoot_messages ADD CONSTRAINT chatwoot_messages_instance_wa_key_key
  UNIQUE (instance_id, wa_key);
ALTER TABLE chatwoot_messages ADD CONSTRAINT chatwoot_messages_message_id_fkey
  FOREIGN KEY (message_id) REFERENCES message_queue(id) ON DELETE SET NULL;

DROP INDEX chatwoot_messages_instance_msg_idx;
CREATE INDEX chatwoot_messages_instance_msg_idx ON chatwoot_messages (instance_id, cw_id);
DROP INDEX chatwoot_messages_conversation_idx;
CREATE INDEX chatwoot_messages_conversation_idx
  ON chatwoot_messages (instance_id, conversation_id, created_at DESC, cw_id DESC);
CREATE INDEX chatwoot_messages_message_id_idx ON chatwoot_messages (message_id)
  WHERE message_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- group_metadata / newsletter_metadata -> channel_metadata: uuid PKs.
-- ---------------------------------------------------------------------------
ALTER TABLE group_metadata ADD COLUMN id uuid DEFAULT gen_random_uuid();
UPDATE group_metadata SET id = gen_random_uuid() WHERE id IS NULL;
ALTER TABLE group_metadata ALTER COLUMN id SET NOT NULL;
ALTER TABLE group_metadata DROP CONSTRAINT group_metadata_pkey;
ALTER TABLE group_metadata ADD PRIMARY KEY (id);
ALTER TABLE group_metadata ADD CONSTRAINT group_metadata_instance_group_key
  UNIQUE (instance_id, group_jid);

ALTER TABLE newsletter_metadata RENAME TO channel_metadata;
ALTER TABLE channel_metadata ADD COLUMN id uuid DEFAULT gen_random_uuid();
UPDATE channel_metadata SET id = gen_random_uuid() WHERE id IS NULL;
ALTER TABLE channel_metadata ALTER COLUMN id SET NOT NULL;
ALTER TABLE channel_metadata DROP CONSTRAINT newsletter_metadata_pkey;
ALTER TABLE channel_metadata ADD PRIMARY KEY (id);
ALTER TABLE channel_metadata ADD CONSTRAINT channel_metadata_instance_channel_key
  UNIQUE (instance_id, channel_jid);

-- ---------------------------------------------------------------------------
-- idempotency_keys: composite PK -> uuid + UNIQUE(instance_id, key).
-- ---------------------------------------------------------------------------
ALTER TABLE idempotency_keys RENAME COLUMN idempotency_key TO key;
ALTER TABLE idempotency_keys RENAME COLUMN response_status TO http_status;
ALTER TABLE idempotency_keys ADD COLUMN id uuid DEFAULT gen_random_uuid();
ALTER TABLE idempotency_keys ADD COLUMN updated_at timestamptz;
UPDATE idempotency_keys SET id = gen_random_uuid() WHERE id IS NULL;
UPDATE idempotency_keys SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE idempotency_keys ALTER COLUMN id SET NOT NULL;
ALTER TABLE idempotency_keys ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE idempotency_keys ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE idempotency_keys DROP CONSTRAINT idempotency_keys_pkey;
ALTER TABLE idempotency_keys ADD PRIMARY KEY (id);
ALTER TABLE idempotency_keys ADD CONSTRAINT idempotency_keys_instance_key_key
  UNIQUE (instance_id, key);
ALTER TABLE idempotency_keys ADD CONSTRAINT idempotency_keys_status_check
  CHECK (status IN ('in_progress', 'completed'));

-- ---------------------------------------------------------------------------
-- event_outbox: pending-only table, error trio, published_at removed.
-- ---------------------------------------------------------------------------
ALTER TABLE event_outbox RENAME COLUMN attempts TO attempt_count;
ALTER TABLE event_outbox RENAME COLUMN last_error TO last_error_message;
ALTER TABLE event_outbox ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE event_outbox ADD COLUMN last_error_code text;
ALTER TABLE event_outbox ADD COLUMN last_error_at timestamptz;
ALTER TABLE event_outbox ADD COLUMN updated_at timestamptz;
UPDATE event_outbox SET last_error_code = 'legacy_error'
  WHERE last_error_message IS NOT NULL AND last_error_message <> '';
UPDATE event_outbox SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE event_outbox ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE event_outbox ALTER COLUMN updated_at SET DEFAULT now();

-- Confirmed publications are removed at the cut; pending rows survive.
-- Dropping published_at also drops the 00001 partial index that filters on
-- it, so only the replacement index is created below.
DELETE FROM event_outbox WHERE published_at IS NOT NULL;
ALTER TABLE event_outbox DROP COLUMN published_at;
CREATE INDEX event_outbox_pending_idx ON event_outbox (created_at);

-- ---------------------------------------------------------------------------
-- webhook_dead_letters: bigint id -> uuid (event_id stays the stable
-- identifier), payload -> envelope, error trio.
-- ---------------------------------------------------------------------------
ALTER TABLE webhook_dead_letters RENAME COLUMN payload TO envelope;
ALTER TABLE webhook_dead_letters RENAME COLUMN attempts TO attempt_count;
ALTER TABLE webhook_dead_letters ADD COLUMN last_error_code text;
ALTER TABLE webhook_dead_letters ADD COLUMN last_error_message text;
ALTER TABLE webhook_dead_letters ADD COLUMN last_error_at timestamptz;
ALTER TABLE webhook_dead_letters ADD COLUMN updated_at timestamptz;
UPDATE webhook_dead_letters SET last_error_message = last_error
  WHERE last_error IS NOT NULL AND last_error <> '';
UPDATE webhook_dead_letters SET last_error_code = 'legacy_error'
  WHERE last_error_message IS NOT NULL;
UPDATE webhook_dead_letters SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE webhook_dead_letters ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE webhook_dead_letters ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE webhook_dead_letters DROP COLUMN last_error;

-- Swap the bigint PK for a generated UUID. Consumers dedupe by event_id
-- (UNIQUE, preserved verbatim), so the internal id has no external meaning.
-- This swap is intentionally one-way: the Down documents that bigint ids are
-- not regenerated.
ALTER TABLE webhook_dead_letters ADD COLUMN id_new uuid DEFAULT gen_random_uuid();
UPDATE webhook_dead_letters SET id_new = gen_random_uuid() WHERE id_new IS NULL;
ALTER TABLE webhook_dead_letters ALTER COLUMN id_new SET NOT NULL;
DROP INDEX webhook_dead_letters_instance_idx;
ALTER TABLE webhook_dead_letters DROP CONSTRAINT webhook_dead_letters_pkey;
ALTER TABLE webhook_dead_letters DROP COLUMN id;
ALTER TABLE webhook_dead_letters RENAME COLUMN id_new TO id;
ALTER TABLE webhook_dead_letters ADD PRIMARY KEY (id);
CREATE INDEX webhook_dead_letters_instance_idx
  ON webhook_dead_letters (instance_id, created_at DESC, id DESC);

-- +goose Down
-- Reverse of the remodel. Column renames are reversed; the bigint PK of
-- webhook_dead_letters is NOT restored (uuid -> bigserial is lossy and the
-- design treats this swap as one-way: event_id is the stable identifier).
-- Rows in instance_connections/instance_webhooks are merged back into
-- instances; remodel_report is dropped after the merge.

ALTER TABLE webhook_dead_letters RENAME COLUMN envelope TO payload;
ALTER TABLE webhook_dead_letters RENAME COLUMN attempt_count TO attempts;
ALTER TABLE webhook_dead_letters ADD COLUMN last_error text NOT NULL DEFAULT '';
UPDATE webhook_dead_letters SET last_error = last_error_message
  WHERE last_error_message IS NOT NULL;
ALTER TABLE webhook_dead_letters DROP COLUMN last_error_code;
ALTER TABLE webhook_dead_letters DROP COLUMN last_error_message;
ALTER TABLE webhook_dead_letters DROP COLUMN last_error_at;
ALTER TABLE webhook_dead_letters DROP COLUMN updated_at;
DROP INDEX webhook_dead_letters_instance_idx;
CREATE INDEX webhook_dead_letters_instance_idx
  ON webhook_dead_letters (instance_id, created_at DESC, id DESC);

ALTER TABLE event_outbox ADD COLUMN published_at timestamptz;
ALTER TABLE event_outbox ALTER COLUMN id DROP DEFAULT;
ALTER TABLE event_outbox RENAME COLUMN attempt_count TO attempts;
ALTER TABLE event_outbox RENAME COLUMN last_error_message TO last_error;
ALTER TABLE event_outbox DROP COLUMN last_error_code;
ALTER TABLE event_outbox DROP COLUMN last_error_at;
ALTER TABLE event_outbox DROP COLUMN updated_at;
DROP INDEX event_outbox_pending_idx;
CREATE INDEX event_outbox_pending_idx ON event_outbox (created_at)
  WHERE published_at IS NULL;

ALTER TABLE idempotency_keys DROP CONSTRAINT idempotency_keys_status_check;
ALTER TABLE idempotency_keys DROP CONSTRAINT idempotency_keys_instance_key_key;
ALTER TABLE idempotency_keys DROP CONSTRAINT idempotency_keys_pkey;
ALTER TABLE idempotency_keys ADD PRIMARY KEY (instance_id, key);
ALTER TABLE idempotency_keys DROP COLUMN id;
ALTER TABLE idempotency_keys DROP COLUMN updated_at;
ALTER TABLE idempotency_keys RENAME COLUMN key TO idempotency_key;
ALTER TABLE idempotency_keys RENAME COLUMN http_status TO response_status;

ALTER TABLE channel_metadata DROP CONSTRAINT channel_metadata_instance_channel_key;
ALTER TABLE channel_metadata DROP CONSTRAINT channel_metadata_pkey;
ALTER TABLE channel_metadata ADD PRIMARY KEY (instance_id, channel_jid);
ALTER TABLE channel_metadata DROP COLUMN id;
ALTER TABLE channel_metadata RENAME TO newsletter_metadata;

ALTER TABLE group_metadata DROP CONSTRAINT group_metadata_instance_group_key;
ALTER TABLE group_metadata DROP CONSTRAINT group_metadata_pkey;
ALTER TABLE group_metadata ADD PRIMARY KEY (instance_id, group_jid);
ALTER TABLE group_metadata DROP COLUMN id;

DROP INDEX chatwoot_messages_message_id_idx;
DROP INDEX chatwoot_messages_conversation_idx;
DROP INDEX chatwoot_messages_instance_msg_idx;
ALTER TABLE chatwoot_messages DROP CONSTRAINT chatwoot_messages_message_id_fkey;
ALTER TABLE chatwoot_messages DROP CONSTRAINT chatwoot_messages_instance_wa_key_key;
ALTER TABLE chatwoot_messages DROP CONSTRAINT chatwoot_messages_pkey;
ALTER TABLE chatwoot_messages ADD PRIMARY KEY (instance_id, wa_key);
ALTER TABLE chatwoot_messages DROP COLUMN id;
ALTER TABLE chatwoot_messages DROP COLUMN message_id;
ALTER TABLE chatwoot_messages DROP COLUMN updated_at;
ALTER TABLE chatwoot_messages RENAME COLUMN cw_id TO chatwoot_message_id;
ALTER TABLE chatwoot_messages RENAME COLUMN chat_jid TO contact_source_id;
CREATE INDEX chatwoot_messages_instance_msg_idx
  ON chatwoot_messages (instance_id, chatwoot_message_id);
CREATE INDEX chatwoot_messages_conversation_idx
  ON chatwoot_messages (instance_id, conversation_id, created_at DESC, chatwoot_message_id DESC);

ALTER TABLE chatwoot_configs DROP CONSTRAINT chatwoot_configs_instance_id_key;
ALTER TABLE chatwoot_configs DROP CONSTRAINT chatwoot_configs_pkey;
ALTER TABLE chatwoot_configs ADD PRIMARY KEY (instance_id);
ALTER TABLE chatwoot_configs DROP COLUMN id;
ALTER TABLE chatwoot_configs RENAME COLUMN is_enabled TO enabled;
ALTER TABLE chatwoot_configs RENAME COLUMN inbox_name TO name_inbox;
ALTER TABLE chatwoot_configs RENAME COLUMN is_sign_enabled TO sign_msg;
ALTER TABLE chatwoot_configs RENAME COLUMN is_reopen_enabled TO reopen_conversation;
ALTER TABLE chatwoot_configs RENAME COLUMN is_pending_enabled TO conversation_pending;
ALTER TABLE chatwoot_configs RENAME COLUMN is_merge_enabled TO merge_brazil_contacts;
ALTER TABLE chatwoot_configs RENAME COLUMN is_import_contacts TO import_contacts;
ALTER TABLE chatwoot_configs RENAME COLUMN is_import_messages TO import_messages;
ALTER TABLE chatwoot_configs RENAME COLUMN import_days TO days_limit;
ALTER TABLE chatwoot_configs RENAME COLUMN is_auto_create TO auto_create;
ALTER TABLE chatwoot_configs RENAME COLUMN ignored_jids TO ignore_jids;

ALTER TABLE jid_cache DROP CONSTRAINT jid_cache_phone_key;
ALTER TABLE jid_cache DROP CONSTRAINT jid_cache_pkey;
ALTER TABLE jid_cache ADD PRIMARY KEY (phone);
ALTER TABLE jid_cache DROP COLUMN id;
ALTER TABLE jid_cache DROP COLUMN updated_at;
ALTER TABLE jid_cache_expires_idx RENAME TO contacts_expires_idx;
ALTER TABLE jid_cache RENAME TO contacts;

ALTER TABLE media DROP CONSTRAINT media_direction_check;
DROP INDEX media_wa_id_idx;
DROP INDEX media_instance_idx;
DROP INDEX media_expires_idx;
ALTER TABLE media DROP CONSTRAINT media_bucket_object_key_key;
ALTER TABLE media ALTER COLUMN id DROP DEFAULT;
ALTER TABLE media DROP COLUMN bucket;
ALTER TABLE media DROP COLUMN object_deleted_at;
ALTER TABLE media DROP COLUMN updated_at;
ALTER TABLE media RENAME COLUMN wa_id TO message_id;
ALTER TABLE media RENAME COLUMN mime_type TO mimetype;
ALTER TABLE media RENAME COLUMN file_name TO filename;
ALTER TABLE media RENAME COLUMN object_key TO storage_path;
CREATE INDEX media_expires_idx ON media (expires_at);

DROP INDEX message_queue_next_attempt_idx;
DROP INDEX message_queue_media_idx;
DROP INDEX message_queue_wa_id_idx;
DROP INDEX message_queue_instance_status_idx;
ALTER TABLE message_queue DROP CONSTRAINT message_queue_media_id_fkey;
ALTER TABLE message_queue DROP CONSTRAINT message_queue_send_status_check;
ALTER TABLE message_queue ALTER COLUMN id DROP DEFAULT;
ALTER TABLE message_queue RENAME COLUMN recipient_jid TO recipient;
ALTER TABLE message_queue RENAME COLUMN message_type TO type;
ALTER TABLE message_queue RENAME COLUMN send_status TO status;
ALTER TABLE message_queue RENAME COLUMN retry_count TO retries;
ALTER TABLE message_queue RENAME COLUMN wa_id TO whatsapp_id;
ALTER TABLE message_queue RENAME COLUMN last_error_message TO last_error;
ALTER TABLE message_queue DROP COLUMN last_error_code;
ALTER TABLE message_queue DROP COLUMN last_error_at;
ALTER TABLE message_queue ALTER COLUMN status SET DEFAULT 'queued';
CREATE INDEX message_queue_instance_status_idx ON message_queue (instance_id, status);
CREATE INDEX message_queue_wa_id_idx ON message_queue (whatsapp_id);

ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ALTER COLUMN id DROP DEFAULT;
ALTER TABLE users RENAME COLUMN instance_limit TO instance_quota;

DROP INDEX instances_name_idx;
DROP INDEX instances_owner_idx;
ALTER TABLE instances DROP CONSTRAINT instances_owner_user_id_fkey;
ALTER TABLE instances ADD CONSTRAINT instances_owner_user_id_fkey
  FOREIGN KEY (owner_user_id) REFERENCES users(id);
ALTER TABLE instances ALTER COLUMN id DROP DEFAULT;

ALTER TABLE instances
  ADD COLUMN status text NOT NULL DEFAULT 'disconnected',
  ADD COLUMN whatsapp_jid text,
  ADD COLUMN device_jid text,
  ADD COLUMN last_connected_at timestamptz,
  ADD COLUMN last_error text,
  ADD COLUMN webhook_url text,
  ADD COLUMN webhook_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN webhook_events text[] NOT NULL DEFAULT '{message,receipt,connection,message.status}';

UPDATE instances i
SET status = c.status,
    device_jid = c.device_jid,
    whatsapp_jid = c.device_jid,
    last_connected_at = c.last_connected_at,
    last_error = c.last_error_message
FROM instance_connections c
WHERE c.instance_id = i.id;

UPDATE instances i
SET webhook_url = w.url,
    webhook_enabled = w.is_enabled,
    webhook_events = w.events
FROM instance_webhooks w
WHERE w.instance_id = i.id;

CREATE UNIQUE INDEX instances_device_jid_uidx ON instances (device_jid)
  WHERE device_jid IS NOT NULL AND device_jid <> '';

DROP TABLE instance_webhooks;
DROP TABLE instance_connections;
DROP TABLE remodel_report;
