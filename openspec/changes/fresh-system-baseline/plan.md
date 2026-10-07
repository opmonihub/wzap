# Implementation Plan — fresh-system-baseline

Branch `codex/fresh-system-baseline`, worktree `.worktrees/fresh-system-baseline`,
base `10ba26a`, artefatos OpenSpec em `151d46a`. Execução via subagent-driven
development: um implementador por task, revisão por diff após cada task.

## Contexto

Instalações novas sobre banco vazio: substituir a cadeia histórica de migrações
por uma migração inicial única, remover caminhos exclusivos de compatibilidade
(backfills, conversores, tolerâncias) e preservar integralmente os fluxos atuais
do gateway (eventos v1, import Chatwoot, correlações `pending:{uuid}`, retries,
segurança, RBAC). Aplicação futura reseta volumes locais sem backup após gates.

## Regras globais

- Go 1.26, `gofmt`, `go vet`; golangci-lint v2.13.2 nos gates (6.1).
- Testes com Postgres real: `WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5432/wzap_test?sslmode=disable'`; NATS: `WZAP_TEST_NATS_URL='nats://127.0.0.1:4222'`. Nunca tocar schema compartilhado; nome de DB termina em `_test`.
- Commits convencionais (`feat(api):`, `fix(events):`, `docs(specs):`, etc.); marcar quebras de contrato **BREAKING**.
- Após editar anotações de handler: `go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -g internal/httpapi/swagger.go -o docs` e `git diff --exit-code -- docs/`.
- Um escritor por arquivo. Tasks paralelas apenas com arquivos disjuntos.
- Busca textual por `legacy` não basta para achar código morto: validar cada remoção por referências e comportamento.
- Tarefa 7.1 (remoção de volumes) exige confirmação explícita do usuário antes de executar.

## Ondas de execução

- **Onda 1 (schema/identidade):** 2.1 → 2.2 → 2.3 → 2.4 (sequencial; migrações, constraints, fixtures e nomes compartilham `internal/storage/`).
- **Onda 2 (boot/config/eventos):** 3.1 ∥ 3.2 (arquivos disjuntos: `cmd/wzap/seed*` vs `internal/storage/postgres/chatwoot*`); depois 3.3 ∥ 3.4 (`internal/config|logger|httpapi/server*` vs `internal/events|storage/postgres/events*`).
- **Onda 3 (REST/idempotência):** 4.1 → 4.2 → 4.3 → 4.4 (sequencial; `internal/httpapi/` e DTOs compartilhados).
- **Onda 4 (mídia/Manager/docs):** 5.1 → 5.2 (mídia/wiring sequencial); 5.3 (Manager) em paralelo com 5.2 (disjunto: `manager/`); 5.4 por último (Swagger/README dependem de todo o Go final).
- **Onda 5 (gates):** 6.1 → 6.2 → 6.3.
- **Onda 6 (reset local):** 7.1 (após confirmação do usuário) → 7.2 → 7.3 → 7.4 → 7.5.

## Task 2.1 — Migração inicial única

TDD:
1. Escrever teste que roda a cadeia em banco vazio e espera instalar as 15 tabelas (`users`, `instances`, `instance_connections`, `instance_webhooks`, `instance_chat_settings`, `message_queue`, `media`, `jid_cache`, `chatwoot_messages`, `chatwoot_configs`, `group_metadata`, `channel_metadata`, `idempotency_keys`, `event_outbox`, `webhook_dead_letters`) sem `remodel_report` — falha primeiro.
2. Criar `internal/storage/migrations/00001_init.sql` a partir do catálogo vigente (UUIDs/defaults, timestamps, enums, unicidades naturais, índices, FKs); remover os SQLs 00001–00010 e `embed.go` da cadeia antiga.
3. `instances.owner_user_id` NOT NULL com FK restritiva; `instances.name` único global case-sensitive.
4. Repetição segura e catálogo final verificados; sem scripts de backfill.

Verificação: `WZAP_TEST_DATABASE_URL=... go test ./internal/storage/postgres -run TestMigrate -count=1`.

## Task 2.2 — Ownership e unicidade de nomes

TDD:
1. Teste: criar instância sem dono válido falha; claim concorrente de nome mapeia para `instance_name_taken` (constraint do banco → erro 409); isolamento por satélite (identity/connection/webhook/chat_settings) mantido.
2. Ajustar repositórios para depender das constraints novas; remover tolerâncias de NULL owner.
3. Mapear violação de unicidade do banco para o erro de serviço atual.

