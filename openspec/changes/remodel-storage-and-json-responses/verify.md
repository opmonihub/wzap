# Verify — remodel-storage-and-json-responses

Verificação documental da etapa 6 (6.1 revisão/gates + 6.2 ensaio de corte e
recuperação). Executado em 2026-10-06 no worktree
`.worktrees/remodel-storage-and-json-responses`, branch
`codex/remodel-storage-and-json-responses`, HEAD final da verificação
`49a0024` (`fix(media): upload migrated objects into the configured bucket`).
Nenhum corte real foi executado: todos os ensaios correram em clones
descartáveis, banco `_test` e serviços de verificação. Nenhum merge, push,
sync de specs ou archive foi feito aqui (integração é do controller).

## BREAKING — consumidores REST

**BREAKING**: os corpos de resposta REST foram remodelados (design §5). DTOs de
transporte próprios: coleções viraram `data.items[].<entity>`, leituras únicas
`data.<entity>`, instância aninhada em `connection`/`webhook`, erro de conexão
em `last_error: {code,message,occurred_at}|null`, `instances_used` calculado
pelo backend, `instance_api_key` só em criação/rotação, token Chatwoot nunca
em leitura. Consumidores REST e o Manager devem usar o contrato novo; clientes
que leem os campos antigos (planos de instância, `external_ref`/`owner`/JIDs em
leitura, resposta de envio com campos zerados) quebram. Também é visível ao
consumidor: replay de idempotência converte corpos legados e responde **410
Gone** `idempotency_response_expired` a corpos não conversíveis — o corpo cru
legado nunca ressurge.

Não é BREAKING para consumidores de eventos: `event_version: 1`, subjects,
campos e `event_id` estável (enviado como `Nats-Msg-Id`) estão preservados
(fixtures de eventos v1 inalteradas; verificado por inspeção + suíte).

## Critérios de liberação do corte (task 6.2)

| Critério | Situação | Evidência |
|---|---|---|
| Todas as tarefas de comportamento verificadas | comprovado | tasks 1.x–5.3 com gates por tarefa; revisão 6.1 ampla (3 áreas) + onda de fix + re-review 6/6 ADDRESSED (`task-18-gates-report.md`, `fix-6.1-report.md`, `progress.md`) |
| Backups e recuperação ensaiados | comprovado | dump fresco + `gzip -t` do artefato histórico + restauração em clone B idêntica ao baseline (ver "Ensaio de recuperação") |
| Imagem exata comprovada | comprovado | `docker.io/cccs/minio@sha256:68eefa6a5ccd82178a872b2d1012687d0ac9b1afa848a1e56f55fa5f1efcb081` (digest pinado em 3 ocorrências de 2 arquivos: `docker-compose.yml:38,59` e `docker-compose.dev.yml:46`; recheck local + container `wzap-minio-verify` rodando `RELEASE.2024-12-18T13-15-44Z`) |
| Nenhuma referência órfã ou divergência sem tratamento | comprovado com ressalva | órfãos de mídia anulados e reportados (`remodel_report.orphan_media_refs=3`, `media_chatwoot_markers=1`, `legacy_connection_errors=2`); divergência `device_jid` não bloqueia o corte — ver DEFER 1 |
| Cobertura de todas as rotas | comprovado | 88/88 rotas registradas no Swagger (task 5.3); suíte de contrato com 89 fixtures por operação da matriz (task 5.1) |
| Manager e Swagger compatíveis | comprovado | manager test/typecheck/lint/build verdes (gates 6.1 + fix wave); Swagger regenerado e testado (5.3, `6862b5d`) |
| Eventos v1 preservados | comprovado | envelopes/subjects intocados pela change; fixtures v1 inalteradas; publisher real exercitado em NATS isolado (gate 3) |
| Nenhum replay produz efeito duplicado | comprovado | ensaio ao vivo: replays com `X-Idempotent-Replay`, `message_queue` com delta 0 em todos os demos; suíte cobre o caminho 202-armazenado |

## Gates re-executados na etapa 6.2

Justificativa da re-execução: os commits `ad75dbb` (somente `internal/media/**`)
e `49a0024` (também `internal/storage/repository.go` e
`internal/storage/postgres/media_repo.go` + testes — o repositório de mídia),
do fix round 2 em voo paralelo durante esta task, entraram depois dos gates
6.1 (`6862b5d`) e da re-execução do fix wave (`e9ee04b`). A ordem CI completa
do plano (etapa 6, passo 2: `go vet` → `golangci-lint run` → `go test` →
`go build`) está coberta e certificada neste estado final.

