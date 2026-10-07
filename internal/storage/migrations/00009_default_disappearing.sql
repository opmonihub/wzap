-- +goose Up
-- Persisted echo of the default disappearing timer
-- (PUT /instances/{id}/chats/default-disappearing), surfaced as
-- settings.default_disappearing in the instance DTO.
--
-- Placement: a small dedicated satellite (like instance_webhooks) instead of a
-- nullable column on instances/instance_connections. The identity row stays
-- identity-only and instance_connections stays lifecycle-only (written by the
-- connection commands); this echo has its own concern and its own single
-- writer, matching the per-concern write split. default_disappearing_seconds
-- is NULL until the first successful write (never configured) and stores 0 for
-- the explicit off value.
CREATE TABLE instance_chat_settings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id uuid NOT NULL UNIQUE REFERENCES instances(id) ON DELETE CASCADE,
  default_disappearing_seconds bigint,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS instance_chat_settings;
