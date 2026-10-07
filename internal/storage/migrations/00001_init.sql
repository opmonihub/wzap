-- +goose Up
-- Fresh-system baseline: installs the CURRENT catalog (15 tables) in one
-- migration. Derived from the final state of the historical chain it
-- replaces (00001..00010, including the remodel); no backfill or remodel
-- machinery ships anymore. goose_db_version and the whatsmeow tables are
-- managed by their own tooling, not here.

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL,
  password_hash text NOT NULL,
  role text CHECK (role IN ('admin', 'user')),
  instance_limit int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
-- Case-insensitive uniqueness of the account address.
CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));

CREATE TABLE instances (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  -- Global case-sensitive uniqueness: `{id}` accepts a UUID or an exact name.
  external_ref text UNIQUE,
  -- Ownership is mandatory; deleting a user that still owns instances is
  -- blocked (restrictive FK), never cascaded or nulled.
  owner_user_id uuid NOT NULL REFERENCES users(id),
  api_key_hash text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT instances_name_key UNIQUE (name)
);
CREATE INDEX instances_owner_idx ON instances (owner_user_id);

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
-- device_jid is unique only when filled.
CREATE UNIQUE INDEX instance_connections_device_jid_uidx
  ON instance_connections (device_jid)
  WHERE device_jid IS NOT NULL AND device_jid <> '';

CREATE TABLE instance_webhooks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL UNIQUE REFERENCES instances(id) ON DELETE CASCADE,
  url text,
  is_enabled bool NOT NULL DEFAULT false,
  events text[] NOT NULL DEFAULT '{message,receipt,connection,message.status}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Persisted echo of the default disappearing timer
-- (PUT /instances/{id}/chats/default-disappearing). NULL until the first
-- successful write (never configured); 0 stores the explicit off value.
CREATE TABLE instance_chat_settings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL UNIQUE REFERENCES instances(id) ON DELETE CASCADE,
  default_disappearing_seconds bigint,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE media (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  direction text NOT NULL CHECK (direction IN ('inbound', 'outbound')),
  wa_id text,
  mime_type text NOT NULL,
  file_name text,
  size_bytes bigint NOT NULL,
  bucket text NOT NULL,
  object_key text NOT NULL,
  sha256 text NOT NULL,
  object_deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  CONSTRAINT media_bucket_object_key_key UNIQUE (bucket, object_key)
);
CREATE INDEX media_expires_idx ON media (expires_at) WHERE object_deleted_at IS NULL;
CREATE INDEX media_instance_idx ON media (instance_id);
CREATE INDEX media_wa_id_idx ON media (wa_id) WHERE wa_id IS NOT NULL;

CREATE TABLE message_queue (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  recipient_jid text NOT NULL,
  message_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}',
  send_status text NOT NULL DEFAULT 'queued'
    CHECK (send_status IN ('queued', 'sending', 'sent', 'failed')),
  retry_count int NOT NULL DEFAULT 0,
  last_error_code text,
  last_error_message text,
  last_error_at timestamptz,
  wa_id text,
  delivered_at timestamptz,
  media_id uuid REFERENCES media(id) ON DELETE SET NULL,
  next_attempt_at timestamptz,
  read_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX message_queue_instance_status_idx ON message_queue (instance_id, send_status);
CREATE INDEX message_queue_created_idx ON message_queue (created_at);
CREATE INDEX message_queue_wa_id_idx ON message_queue (wa_id);
CREATE INDEX message_queue_media_idx ON message_queue (media_id) WHERE media_id IS NOT NULL;
CREATE INDEX message_queue_next_attempt_idx ON message_queue (next_attempt_at)
  WHERE send_status = 'queued';

CREATE TABLE jid_cache (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  phone text NOT NULL,
  jid text NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT jid_cache_phone_key UNIQUE (phone)
);
CREATE INDEX jid_cache_expires_idx ON jid_cache (expires_at);