| Comando | Resultado |
|---|---|
| `gofmt -l .` | vazio |
| `go vet ./...` | pass |
| `golangci-lint run` | **1 issue (errcheck)** no HEAD `7e0c33e`: `internal/media/objects_test.go:691`, `rc.Close()` sem checagem (herdado do fix round 2) — corrigido para `_ = rc.Close()` (estilo já usado no arquivo) e re-executado: **0 issues.** |
| `go test ./... -count=1` (unit, sem vars de integração) | pass — 27 pacotes `ok`; `internal/storage/postgres` roda skipped (0.087s) sem `WZAP_TEST_DATABASE_URL` |
| `WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5435/wzap_test?sslmode=disable' go test ./... -count=1` | pass — integração real: `internal/storage/postgres ok 115.9s`, `cmd/wzap ok 34.5s`, `internal/httpapi ok 26.5s`, `chatwoot/import ok 21.0s`, `session/whatsmeow ok 16.7s` (2864+ testes, schemas isolados) |
| `WZAP_TEST_S3_ENDPOINT='http://127.0.0.1:19000' go test ./internal/media/ -run TestObjectStoreS3Integration -count=1` | PASS — MinIO real (`wzap-minio-verify`, digest pinado) |
| `go build ./...` | pass (em `7e0c33e` e re-executado após a correção de errcheck) |

Gates 6.1 originais (`task-18-gates-report.md`, em `6862b5d`): gofmt/vet/lint(0
issues)/build/unit; Postgres real `wzap_test@5435`; NATS isolado `4324`
(`TestNATSPublisherIntegration` + `TestMirrorConsumerRedeliveryIsIdempotent`);
MinIO `wzap-minio-verify@19000`; manager test/typecheck/lint/build. Re-execução
do fix wave (`fix-6.1-report.md`, em `e9ee04b`) refez gofmt/vet/lint/build/unit
+ Postgres real + manager test/typecheck/lint. NATS não foi re-executado em
6.2: os commits pós-gates (`ad75dbb`, `49a0024`) tocaram `internal/media` e o
repositório de mídia em `internal/storage` (`repository.go`,
`postgres/media_repo.go`), e nenhum deles alterou caminhos de `internal/events`
(além das docstrings do fix wave); o gate 3 cobre.

## Ensaio de corte e recuperação (6.2)

Ambiente: execução `rehearsal-20261006t224652z`. Artefatos privados (dumps,
inventários, logs, credenciais do ensaio) em
`backups/rehearsal-20261006t224652z/` (git-ignored via `.git/info/exclude`,
modo 0600; não commitados). Serviços usados: clones PostgreSQL 18 efêmeros
(`postgres:18-alpine`, tmpfs, portas só em 127.0.0.1:55432/55433 — nunca os
bancos reais `wzap`/`wzap_test` de 5435), MinIO de verificação
`wzap-minio-verify@19000`, binário real do serviço compilado do worktree.
NATS: endpoint propositalmente inalcançável (`nats://127.0.0.1:4399`) para que
nenhum evento fosse publicado para fora durante o ensaio.

### 1. Backup e baseline

- Dump fresco (somente leitura) do banco dev: `docker exec wzap-postgres-1
  pg_dump -U wzap -d wzap --schema=public` → `baseline-public.sql` (11.7 MB,
  64.120 linhas). Sanidade: **0** ocorrências de `CREATE DATABASE`/`\connect`
  (restauração contida no banco alvo).
- Artefato histórico `backups/wzap-dev-20261006.sql.gz` (3.942.708 bytes):
  `gzip -t` íntegro.
- Staging de mídia sem remoção de origem: `docker cp wzap-wzap-1:/data` → cópia
  vazia (volume sem arquivos; consistente com `media` sem rows na origem).
  Para exercitar arquivos, o ensaio semeou 3 rows de mídia + arquivos (fixtures
  rotuladas: `1111…` com WA ID real, `2222…` com marcador `chatwoot-42-1`,
  `3333…` expirada) e 1 row extra fs-era (`4444…`, bucket `local`) — dados de
  ensaio em clone, não produção.
- Inventário reproduzível: `rehearsal-inventory.sql` (somente leitura,
  tolerante a pré/pós-corte). Baseline pré-corte em
  `inventory-A0-precut.txt` (12 tabelas próprias, goose v0–7).

### 2. Corte simulado (clone A)

`wzap migrate` (binário real) sobre o clone restaurado: `OK 00008_remodel.sql
(499.81ms)`, `goose: successfully migrated database to version: 8`.

Comparação pré (`A1`, baseline + sementes) ↔ pós (`A2`):

