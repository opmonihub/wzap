-- +goose Up
-- whatsmeow device identity (linked-device JID) bound 1:1 to an instance.
ALTER TABLE instances ADD COLUMN device_jid text;
UPDATE instances
SET device_jid = whatsapp_jid
WHERE device_jid IS NULL AND whatsapp_jid IS NOT NULL AND whatsapp_jid <> '';
CREATE UNIQUE INDEX instances_device_jid_uidx ON instances (device_jid)
  WHERE device_jid IS NOT NULL AND device_jid <> '';

-- +goose Down
DROP INDEX IF EXISTS instances_device_jid_uidx;
ALTER TABLE instances DROP COLUMN IF EXISTS device_jid;