Verificação: `go test ./internal/storage/postgres ./internal/storage -count=1` com Postgres `_test`.

## Task 2.3 — Fixtures e testes sem histórico

TDD:
1. Remover fixtures/testes cujo único objetivo é upgrade/conversão/tolerância histórica (ex.: cenários de `00008_remodel`, dados duplicados de nomes).
2. Adaptar fixtures válidas ao modelo novo (owner obrigatório, nome único) sem reduzir cobertura de segurança/concorrência.
3. Rodar suíte de storage completa.

Verificação: `go test ./internal/storage/... -count=1` com Postgres `_test`.

## Task 2.4 — Nomes sem ambiguidade

TDD:
1. Remover `ErrInstanceNameAmbiguous` (`internal/storage/repository.go:20`, `internal/instance/service.go:28`) e exceções de nomes históricos.
2. Preservar: UUID prioritário, nome exato, reserva de `stats`/UUID, renomeação 404 do alias antigo, quotas, autorização.
3. Testes de criação/rename/resolve por nome com banco de nomes único.

Verificação: `go test ./internal/instance ./internal/httpapi -count=1`.

## Task 3.1 — Seed mínimo

TDD:
1. Teste: seed cria somente o primeiro admin; com contas existentes, reinício não altera nada; ausência de dono válido impede persistência.
2. Remover `ownerBackfiller` (`cmd/wzap/seed.go:19`) e `BackfillOwner` (`internal/storage/postgres/instances.go:363`) e compensações de adoção; remover `instances_backfill_test.go`, fakes de seed (`cmd/wzap/seed_test.go:262`).

Verificação: `go test ./cmd/wzap ./internal/storage/postgres -count=1` com Postgres `_test`.

## Task 3.2 — Chatwoot cifrado obrigatório

TDD:
1. Teste: Chatwoot habilitado sem `WZAP_CHATWOOT_TOKEN_KEY` (ou chave != 32 bytes base64) falha a configuração; token não vazio sem chave válida é recusado; round-trip AES-256-GCM; ciphertext adulterado falha; config desligada sem token não exige chave; respostas públicas nunca expõem token.
2. Remover modo plaintext, `BackfillTokenSeal` (`internal/storage/postgres/chatwoot.go`, testes `chatwoot_test.go:431`) e warnings de backfill.
3. Wiring/testes de repositório passam a fornecer chave de teste válida.
4. Preservar import operacional e `pending:{uuid}`.

Verificação: `go test ./internal/storage/postgres ./internal/chatwoot/... -count=1` com Postgres `_test`.

## Task 3.3 — Log format e routing

TDD:
1. Teste: `WZAP_LOG_FORMAT` aceita só `json`/`console`; outro valor falha a configuração; handler dedicado `/api/v1/` removido (`internal/httpapi/server.go:68-71`), paths não registrados seguem routing/autorização genérica.
2. Remover alias `text` (`internal/logger/logger.go:43-47`) e referências a envs anteriores sem criar aliases novos.

Verificação: `go test ./internal/logger ./internal/config ./internal/httpapi -count=1`.

## Task 3.4 — Outbox pending-only

TDD:
1. Teste: confirmação de publicação remove o pendente; broker indisponível mantém retries; envelope v1 e `event_id` estáveis.
2. Remover `DeletePublishedBefore` (`internal/storage/repository.go:185`, `internal/storage/postgres/events.go:116`, `internal/events/relay.go:230`), scheduling/limpeza vestigial e fakes/testes correspondentes (`relay_test.go:121,429`, `events_test.go:296`).

Verificação: `go test ./internal/events ./internal/storage/postgres -count=1` + `WZAP_TEST_NATS_URL='nats://127.0.0.1:4222' go test ./internal/events/ -count=1`.

## Task 4.1 — Replay sem conversores

TDD:
1. Teste: replay devolve status HTTP e corpo originais do contrato atual, após autorização; TTL, fingerprint, concorrência, headers, liberação de chave para 4xx/503 e ausência de segundo efeito mantidos.
2. Remover `convertReplayBody` e respostas especiais (`internal/httpapi/idempotency.go:193,205,209`) e seus testes de conversão de envelopes.

Verificação: `go test ./internal/httpapi -run TestIdempotency -count=1` (ajustar padrão ao nome real).