| Item | Resultado |
|---|---|
| IDs (digests md5 do conjunto ordenado) | **idênticos**: `instances 96d7db7e…`, `users ac1339d8…`, `message_queue e16720c6…`, `media 5d0aa434…`, `idempotency_keys 8ba4fe5c…` |
| Contagens | preservadas: 2 instances, 1 user, 3 message_queue, 3 media, 5 idempotency_keys; derivados novos corretos (`instance_connections=2`, `instance_webhooks=2`, `remodel_report=6`) |
| Constraints | exatamente o conjunto aprovado: FKs novas (`message_queue_media_id_fkey` SET NULL, `chatwoot_messages_message_id_fkey` SET NULL, satélites CASCADE, `instances_owner_user_id_fkey` SET NULL), `UNIQUE(instance_id,key)`, `UNIQUE(bucket,object_key)`, `UNIQUE(instance_id,wa_key)`, CHECKs de enum (`send_status`, `direction`, `status` conexão, `role`) — ver `constraints-postcut-key.txt` |
| Owners | idênticos: as 2 instâncias seguem do usuário `e1262bf7-…` |
| Quotas | idênticas: `instance_limit=0` (ilimitado) preservado |
| Arquivos | SHA-256 dos arquivos de mídia **inalterados** pelo corte (diff vazio pré/pós) |
| Outbox | 306 → 0 rows: comprovado no dump que as 306 eram **todas publicadas** (`published_at` preenchido; 0 pendentes antes e depois) — remoção de confirmados é a política do design §7, pendentes sobrevivem |
| Marcadores Chatwoot | `media.wa_id` com `chatwoot-42-1` virou `NULL` e foi reportado (`remodel_report.media_chatwoot_markers=1`); WA IDs reais preservados |
| Órfãos de mídia | 3 `message_queue.media_id` apontando para rows inexistentes → anulados + reportados (`orphan_media_refs=3`); 0 referências órfãs após o corte |
| Erros legados | `legacy_connection_errors=2`; `last_error` público sai como `legacy_error` com `occurred_at: null` (sem data inventada) — confirmado ao vivo no `GET /instances` |

### 3. Ensaio de recuperação (clone B)

Clone B (servidor efêmero separado, nunca tocado pelo corte) restaurado do
mesmo `baseline-public.sql`: inventário `B0` **idêntico** a `A0` (diff contém
apenas o rótulo de coluna do digest, diferença do script — o valor
`idempotency_keys … 7dd55454…` coincide). É exatamente o rollback documentado
no design §3: "restaurar o dump em banco paralelo, repontar o app". O reponte
do binário antigo não foi exercitado (exigiria a imagem pré-corte; fora do
alcance desta entrega documental). Não há downgrade in-app (documentado).

### 4. Respostas idempotentes (app real contra o clone pós-corte)

Binário real em `127.0.0.1:18081`, `WZAP_DATA_DIR` = staging, NATS inalcançável
(`readyz` 503 por design, `healthz` 200). Chaves de idempotência semeadas
**antes** do corte (com fingerprint `sha256("POST\nPOST /instances/{id}/messages/text\n"+body)`)
e replayadas **depois** — a chave atravessou o corte:

1. Chave legada com corpo `{"data":{"message_id":"44444444-…","status":"queued"}}`
   → replay responde **202 + `X-Idempotent-Replay: true`** com o corpo
   **convertido** para o contrato novo
   `{"data":{"message":{"id":"44444444-…","instance_id":"8abab8c9-…","send_status":"queued","media_id":null}}}`:
   mesmo UUID entregue uma vez, sem reexecutar a operação, sem campos removidos
   ressurgindo.
2. Chave legada com corpo não conversível → **410 Gone**
   `idempotency_response_expired`; efeito nunca reexecuta, corpo cru não ressurge.
3. Envio real com chave nova e a mesma requisição repetida, mais conteúdo igual
   com chave diferente: `message_queue` com **delta 0** em todos (sem sessão
   WhatsApp o resolver responde 503/422 — ver limitações); observada ao vivo a
   semântica documentada de **liberação de chave** para 4xx e 503 (o retry é
   possível; replay só para respostas armazenadas = 2xx/5xx).
4. `GET /instances` devolve o envelope remodelado exato do design §5
   (`data.items[].instance` com `connection`/`webhook` aninhados,
   `last_error` estruturado ou legado sem data) e **zero** ocorrências de
   `external_ref`, `owner_user_id`, `device_jid`, `whatsapp_jid`, `password`,
   `token`, `instance_api_key` no corpo.