CREATE TABLE chatwoot_configs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL UNIQUE REFERENCES instances(id) ON DELETE CASCADE,
  is_enabled bool NOT NULL DEFAULT false,
  url text NOT NULL DEFAULT '',
  account_id text NOT NULL DEFAULT '',
  token text NOT NULL DEFAULT '',
  inbox_name text NOT NULL DEFAULT '',
  is_sign_enabled bool NOT NULL DEFAULT false,
  sign_delimiter text NOT NULL DEFAULT '',
  is_reopen_enabled bool NOT NULL DEFAULT true,
  is_pending_enabled bool NOT NULL DEFAULT false,
  is_merge_enabled bool NOT NULL DEFAULT false,
  is_import_contacts bool NOT NULL DEFAULT false,
  is_import_messages bool NOT NULL DEFAULT false,
  import_days int NOT NULL DEFAULT 0,
  is_auto_create bool NOT NULL DEFAULT false,
  organization text NOT NULL DEFAULT '',
  logo text NOT NULL DEFAULT '',
  ignored_jids text[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE chatwoot_messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  message_id uuid REFERENCES message_queue(id) ON DELETE SET NULL,
  wa_key text NOT NULL,
  cw_id bigint NOT NULL,
  conversation_id bigint NOT NULL,
  inbox_id bigint NOT NULL,
  chat_jid text NOT NULL DEFAULT '',
  is_read bool NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT chatwoot_messages_instance_wa_key_key UNIQUE (instance_id, wa_key)
);
CREATE INDEX chatwoot_messages_instance_msg_idx ON chatwoot_messages (instance_id, cw_id);
CREATE INDEX chatwoot_messages_conversation_idx
  ON chatwoot_messages (instance_id, conversation_id, created_at DESC, cw_id DESC);
CREATE INDEX chatwoot_messages_message_id_idx ON chatwoot_messages (message_id)
  WHERE message_id IS NOT NULL;

CREATE TABLE group_metadata (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  group_jid text NOT NULL,
  name text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  participant_count int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT group_metadata_instance_group_key UNIQUE (instance_id, group_jid)
);

CREATE TABLE channel_metadata (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  channel_jid text NOT NULL,
  title text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  follower_count int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT channel_metadata_instance_channel_key UNIQUE (instance_id, channel_jid)
);

CREATE TABLE idempotency_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  key text NOT NULL,
  request_hash text NOT NULL,
  status text NOT NULL CHECK (status IN ('in_progress', 'completed')),
  http_status int,
  response_body jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  CONSTRAINT idempotency_keys_instance_key_key UNIQUE (instance_id, key)
);
CREATE INDEX idempotency_keys_expires_idx ON idempotency_keys (expires_at);

CREATE TABLE event_outbox (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  subject text NOT NULL,
  envelope jsonb NOT NULL,
  attempt_count int NOT NULL DEFAULT 0,
  last_error_code text,
  last_error_message text,
  last_error_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX event_outbox_pending_idx ON event_outbox (created_at);

CREATE TABLE webhook_dead_letters (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  event_id uuid NOT NULL UNIQUE,
  event_type text NOT NULL DEFAULT '',
  envelope jsonb NOT NULL DEFAULT '{}',
  attempt_count int NOT NULL DEFAULT 0,
  last_error_code text,
  last_error_message text,
  last_error_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webhook_dead_letters_instance_idx
  ON webhook_dead_letters (instance_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE webhook_dead_letters;
DROP TABLE event_outbox;
DROP TABLE idempotency_keys;
DROP TABLE channel_metadata;
DROP TABLE group_metadata;
DROP TABLE chatwoot_messages;
DROP TABLE chatwoot_configs;
DROP TABLE jid_cache;
DROP TABLE message_queue;
DROP TABLE media;
DROP TABLE instance_chat_settings;
DROP TABLE instance_webhooks;
DROP TABLE instance_connections;
DROP TABLE instances;
DROP TABLE users;