## Task 4.2 — Erros estruturados sem legacy_error

TDD:
1. Teste: falhas de conexão, envio, outbox e dead letters gravam código/mensagem/instante reais; nenhum produtor grava `legacy_error` nem timestamp fabricado.
2. Remover `legacy_error` de DTOs e fixtures (`internal/httpapi/dto.go:6,17,270`, `internal/model/model.go:12`); `LastError` textual do erro estruturado vigente permanece para consumidores atuais.

Verificação: `go test ./internal/httpapi ./internal/message ./internal/app ./internal/storage/postgres -count=1`.

## Task 4.3 — Revogação sem campo reservado

TDD:
1. Teste: sucesso mantém `data.revoked=true`; falhas atuais preservadas; nenhum campo `reason` reservado.
2. Remover o campo `reason` da resposta de revogação e fixtures do contrato HTTP.

Verificação: `go test ./internal/httpapi ./internal/message -count=1`; regenerar `docs/` se anotações mudarem.

## Task 4.4 — Envelopes do contrato vigente

TDD:
1. Remover testes de envelopes históricos; preservar testes de privacidade (sem `external_ref`/`owner_user_id`/`device_jid`/tokens) e contratos atuais.

Verificação: `go test ./internal/httpapi ./internal/message ./internal/app ./internal/session/... -count=1`.

## Task 5.1 — Remover media-migrate

TDD:
1. Teste/verificação: comando `media-migrate` (`cmd/wzap/main.go`) não existe; `MigrateLocalFiles` (`internal/media/storage.go:401,434`) e `SetBucket` usado só pelo migrador (`storage.go:522`, `media_repo.go`) removidos sem consumidores remanescentes; fallback de bucket vazio removido.
2. Ajustar wiring de boot.

Verificação: `go test ./cmd/wzap ./internal/media ./internal/storage/postgres -count=1` + `rg` sem consumidores.

## Task 5.2 — Backends oficiais de mídia

TDD:
1. Teste: S3 configurado → bucket explícito do config; disco → bucket `local`; leitura/exclusão usam o bucket da row; falha de preparação do bucket S3 no boot interrompe o boot; falha posterior não troca para disco; checksum, download, TTL e cache preservados.
2. Remover default de bucket vazio; manter MinIO como padrão no compose.

Verificação: `go test ./internal/media ./internal/storage/postgres -count=1` (S3 via fake/minio dedicado na 6.2).

## Task 5.3 — Manager no contrato atual

1. Formulários/tipos/traduções/mensagens sem nomes históricos e erros antigos (`manager/app/types/api.ts` e consumos).
2. Falhas estruturadas consumidas do contrato vigente.

Verificação: `pnpm --dir manager test`, `lint`, `typecheck`, `build`.

## Task 5.4 — README, AGENTS e Swagger

1. Remover anotações/rotas de compatibilidade; atualizar README/instruções vigentes (media-migrate, envs removidos, reset sem backup).
2. Regenerar Swagger limpo.

Verificação: `go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -g internal/httpapi/swagger.go -o docs && git diff --exit-code -- docs/`.

## Task 6.1 — Gates Go

Executar e registrar: `gofmt` limpo, `go vet ./...`, golangci-lint v2.13.2, `WZAP_TEST_DATABASE_URL=... go test ./... -count=1`, `go build ./...`. Registrar comandos, versões e resultados reais.

## Task 6.2 — Integrações e gates Manager/Swagger

NATS dedicado (`WZAP_TEST_NATS_URL`), bucket S3/MinIO dedicado, `pnpm --dir manager test|lint|typecheck|build`, Swagger diff-clean. Registrar serviços reais vs fakes e cenários não exercitados.

## Task 6.3 — Revisão do diff e inventário

Confirmar retirada de todo suporte histórico e preservação de segurança, import operacional e `pending:{uuid}`; evidências `file:line`; resolver findings antes do reset.

## Task 7.1 — Reset local (requer confirmação do usuário)

Parar a única réplica local; verificar mounts; remover somente `wzap_pgdata`, `wzap_natsdata`, `wzap_miniodata`, `wzap_media` sem backup; verificar ausência dos quatro volumes antes de recriar. Sem prune global.

## Task 7.2 — Rebuild e validação de instalação vazia

Reconstruir/iniciar réplica única com volumes vazios; recriar `wzap_test`; validar `/healthz`, `/readyz`, seed, login, criação de instância e mídia.

