-- +goose Up
CREATE TABLE IF NOT EXISTS group_metadata (
  instance_id uuid NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  group_jid text NOT NULL,
  name text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  participant_count int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (instance_id, group_jid)
);
CREATE TABLE IF NOT EXISTS newsletter_metadata (
  instance_id uuid NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  channel_jid text NOT NULL,
  title text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  follower_count int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (instance_id, channel_jid)
);

-- +goose Down
DROP TABLE IF EXISTS newsletter_metadata;
DROP TABLE IF EXISTS group_metadata;