O caminho 202-armazenado → replay não pôde ser produzido ao vivo (requer sessão
WhatsApp real para resolver número): coberto pela suíte
(`TestIdempotencyReplayAcceptsBothEnvelopes`,
`TestIdempotencyUnconvertibleReplayAnswers410`, fixtures de contrato).

### 5. Mídia file→object (transferência verificável)

Executada após o commit do round 2 (`ad75dbb`), complementada em `49a0024`:

- `wzap media-migrate` (binário real, MinIO `wzap-minio-verify@19000`, bucket
  configurado `wzap-media` — igual ao bucket das rows do backfill; sem tocar
  qualquer MinIO de produção, que não existe neste ambiente): **2 objetos
  enviados** (rows não expiradas), SHA-256 conferido no arquivo local antes do
  upload. Re-execução: **0 objetos** (sonda encontra o objeto; convergente).
- Classe fs-era (row com bucket `local`, o alvo do fix `49a0024`): **1 objeto
  enviado para a bucket configurada** e a row reescrita
  (`bucket: local → wzap-media`, `MediaRepository.SetBucket`); re-execução
  convergente (0).
- Leitura de volta pelo app em modo S3 (`GET /media/{id}`): **bytes idênticos**
  aos arquivos locais (SHA-256 `cbada9e8…`, `543d22b2…`, `88abb112…`/`4444…`
  conferidos por `cmp`), `Content-Type` preservado (`text/plain`,
  `application/octet-stream`).
- Mídia expirada: o TTL cleaner do app removeu o arquivo da row expirada e
  marcou `object_deleted_at` (comportamento projetado); `GET` dessa mídia →
  **404** `not_found` mesmo sem objeto remoto.
- Rollback preservado: os arquivos locais permanecem no staging com SHA-256
  inalterado após a transferência (o descarte das origens é etapa de deploy
  futura, fora desta entrega).

## Testes ignorados, fakes e serviços realmente exercitados

- **Serviços realmente exercitados**: PostgreSQL 18 (clones de ensaio +
  `wzap_test@5435` na suíte), MinIO/S3 real (`wzap-minio-verify@19000`, imagem
  digest-pinnada), HTTP API real (binário do worktree, middleware real de
  idempotência), TTL cleaner real, `wzap migrate`/`wzap media-migrate` reais.
- **Não exercitado**: NATS/JetStream durante o ensaio (inalcançável por
  design; o publisher real foi exercitado no gate 6.1 com broker isolado
  4324); sessão WhatsApp real (nenhum número resolvido via upstream; o boot do
  clone tentou reconectar o dispositivo persistido e foi rejeitado como stale —
  único contato externo do ensaio, sem mensagens; ver limitações).
- **Testes ignorados nesta rodada 6.2**: `TestNATSPublisherIntegration` e
  `TestMirrorConsumerRedeliveryIsIdempotent` (exigem `WZAP_TEST_NATS_URL`;
  cobertos pelo gate 3 em broker isolado). `TestObjectStoreS3Integration` foi
  executado de verdade (MinIO real).
- **Fakes**: os unitários usam `fakeObjects`/`sessiontest`/repositórios fake —
  nenhum deles foi usado como evidência de serviço; as evidências acima vêm de
  serviços reais. As rows de mídia/chaves de idempotência do ensaio são
  fixtures semeadas em clone (rotuladas como tal), não dados de produção.

## Limitações e decisões pendentes (DEFER da revisão 6.1)

Adiados deliberadamente pela onda de fix 6.1 (decisão de escopo, não omissão).
Cada um exige decisão/escopo próprios antes ou depois do corte real:

1. **`device_jid` divergente não bloqueia o corte** — o design §3 diz que
   divergência "bloqueia o corte até resolução explícita", mas a migração 00008
   apenas reporta (`remodel_report.device_jid_conflicts`) e segue: há
   autocontradição no design. Requer decisão de design (guard pós-migração que
   aborta, ou emenda do design para "reporta e prossegue"). Por quê: mudar o
   comportamento do corte exige alinhar design+migração juntos; escolher
   silenciosamente um dos lados criaria risco de perda de bind do dispositivo.
2. **Lifecycle rule S3 (`media/`) como backstop** — se `repo.Create` falha após
   o `Put` e o descarte do objeto também falhar, sobra objeto órfão sem row. O
   código já descarta o objeto nesse cenário; a lifecycle rule é defesa em
   profundidade. Por quê: é configuração de infra do bucket, fora do código do
   serviço; precisa de decisão operacional (prefixo, expiração) antes do corte
   real.
