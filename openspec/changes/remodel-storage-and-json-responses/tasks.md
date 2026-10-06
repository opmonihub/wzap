## 1. Fechar condições de execução

- [x] 1.1 Fechar os cinco bloqueios do design, preenchendo tipos, nullabilidade, defaults, enums, índices, exclusões, rollback, vínculos legados, correlação pré-WA ID, catálogo de erros, matriz completa por rota e origem verificável da imagem exata; verificar que nenhuma decisão material permanece aberta.

  Registro (1.1): a tag MinIO aprovada foi substituída por `docker.io/cccs/minio:latest` por ruling do controller (quay 401 em todas as tags); o spec `wzap-media` ainda cita a tag quay e será reconciliado na sincronização de specs. Detalhes em design.md §"Decisões fechadas na task 1.1" e `.superpowers/sdd/plan/task-1.1-report.md`.
- [x] 1.2 Coordenar a baseline e a migração 00007 com a change de origem, preparar worktree em `.worktrees/` e preflight com backups e fixtures; verificar que alterações locais alheias não entram no diff e que os bancos de teste são isolados.

  Registro (1.2): preflight em `preflight.md` (schema 12 próprias + 17 whatsmeow + goose, 00007 registrada em `goose_db_version`, contagens por tabela, política de backup descartável + `pg_dump` local em `backups/` não commitado). Fixtures em `internal/storage/postgres/postgrestest/fixtures.go` (`SeedRemodelFixtures` cobre device_jid presente/ausente/legado/divergente, media_id órfão, marcadores `chatwoot-*`, `pending:{uuid}`, idempotência completed/in_progress, media expirada, dead letter, outbox publicado/pendente). `go build`/`vet`/`test` do pacote limpos; integração real exige `WZAP_TEST_DATABASE_URL` (skipped sem ela). Detalhes em `.superpowers/sdd/plan/task-1.2-report.md`.

## 2. Remodelar persistência

- [x] 2.1 Implementar e ensaiar expansão, backfill e constraints das 14 tabelas; verificar upgrade e banco vazio em PostgreSQL com preservação de UUIDs, unicidade, timestamps, owners e dados recuperáveis.

  Registro (2.1): migração única `00008_remodel.sql` (expandir+renomear+backfill+constraints num só corte, permitido pelo brief) + tabela de auditoria `remodel_report` (categorias `device_jid_conflicts`, `legacy_connection_errors`, `orphan_media_refs`, `media_chatwoot_markers`). Testes novos em `internal/storage/postgres/migrate_remodel_test.go` verdes contra Postgres real em `127.0.0.1:5435` (`TestMigrateRemodelFresh`, `TestMigrateRemodelUpgrade`, `TestMigrateRemodelDown`, `TestMigrate` atualizado para os nomes finais). Esperado e conhecido: ~90 testes de repositório quebram porque usam os nomes antigos de colunas — correção é escopo de 2.2/2.3/2.4. Down não restaura o bigint de `webhook_dead_letters.id` (documentado no SQL). Detalhes em `.superpowers/sdd/plan/task-2.1-report.md`.
- [x] 2.2 Separar repositórios e comandos de identidade, conexão e webhook; verificar reconexão pelo dispositivo correto, exclusividade e edição concorrente sem regressão do estado.

  Registro (2.2): `model.Instance` virou agregado (`Connection`/`Webhook` satélites + `InstanceError` estruturado); `InstanceRepository.Update` deu lugar a `UpdateIdentity`, `SetConnection`, `SetConnectionState` e `SetWebhook` — writes por preocupação matam o lost-update (PATCH de nome concorrente com transição de sessão não regredia status; regressão coberta por `TestInstanceRepositoryUpdateIdentityDoesNotTouchConnection`). Reconexão mantém o bind por `device_jid` exclusivo (`instance_connections_device_jid_uidx` → `ErrDeviceJIDTaken`); leituras dão LEFT JOIN nos três quadros. HTTP segue flat (corte JSON é task 5.x); `event_version: 1` intacto. Gate `go test ./internal/storage/postgres ./internal/instance ./internal/session/... ./internal/app ./internal/webhook` verde contra Postgres em `127.0.0.1:5435` + suite completa `go test ./...` verde. Divergências do plano: outbox virou pending-only (`MarkPublished` deleta a linha; `DeletePublishedBefore` é no-op até o relay ser reworkado na 4.1) e `instances.owner_user_id` é `ON DELETE SET NULL` (teste de delete de owner atualizado; guard de CountByOwner continua no service). Detalhes em `.superpowers/sdd/plan/task-2.2-report.md`.