## Task 7.3 — Pareamento/envio/webhook reais

Novo pareamento, envio e webhook com número de teste; registrar evidência sem expor dados pessoais/segredos; tarefa fica pendente se número indisponível.

## Task 7.4 — verify.md e retrospective.md

Sete checks com evidências reais; distinguir integrações reais, fakes e skips; cobrir todas as tarefas e falhas resolvidas.

## Task 7.5 — Sincronizar specs e arquivar

Sincronizar deltas, atualizar Purpose das specs principais; `openspec validate --all --strict`; arquivar por último.

## Inventário de caminhos históricos (busca textual)

Migrações:
- `internal/storage/migrations/00001_init.sql` … `00010_device_jid_cutover_gate.sql` (cadeia completa; 00008_remodel.sql, dois 00009, 00010 gate).
- `internal/storage/postgres/migrate_remodel_test.go`.

Ownership/seed:
- `cmd/wzap/seed.go:16-64` (`ownerBackfiller`, `BackfillOwner` no seed); `cmd/wzap/seed_test.go:262-267`.
- `internal/storage/postgres/instances.go:360-363` (`BackfillOwner`); `internal/storage/postgres/instances_backfill_test.go`.

Nomes:
- `internal/storage/repository.go:20-21`; `internal/instance/service.go:28-29` (`ErrInstanceNameAmbiguous`).
- `internal/instance/name_test.go:79,115`; `internal/instance/service_test.go:122`; `internal/httpapi/instance_reference_test.go:76`; `internal/storage/postgres/instances_name_test.go`.

Idempotência/respostas:
- `internal/httpapi/idempotency.go:193,205,209` (`convertReplayBody`); `internal/httpapi/messages_test.go`.
- `internal/httpapi/dto.go:6,17,270`; `internal/model/model.go:12` (`legacy_error`); `internal/app/runtime_test.go:167`.

Eventos/outbox:
- `internal/events/relay.go:230`; `internal/events/relay_test.go:121,429`.
- `internal/storage/repository.go:185-186`; `internal/storage/postgres/events.go:116-119`; `internal/storage/postgres/events_test.go:296`.

Chatwoot/secrets:
- `internal/storage/postgres/chatwoot.go` (`BackfillTokenSeal`); `internal/storage/postgres/chatwoot_test.go:431-466`.

Mídia:
- `internal/media/storage.go:401,434` (`MigrateLocalFiles`); `:522` (`SetBucket`); `internal/storage/postgres/media_repo.go` (`SetBucket`).
- `internal/media/objects_test.go:562-759` (testes de migração); comando `media-migrate` em `cmd/wzap/main.go`.

Config/logger/routing:
- `internal/logger/logger.go:43-47` (alias `text` deprecated); `internal/config/config_test.go`.
- `internal/httpapi/server.go:68-71` (handler `/api/v1/`).

Manager/docs:
- `manager/app/types/api.ts` (tipos/erros legados); `README.md`, `AGENTS.md` (media-migrate, envs).

## Cobertura dos 11 deltas

| Delta | Tasks |
|---|---|
| `wzap-storage-model` | 2.1, 2.2, 2.3 |
| `wzap-accounts` | 3.1 |
| `wzap-api-keys` | 2.2, 3.1, 4.3 |
| `wzap-instances` | 2.4, 3.1 |
| `wzap-response-contract` | 4.1, 4.2 |
| `wzap-message-lifecycle` | 4.3 |
| `wzap-outbound-messaging` | 4.1, 4.4 |
| `wzap-media` | 5.1, 5.2 |
| `wzap-chatwoot-config` | 3.2 |
| `wzap-operations` | 3.3, 5.4, 6.1–6.3, 7.1–7.2 |
| `wzap-manager` | 5.3 |

## Verificação de ausência de exclusões de fluxos atuais

Busca textual confirma preservação obrigatória: `internal/chatwoot/import/scheduler.go` (import operacional), correlações `pending:` em `internal/chatwoot/inbound/webhook.go` e `internal/chatwoot/mirror/worker.go`, retries e envelope v1 em `internal/events/`. Nenhuma task remove esses fluxos; gates 6.3 e 7.4 re-verificam com evidências `file:line`. Testes de segurança (auth, RBAC, privacidade, concorrência, replay autorizado) são preservados/adaptados, nunca removidos.