3. **`MigrateLocalFiles` não re-lê bytes após o `Put`** — o SHA-256 é conferido
   no arquivo local antes do upload, mas não há confirmação do lado do objeto
   (GET pós-put re-hasheado). Por quê: custo de I/O dobra por objeto; o ensaio
   6.2 fez essa conferência manualmente (leitura de volta byte-idêntica) e um
   checksum nativo do S3/MinIO seria a implementação definitiva.
4. **Backfill de `message_id` (00008) pode casar múltiplas queue rows** — a
   junção `wa_key = message_queue.wa_id` não é determinística quando `wa_id` se
   repete na fila (sem UNIQUE). Por quê: escolher a row "certa" exige regra de
   desempate (mais recente?) que o design não fechou; dado real não tem o caso
   (backfill vazio no ensaio), fixtures cobrem o caminho com evidência única.
5. **`HandleMessageStatus` (chatwoot mirror/worker.go) sem gate
   `global.Enabled`** — os siblings têm o gate; este handler processa
   `message.status` mesmo com o mirror global desligado. Por quê: comportamento
   pode ser intencional (promoção de `pending:{uuid}` não é mirror), mas não há
   registro da intenção; mudar sem decisão pode quebrar a promoção de
   correlação.
6. **`recordPending` engole erro não-`NotFound` de `LatestByConversation`**
   (webhook.go ~475) sem warn. Por quê: silenciar erro de banco esconde
   degradação da correlação; o fix exige escolher o nível de log e o efeito
   (seguir sem correlação vs. falhar o webhook).
7. **`GetByChatwootID` (postgres/chatwoot.go ~266) `LIMIT 1` sem `ORDER BY`**
   com N rows por `cw_id` (cardinalidade N:1 de anexos). Por quê: falta a
   regra de desempate canônica (row mais recente? a com `wa_key` real?); a
   escolha afeta reply/revoke.
8. **`handleListUsers` roda `CountByOwner` por usuário (N+1)** — rota admin.
   Por quê: corrigir exige agregação em uma query; baixo risco (poucos
   usuários, rota administrativa), adiado como otimização.
9. **`create` rejeita `instance_limit: null` com 422 enquanto `PATCH` trata
   `null` como ilimitado** — assimetria; a matriz só especifica `PATCH`. Por
   quê: fechar a intenção em create exige decisão de contrato (aceitar null =
   ilimitado ou manter 422 e documentar), que mudaria o Swagger.
10. **Fixtures de contrato: privacy check só por chaves; group/channel
    assertam presença, não conjuntos exatos** — menor rigor que o
    `requireContractKeys` das demais rotas. Por quê: os conjuntos exatos dessas
    respostas têm campos dinâmicos; o assert exato exigiria normalização que
    não valeu o custo no fix 6.1.
11. **Swagger 2.0 sem marcador `nullable` em campos ponteiro** — limitação do
    formato (Swagger 2.0 não tem `nullable` como OpenAPI 3). Por quê: migrar
    para OpenAPI 3 é mudança de toolchain (`swag`), fora do escopo da change.

## Outras pendências registradas (fora do escopo desta task)

- **Sync de specs** (controller): `specs/wzap-media/spec.md:35` ainda cita a
  tag quay do design antigo; `tasks.md` histórico (linhas 5 e 29) cita
  `cccs/minio:latest` — reconciliar para o digest
  `sha256:68eefa6a5c…` na sincronização de specs, junto do registro da decisão
  (preflight verification-preflight-report.md).
- Prefixo `pending:` duplicado entre const Go (`webhook.go:67`) e literal SQL
  (`chatwoot.go:287`) — minor adiado (re-review round 2).
- `relay.cleanup`/`DeletePublishedBefore` viraram no-op permanente pós
  pending-only — vestigial, inofensivo (re-review round 2).
- Manager lint: 2 warnings pré-existentes (`DataTableToolbar.vue`,
  `PageState.vue`); ferramentas: Go 1.27.0 vs pin 1.26.0 e pnpm 12.4.2 vs
  11.22.0 declarado (registrados no preflight/gates).
- Contato externo do ensaio: o boot do app no clone reconectou o dispositivo
  whatsmeow persistido e foi rejeitado upstream como stale (socket aberto e
  recusado; nenhuma mensagem enviada; `last_error` estruturado
  `session_rejected` registrado no clone). Nenhum outro serviço externo foi
  alcançado; NATS permaneceu desconectado por todo o ensaio.

## Conclusão

Os critérios de liberação do corte estão atendidos para um corte real futuro,
com as ressalvas registradas acima (DEFER 1 em especial: a divergência de
`device_jid` precisa de decisão de design antes do corte em produção). O
rollback (dump → banco paralelo) está ensaiado e comprovado. Integração
(merge), sincronização de specs e archive permanecem com o controller.
