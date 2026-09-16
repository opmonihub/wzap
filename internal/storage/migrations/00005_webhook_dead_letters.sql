-- +goose Up
CREATE TABLE webhook_dead_letters (
  id bigserial PRIMARY KEY,
  instance_id uuid NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  event_id uuid NOT NULL UNIQUE,
  event_type text NOT NULL DEFAULT '',
  payload jsonb NOT NULL DEFAULT '{}',
  attempts int NOT NULL DEFAULT 0,
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webhook_dead_letters_instance_idx ON webhook_dead_letters (instance_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE webhook_dead_letters;
