-- +goose Up
-- Cutover gate for divergent device identities. The 00008 remodel copies
-- instances.whatsapp_jid / instances.device_jid into
-- instance_connections.device_jid preferring device_jid (the 00007 rule) and
-- records every divergence in remodel_report (category 'device_jid_conflicts')
-- with both values. The approved design ("JIDs divergentes") requires those
-- divergences to enter the report AND block the cutover until explicit
-- resolution — never silently choosing a side. 00008 only ever recorded them;
-- this migration is the blocking half: it fails while any conflict row is
-- unresolved, so `wzap migrate` aborts and a boot with WZAP_AUTO_MIGRATE=true
-- aborts, leaving the database at version 9 until an operator reconciles the
-- identity.
--
-- Operator resolution loop (explicit and auditable; the report rows remain
-- as audit history, only the gate condition lifts):
--   1. Read the evidence:
--        SELECT ref_id, detail, created_at FROM remodel_report
--        WHERE category = 'device_jid_conflicts' AND resolved_at IS NULL;
--      detail carries the name and both candidate JIDs recorded by 00008.
--   2. Reconcile the identity when the preferred device_jid is the wrong
--      side (the value 00008 chose lives in instance_connections.device_jid;
--      the other candidate is detail->>'whatsapp_jid'):
--        UPDATE instance_connections SET device_jid = '<decided>'
--          WHERE instance_id = '<ref_id>';
--      Re-pairing the instance is the alternative when neither candidate is
--      trustworthy.
--   3. Record the resolution:
--        UPDATE remodel_report SET resolved_at = now()
--          WHERE category = 'device_jid_conflicts' AND ref_id = '<ref_id>';
--   4. Rerun `wzap migrate`. The gate passes only once every conflict row
--      carries resolved_at.
--
-- Other remodel_report categories (orphan_media_refs, media_chatwoot_markers,
-- legacy_connection_errors) are evidence-only by design and do not gate the
-- cutover.

-- goose splits statements on semicolons without dollar-quote awareness, so
-- the procedural DO block is fenced with StatementBegin/StatementEnd.

-- +goose StatementBegin
DO $$
DECLARE
  conflicts text;
BEGIN
  SELECT string_agg(
           coalesce(detail->>'name', '(unnamed)') || ' [' || ref_id || ']'
             || ' whatsapp_jid=' || coalesce(detail->>'whatsapp_jid', '<null>')
             || ' device_jid=' || coalesce(detail->>'device_jid', '<null>'),
           E'\n  ' ORDER BY ref_id)
    INTO conflicts
    FROM remodel_report
   WHERE category = 'device_jid_conflicts'
     AND resolved_at IS NULL;
  IF conflicts IS NOT NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'device JID divergence blocks the cutover; unresolved device_jid_conflicts in remodel_report:' || E'\n  ' || conflicts,
      HINT = 'reconcile instance_connections.device_jid for each listed instance, mark its remodel_report row resolved (resolved_at = now()) and rerun wzap migrate';
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- The gate is a check, not a schema change: there is nothing to reverse. The
-- resolved_at column it reads is dropped by 00009's Down.