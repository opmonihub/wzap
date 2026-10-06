-- rehearsal-inventory.sql — inventário somente leitura para o ensaio de corte/
-- recuperação da change remodel-storage-and-json-responses (task 6.2).
--
-- Uso: psql -X -d <clone> -v ON_ERROR_STOP=1 -f rehearsal-inventory.sql
-- O script é tolerante à versão do schema: funciona no clone pré-corte (nomes
-- antigos) e no clone pós-corte (nomes remodelados), usando SQL dinâmico para
-- as colunas/tabelas renomeadas. Nenhuma instrução de escrita em dados de
-- usuário — só tabelas temporárias de trabalho.

\pset pager off
\echo '=== INVENTORY BEGIN ==='

\echo '--- goose version ---'
SELECT version_id, is_applied, tstamp FROM goose_db_version ORDER BY version_id, tstamp;

-- Contagens por tabela (só tabelas existentes na versão do clone).
CREATE TEMP TABLE inv_counts (tbl text PRIMARY KEY, n bigint);
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'instances','users','message_queue','idempotency_keys','contacts','jid_cache','media',
    'event_outbox','chatwoot_configs','chatwoot_messages','webhook_dead_letters',
    'group_metadata','channel_metadata','newsletter_metadata',
    'instance_connections','instance_webhooks','remodel_report']
  LOOP
    IF to_regclass('public.' || t) IS NOT NULL THEN
      EXECUTE format('INSERT INTO inv_counts SELECT %L, count(*) FROM %I', t, t);
    END IF;
  END LOOP;
END $$;
\echo '--- own tables: row counts ---'
SELECT tbl, n FROM inv_counts ORDER BY tbl;

-- Digestos de conjuntos de IDs (md5 da lista ordenada; a comparação pré/pós
-- corte prova que nenhum UUID se perdeu nem nasceu).
CREATE TEMP TABLE inv_ids (tbl text PRIMARY KEY, ids_md5 text);
DO $$
DECLARE t text; col text;
BEGIN
  FOREACH t IN ARRAY ARRAY['instances','users','message_queue','media','event_outbox'] LOOP
    IF to_regclass('public.' || t) IS NOT NULL THEN
      EXECUTE format(
        'INSERT INTO inv_ids SELECT %L, md5(string_agg(id::text, '','' ORDER BY id)) FROM %I', t, t);
    END IF;
  END LOOP;
  IF to_regclass('public.webhook_dead_letters') IS NOT NULL THEN
    EXECUTE 'INSERT INTO inv_ids SELECT ''webhook_dead_letters(event_id)'', '
            'md5(string_agg(event_id::text, '','' ORDER BY event_id)) FROM webhook_dead_letters';
  END IF;
  IF to_regclass('public.idempotency_keys') IS NOT NULL THEN
    col := NULL;
    SELECT c.column_name INTO col FROM information_schema.columns c
    WHERE c.table_schema = 'public' AND c.table_name = 'idempotency_keys'
      AND c.column_name IN ('key','idempotency_key') LIMIT 1;
    EXECUTE format(
      'INSERT INTO inv_ids SELECT %L, md5(string_agg(instance_id::text || '':'' || %I::text, '','' '
      'ORDER BY instance_id, %I)) FROM idempotency_keys',
      'idempotency_keys(' || col || ')', col, col);
  END IF;
  IF to_regclass('public.chatwoot_messages') IS NOT NULL THEN
    col := NULL;
    SELECT c.column_name INTO col FROM information_schema.columns c
    WHERE c.table_schema = 'public' AND c.table_name = 'chatwoot_messages'
      AND c.column_name IN ('cw_id','chatwoot_message_id') LIMIT 1;
    EXECUTE format(
      'INSERT INTO inv_ids SELECT %L, md5(string_agg(instance_id::text || '':'' || %I::text, '','' '
      'ORDER BY instance_id, %I)) FROM chatwoot_messages',
      'chatwoot_messages(' || col || ')', col, col);
  END IF;
END $$;
\echo '--- id set digests ---'
SELECT tbl, ids_md5 FROM inv_ids ORDER BY tbl;

\echo '--- owners (instance -> owner_user_id) ---'
SELECT id AS instance_id, owner_user_id FROM instances ORDER BY id;

-- Quota por usuário: coluna renomeada (instance_quota -> instance_limit).
CREATE TEMP TABLE inv_quotas (user_id uuid, quota bigint);
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_schema = 'public' AND table_name = 'users'
               AND column_name = 'instance_limit') THEN
    INSERT INTO inv_quotas SELECT id, instance_limit FROM users;
  ELSE
    INSERT INTO inv_quotas SELECT id, instance_quota FROM users;
  END IF;
END $$;
\echo '--- quotas ---'
SELECT user_id, quota FROM inv_quotas ORDER BY user_id;

-- Metadados de mídia: colunas renomeadas (message_id/mimetype/storage_path ->
-- wa_id/mime_type/object_key).
CREATE TEMP TABLE inv_media (
  id uuid, instance_id uuid, size_bytes bigint, sha256 text,
  object_key text, deleted_at timestamptz);
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_schema = 'public' AND table_name = 'media'
               AND column_name = 'object_key') THEN
    INSERT INTO inv_media
      SELECT id, instance_id, size_bytes, sha256, object_key, object_deleted_at FROM media;
  ELSE
    INSERT INTO inv_media
      SELECT id, instance_id, size_bytes, sha256, storage_path, NULL FROM media;
  END IF;
END $$;
\echo '--- media metadata (id, instance_id, size, sha256, key) ---'
SELECT id, instance_id, size_bytes, sha256, object_key FROM inv_media ORDER BY id;

\echo '--- idempotency keys (status mix) ---'
SELECT status, count(*),
       count(*) FILTER (WHERE response_body IS NOT NULL) AS with_response
FROM idempotency_keys GROUP BY status ORDER BY status;

\echo '--- constraints of own tables ---'
SELECT c.conrelid::regclass::text AS tbl, c.conname, pg_get_constraintdef(c.oid) AS def
FROM pg_constraint c
WHERE c.connamespace = 'public'::regnamespace
  AND c.conrelid::regclass::text IN
  ('instances','users','message_queue','idempotency_keys','contacts','jid_cache','media',
   'event_outbox','chatwoot_configs','chatwoot_messages','webhook_dead_letters',
   'group_metadata','channel_metadata','newsletter_metadata',
   'instance_connections','instance_webhooks')
ORDER BY 1, 2;

\echo '--- indexes of own tables ---'
SELECT tablename, indexname
FROM pg_indexes
WHERE schemaname = 'public'
  AND tablename IN
  ('instances','users','message_queue','idempotency_keys','contacts','jid_cache','media',
   'event_outbox','chatwoot_configs','chatwoot_messages','webhook_dead_letters',
   'group_metadata','channel_metadata','newsletter_metadata',
   'instance_connections','instance_webhooks')
ORDER BY 1, 2;

\echo '--- total tables in public ---'
SELECT count(*) AS table_count FROM pg_tables WHERE schemaname = 'public';
\echo '=== INVENTORY END ==='