- [x] 2.3 Atualizar usuários, cache JID, metadados, configurações Chatwoot e dead letters para os nomes aprovados; verificar quotas, lookup global, unicidade por instância e limite de 500 falhas por instância.

  Registro (2.3): implementado junto com a 2.2 (o brief mandava consertar todos os testes de repo quebrados pelos renomes). `users.go` em `instance_limit` (quota preservada, `TestUserRepositoryUpdateQuota`/`Count` verdes), `jidcache.go` em `jid_cache` (put/get/overwrite/expires verdes), `newsletter_metadata.go` em `channel_metadata` (round-trip verde), `chatwoot.go` com flags `is_*`, `cw_id`, `chat_jid`, `ignored_jids` (round-trip + tiebreak determinístico + token sealed verdes), `deadletters.go` com `envelope`/`attempt_count`/trio `delivery_failed` e aparo transacional a `deadLetterRetentionPerInstance = 500` — coberto por `TestDeadLetterRetentionCapsPerInstance` (novo, 2.5s contra Postgres real): 503 inserções viram exatamente 500, as mais antigas saem e outras instâncias ficam intactas. Toda a bateria `TestUserRepository|TestJIDCache|TestChatwoot|TestGroupNewsletter|TestDeadLetter` verde contra `127.0.0.1:5435`.
- [ ] 2.4 Implementar FKs opcionais de mídia e correlação Chatwoot com isolamento por instância; verificar compartilhamento de mídia, múltiplos envios por cw_id e entradas sem fila artificial.

## 3. Migrar mídia para MinIO

- [ ] 3.1 Integrar cliente S3, ambiente e Compose com a imagem exata e rede existente; verificar inicialização e armazenamento contra a imagem comprovada sem credenciais fixas.
- [ ] 3.2 Adaptar upload, download, envio e mirror para objetos e temporários; verificar integridade, autorização e cleanup em sucesso, falha e cancelamento.
- [ ] 3.3 Implementar transferência verificável dos arquivos e expiração dos objetos; verificar preservação de IDs, SHA-256, tamanho, metadados e FKs, exclusão idempotente e rollback ensaiado.
- [ ] 3.4 Separar marcadores locais Chatwoot de wa_id reais conforme o relatório de migração; verificar que leitura, reply e revoke não usam marcadores sintéticos como IDs WhatsApp.

## 4. Preservar durabilidade e replay

- [ ] 4.1 Alterar o relay para remover somente eventos confirmados pelo JetStream; verificar indisponibilidade, reinício e falha de exclusão após PubAck com o mesmo UUID.
- [ ] 4.2 Capturar os erros estruturados nas quatro tabelas; verificar last_error público nulo ou estruturado, legado sem data inventada e fixtures de eventos v1 inalteradas.
- [ ] 4.3 Registrar os UUIDs de saídas Chatwoot conforme a política fechada em 1.1; verificar janela pré-WA ID, múltiplos anexos, falhas parciais e deduplicação de mirror/import.
- [ ] 4.4 Converter respostas legadas de idempotência sem repetir efeitos; verificar autorização, fingerprint, status HTTP, UUID, chave entregue uma vez e política segura para corpos não conversíveis.

## 5. Atualizar contrato HTTP e consumidores

- [ ] 5.1 Implementar DTOs e mapeadores de todas as operações da matriz fechada; verificar envelopes, coleções vazias, ocultação de campos, comandos e exceções com fixtures por rota.
- [ ] 5.2 Atualizar tipos e fluxos do Manager, incluindo instances_used e flags Chatwoot; verificar quotas, formulários sem apagar campos ocultos, opções de webhook e executar testes, typecheck e build.
- [ ] 5.3 Atualizar anotações dos handlers e regenerar Swagger; verificar cobertura de rotas, exemplos e segurança global sem campo apikey duplicado.

## 6. Verificar e concluir a implementação futura

- [ ] 6.1 Fazer revisão independente e executar os checks Go, Manager, PostgreSQL, NATS e MinIO previstos no plano; registrar os ambientes realmente executados e corrigir falhas antes do corte.
- [ ] 6.2 Ensaiar corte e recuperação com backups, produzir verify.md e retrospective.md e concluir o fluxo de integração; verificar evidências, marcar somente tarefas comprovadas e arquivar a change por último.
