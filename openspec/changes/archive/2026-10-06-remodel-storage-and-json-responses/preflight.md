# Preflight — remodel-storage-and-json-responses

Capturado em 2026-10-06 no ambiente dev local (`docker compose`, container `wzap-postgres-1`, banco `wzap`). Branch `codex/remodel-storage-and-json-responses` em `.worktrees/remodel-storage-and-json-responses`, base `main c716b01`.

## Coordenação da baseline e da migração 00007

- `internal/storage/migrations/00007_instance_device_jid.sql` está commitada na base (`c716b01`, veio da change de nomes de instância): `ALTER TABLE instances ADD COLUMN device_jid`, backfill `whatsapp_jid → device_jid`, `instances_device_jid_uidx` (UNIQUE parcial, `device_jid IS NOT NULL AND <> ''`).
- `goose_db_version` registra versões 0–7 aplicadas; `version_id=7` aplicado em 2026-10-04 03:16:22 — **00007 está aplicada e registrada** no banco dev.
- Índice `instances_device_jid_uidx` presente e ativo; no remodel ele sai de `instances` e migra para `instance_connections` (design §2.2) — a migration 00007 é o ponto de partida, não um conflito.
- `git status` do worktree limpo antes do preflight: nenhuma alteração alheia em `internal/`, `docs/`, `manager/` entra no diff desta change.

## Inventário do schema atual (baseline)

30 tabelas em `public`: **12 próprias**, **17 whatsmeow**, **1 goose**.

### Tabelas próprias (rows no banco dev)

| tabela | rows |
|---|---|
| `instances` | 2 |
| `users` | 1 |
| `message_queue` | 3 |
| `idempotency_keys` | 3 |
| `contacts` | 1 |
| `media` | 0 |
| `event_outbox` | 305 |
| `chatwoot_configs` | 0 |
| `chatwoot_messages` | 0 |
| `webhook_dead_letters` | 0 |
| `group_metadata` | 1 |
| `newsletter_metadata` | 1 |

### Tabelas whatsmeow (intocáveis pelo remodel)

`whatsmeow_app_state_mutation_macs`, `whatsmeow_app_state_sync_keys`, `whatsmeow_app_state_version`, `whatsmeow_chat_settings`, `whatsmeow_contacts`, `whatsmeow_device`, `whatsmeow_event_buffer`, `whatsmeow_identity_keys`, `whatsmeow_lid_map`, `whatsmeow_message_secrets`, `whatsmeow_nct_salt`, `whatsmeow_pre_keys`, `whatsmeow_privacy_tokens`, `whatsmeow_retry_buffer`, `whatsmeow_sender_keys`, `whatsmeow_sessions`, `whatsmeow_version`.

### Tabela de controle

`goose_db_version` — fora do escopo de remodel (registrar apenas `INSERT` das novas versões via goose).

## Estado relevante do conteúdo (sinais para o design)

- `instances`: 1 de 2 com `device_jid` preenchido (e igual a `whatsapp_jid`); a outra sem JIDs (nunca pareada). Nenhuma divergência `whatsapp_jid ≠ device_jid` — `device_jid_conflicts` vazio na prática, mas a fixture cobre o caso.
- `media`: 0 rows — o caminho `media_id` órfão e os marcadores `chatwoot-*` em `media.message_id` só serão exercitados via fixtures de upgrade.
- `chatwoot_messages`: 0 rows — janela pré-WA ID (`pending:{uuid}`) e backfill com evidência cobertos por fixtures.
- `event_outbox`: 305 rows — inclui eventos já `published_at` não nulos (legado da política "manter confirmados"; a política nova remove confirmados — design §2.13/etapa 4.1).
- `webhook_dead_letters`: 0 rows.

## Política de backup antes do DDL

**Decisão**: dump completo em `pg_dump` por precaução, mas o banco dev é tratado como **descartável**.

- Backup: `openspec/changes/remodel-storage-and-json-responses/backups/wzap-dev-20261006.sql.gz` (pg_dump completo de `wzap`, ~3,9 MB compactado). O arquivo é **local-only**: contém dados operacionais dev (hashes de api_key, tokens, payload de outbox) e **não é commitado** — manter como artefato de rollback até a etapa 6.2 (ensaiar corte/recuperação), depois pode ser descartado junto com o worktree.
- Justificativa de descartabilidade: os dados dev são fixtures acidentais (2 instâncias de laboratório, 305 eventos de outbox já obsoletos); a reprodução exata da baseline vem das fixtures determinísticas de `postgrestest.SeedRemodelFixtures`, não do estado dev. Em perda total, `wzap migrate` recria o schema e os testes de upgrade validam a migração contra dados representativos isolados por schema.
- **Proibição**: nenhum DDL do remodel pode rodar em `public` do banco dev antes de um novo `pg_dump` datado imediatamente anterior ao corte real; os testes de upgrade rodam exclusivamente em schemas `wzap_test_*` isolados do banco `wzap_test` (`postgrestest.NewPool`), que exige sufixo `_test` e falha em qualquer outro banco.

## Isolamento dos bancos de teste

- `WZAP_TEST_DATABASE_URL` aponta para `wzap_test` (sufixo obrigatório); `postgrestest.NewPool` cria `CREATE SCHEMA wzap_test_<hex>` por teste com `search_path` dedicado e `DROP SCHEMA ... CASCADE` no cleanup — testes paralelos não colidem e nunca tocam `public` nem o banco dev.
- `wzap_test` existe no container (`createdb` já executado). Sem a env var, os testes de integração são **skipped** (não executados) — esta sessão não rodou integração.

## Fixtures representativas

`internal/storage/postgres/postgrestest/fixtures.go` expõe `SeedRemodelFixtures(t, pool)` que popula o schema isolado com as situações exigidas pelo design/etapa 2:

- instância **sem** `device_jid`/`whatsapp_jid` (nunca pareada);
- instância **com** `device_jid` e `whatsapp_jid` alinhados;
- instância legada **só** com `whatsapp_jid` (backfill 00007 pendente de drift) e instância com `whatsapp_jid` **divergente** de `device_jid` (caso `device_jid_conflicts`);
- `message_queue` com e sem `media_id`, incluindo **mídia órfã** (`media_id` apontando para id inexistente — sem FK ainda, representa o caso `orphan_media_refs`);
- `media` expirada e não expirada, com `message_id` WA real e com marcador `chatwoot-{id}-{idx}` (caso `media_chatwoot_markers`);
- `chatwoot_messages` com `wa_key` real correlacionável a `message_queue.whatsapp_id` (backfill com evidência), com `wa_key` de entrada sem fila (`message_id` permanece NULL) e marcador `pending:{uuid}`;
- `webhook_dead_letters` com `event_id` único;
- `idempotency_keys` em `completed` (com `response_body`) e `in_progress`;
- `event_outbox` publicado (`published_at` preenchido) e pendente;
- `contacts`/`jid_cache` válido;
- `users` admin + user com `instance_quota`, instância com `owner_user_id` e instância órfã de dono (`NULL`).

A função retorna os IDs semeados em uma struct `RemodelFixtures` para asserts dos testes de upgrade. Compilação validada com `go build ./internal/storage/postgres/postgrestest`; a execução real requer `WZAP_TEST_DATABASE_URL` (skipped sem ela).
