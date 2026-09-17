# Plano: expand-whatsapp-coverage

Plano de execução por ID de tarefa de `tasks.md`. Somente planejamento —
nenhum código foi alterado neste passo.

## 0. Constraints globais (valem para todas as tarefas)

- Contratos congelados: envelopes `{"data": ...}` / `{"error": {"code",
  "message"}}` + `X-Request-Id`; `event_version: 1` com `event_id` estável
  enviado como `Nats-Msg-Id`; entrega at-least-once com dedupe por
  `event_id` no consumidor. Sem **BREAKING**: tudo é aditivo (rotas, corpos,
  tipos de evento). Qualquer quebra descoberta no apply deve ser marcada com
  **BREAKING** antes de implementar.
- Fronteira de auth inalterada: dual auth (JWT de sessão manager em cookie
  httpOnly OU header `apikey:` global/por instância) + ownership por dono
  imutável. Rotas novas: `404` instância inexistente, `403` sem ownership,
  `409` desconectada/sem canal, `422` conteúdo inválido, `501` documentado
  quando o upstream não suportar. Nenhuma rota pública nova (`/healthz`,
  `/readyz`, `/swagger/*`, `/manager/` continuam as únicas públicas).
- TDD obrigatório para mudança de comportamento: teste falhando primeiro,
  implementação mínima depois. Seguir padrões existentes de handlers
  (`instanceID()` → `denyForeignInstanceKey()` → `instances.Get()` →
  `authorizeInstance()` → decodifica corpo → chama serviço → `JSON`/`Error`
  envelopados), como em `internal/httpapi/connection.go` e
  `internal/httpapi/messages.go`.
- Fase 1 não toca na sessão: só transporte + validação + mapeamento de erro
  sobre `DeleteMessage`, `MarkRead`, `SendPresence`, `PairPhone` já existentes
  em `internal/session/session.go`. Fase 2 pode estender `Session` + adapter
  `internal/session/whatsmeow/` + fakes `internal/session/sessiontest/`.
- Imports internos sempre rooted em `wzap`. `gofmt -l .` deve imprimir nada.
  Commits convencionais (`feat(api):`, `feat(events):`, `feat(storage):`,
  `docs(specs):`).
- `tasks.md` é o contrato de escopo: marcar `- [ ]` → `- [x]` somente após a
  verificação declarada na tarefa passar de verdade.

## 1. Estratégia de worktree e ordem

- Worktree isolado (não reutilizar o de `manager-dashboard-frontend`):
  ```bash
  git -C /home/obsidian/dev/wzap fetch origin
  git -C /home/obsidian/dev/wzap worktree add .worktrees/expand-whatsapp-coverage -b feat/expand-whatsapp-coverage origin/main
  ```
  Todo o apply roda dentro de
  `/home/obsidian/dev/wzap/.worktrees/expand-whatsapp-coverage`. O diretório
  principal permanece limpo; nunca commitar arquivos de um change no worktree
  do outro. Ao final, PR a partir de `feat/expand-whatsapp-coverage`.
- Ordem estrita: Fase 1 (lote A) primeiro, mergeável sozinha; depois Fase 2
  na ordem mensagens ricas (lote B) → grupos/newsletters (lote C) → status,
  chamadas, perfil/privacidade (lote D) → fechamento (lote E). Não iniciar o
  lote seguinte com o anterior vermelho.
- Subagentes (`subagent-driven-development`): no máximo um lote por vez por
  worktree; lotes são sequenciais por dependência de `Session`/`Server`/
  migração. Dentro de um lote, testes de IDs diferentes podem ser escritos em
  paralelo, mas a implementação e o `server.go` são serializados.
- `verify.md` e `retrospective.md` são pós-apply, antes do PR — fora deste
  plano, não escrever agora.

## 2. Verificação por lote (padrão repetido em todos)

Ao fechar cada lote, rodar nesta ordem dentro do worktree, e só avançar com
saída limpa:

```bash
gofmt -l .
go vet ./...
go test <pacotes focados do lote> -count=1
```

- `go test ./... -count=1` completo só no lote E (tarefa 5.2), mais
  `golangci-lint run` e `go build ./...` na ordem do CI
  (`go vet` → `golangci-lint` → `go test` → `go build`).
- Testes Postgres (`WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5432/wzap_test?sslmode=disable'`)
  somente no lote C (tarefa 3.4) e no gate final se a variável estiver
  definida; sem ela, reportar como pulados, nunca como executados. Teste NATS
  (`WZAP_TEST_NATS_URL`) somente onde o lote declarar.

## 3. Lote A — Fase 1: expor sessão existente (tarefas 1.1–1.4)

Base: `Session` já tem os 4 métodos; `InstanceService`
(`internal/httpapi/instances.go:31`) hoje só tem
`Create/OldestAdmin/Get/List/Update/Delete/Disconnect/Connect/QR` e precisará
de 4 delegações finas ao `instance.Service` → `session.Manager`.

### 3.1 Tarefa 1.1 — revogação e leitura

Arquivos: `internal/instance/service.go` (métodos `RevokeMessage`,
`MarkRead` delegando à sessão + mapeando `session.ErrNotConnected` →
`instance.ErrNotConnected`), `internal/httpapi/lifecycle.go` (novo:
`handleRevokeMessage`, `handleMarkRead` + requests/responses + anotações
swag), `internal/httpapi/server.go` (registrar `POST
/instances/{id}/messages/revoke` e `POST /instances/{id}/chats/mark-read`),
`internal/httpapi/lifecycle_test.go` (novo), `internal/httpapi/instances_test.go`
(fake: acrescentar `revokeFn`/`markReadFn`).

Micro-passos TDD:
1. Estender o fake com erro configurável por chamada (conectado vs
   `instance.ErrNotConnected` vs erro de janela).
2. RED: `TestRevoke` — `200` com `{"data":{"revoked":true}}` conectado;
   `409` desconectado; `422` alvo inválido/janela expirada (`revoked:false`
   + motivo documentado, nunca `500`, cf. design); `404`/`403` herdados do
   padrão. `TestMarkRead` — `200` em 1:1 sem `sender` (completa com o próprio
   `chat`); `422` em grupo sem `sender`; `409` desconectada.
3. GREEN: handlers síncronos diretos à sessão (sem outbox, sem `202`),
   validando `chat`, `message_id` não vazios e `sender` obrigatório em grupo
   (heurística `isGroup` = JID grupo, igual ao adapter).
4. Refino: traduzir resposta "fora da janela" do upstream em `200`
   `revoked:false` + motivo OU `422` — decidir contra o adapter no apply e
   documentar a escolha no Swagger; sem vazar erro interno.

Verificação: `go test ./internal/httpapi/ -run 'TestRevoke|TestMarkRead' -count=1`
+ padrão do lote (`gofmt -l .`, `go vet ./...`).
Commit: `feat(api): expose message revoke and mark-read`.

### 3.2 Tarefa 1.2 — presença

Arquivos: `internal/instance/service.go` (`SendPresence`), `internal/httpapi/presence.go`
(novo: `handleSendPresence`), `internal/httpapi/server.go` (`POST
/instances/{id}/presence`), `internal/httpapi/presence_test.go` (novo).

Micro-passos TDD:
1. RED: `TestPresence` — `200` para cada estado da allowlist (`composing`,
   `paused`, `available`, `unavailable`); `422` para estado desconhecido
   (validar antes de tocar a sessão); `409` desconectada; `404`/`403`
   padrão.
2. GREEN: validação de allowlist no handler + chamada direta a
   `Session.SendPresence(chatJID, state)`; sem modo contínuo/heartbeat
   (risco de throttle documentado no design).
3. Swagger da rota com `chat` + `state`.

Verificação: `go test ./internal/httpapi/ -run TestPresence -count=1` + padrão do lote.
Commit: `feat(api): expose presence`.

### 3.3 Tarefa 1.3 — pareamento por telefone

Arquivos: `internal/instance/service.go` (`PairPhone`: exige canal aberto,
mapeia "sem canal" → conflito), `internal/httpapi/pair_phone.go` (novo:
`handlePairPhone`), `internal/httpapi/server.go` (`POST
/instances/{id}/pair-phone`), `internal/httpapi/pair_phone_test.go` (novo).

Micro-passos TDD:
1. RED: `TestPairPhone` — `200 {"data":{"pairing_code","expires_at"}}` com
   canal aberto (após `connect`); `409` sem canal (`connect` prévio
   ausente); `422` número inválido com mensagem genérica (anti-enumeração);
   `404`/`403` padrão.
2. GREEN: delegação a `Session.PairPhone`; não abrir canal implicitamente
   (simetria com QR); expiração = expiração do canal/QR.
3. Logar `op=pair-phone` no padrão de `connection.go` sem logar o número.

Verificação: `go test ./internal/httpapi/ -run TestPairPhone -count=1` + padrão do lote.
Commit: `feat(api): expose phone pairing`.

### 3.4 Tarefa 1.4 — Swagger da fase 1

Arquivos: anotações `@Router/@Param/@Success/@Failure` em `lifecycle.go`,
`presence.go`, `pair_phone.go`; `docs/swagger.json`, `docs/swagger.yaml`,
`docs/docs.go` (regenerados); `internal/httpapi/swagger_test.go` (existente,
check de frescura).

Micro-passos:
1. `swag init --parseInternal` (mesmo comando do CI/docs do repo).
2. Conferir dual auth (`@Security apikey`), envelopes e códigos
   `200/401/403/404/409/422/500` em cada rota nova.
3. `git diff --exit-code docs/` deve refletir só as 4 rotas; teste de
   frescura verde.

Verificação: `swag init --parseInternal` sem diff inesperado +
`go test ./internal/httpapi/ -run TestSwagger -count=1` + padrão do lote.
Commit: `docs(api): document phase-1 routes`.
Gate do lote A: `gofmt -l .`, `go vet ./...`,
`go test ./internal/httpapi/ ./internal/instance/ -count=1`.

## 4. Lote B — Fase 2: mensagens ricas (tarefas 2.1–2.2)

### 4.1 Tarefa 2.1 — aceite de enquete, reação, figurinha, lista, botões

Arquivos: `internal/message/service.go` (`TypePoll`, `TypeReaction`,
`TypeList`, `TypeButtons`; `type=sticker` via mídia; novos campos em
`EnqueueInput` + validação por tipo), `internal/message/senders.go`
(construção `session.OutboundMessage` por tipo), `internal/httpapi/messages.go`
(novos handlers/rotas ou extensão do aceite), `internal/httpapi/media.go`
(allowlist `type=sticker` com validação webp + `MaxMediaBytes`),
`internal/message/service_test.go`, `internal/message/senders_test.go`,
`internal/httpapi/messages_test.go`, `internal/httpapi/media_test.go`.

Micro-passos TDD:
1. RED no `message`: `Enqueue` aceita `poll` (pergunta + ≥2 opções + limite
   de opções), `reaction` (emoji ou vazio = remover), `sticker` (só via
   `/messages/media` com `type=sticker`), `list` (seções/linhas), `buttons`
   (incl. PIX quando suportado); replay com mesmo `Idempotency-Key` retorna
   o mesmo id + header `X-Idempotent-Replay`; `422` conteúdo inválido; `409`
   desconectada.
2. GREEN: validação por tipo no `Service` (nada persistido quando inválido);
   `sticker` reusa `handleSendMedia` + `validMediaKind` estendido (sem
   endpoint novo de upload).
3. RED no `httpapi`: um teste por tipo cobrindo `202` + replay + `422` +
   `409`; `GET /messages/{id}` continua lendo os novos tipos.
4. Confirmar no apply as duas questões abertas do design aplicáveis aqui:
   `501` vs `422` para não suportado pelo upstream; remoção de reação
   (emoji vazio no mesmo endpoint).

Verificação: `go test ./internal/message/ ./internal/httpapi/ -count=1` + padrão do lote.
Commit: `feat(api): accept rich message types`.

### 4.2 Tarefa 2.2 — eventos inbound de voto, reação, resposta interativa

Arquivos: `internal/session/session.go` (tipos `PollVote`, `Reaction`,
`InteractiveResponse` + métodos no `EventSink`), `internal/session/whatsmeow/`
(tradução dos tipos library → domínio), `internal/session/sessiontest/`
(fakes), `internal/app/` (sink → outbox + mídia quando houver),
`internal/events/` (construtores de envelope + subjects), `internal/webhook/`
(entrega, sem mudar assinatura default — tipos novos opt-in via `events`),
`internal/app/*_test.go`, `internal/events/*_test.go`.

Micro-passos TDD:
1. RED: cada inbound gera envelope versionado (`event_version: 1`,
   `event_id` estável sobrevivendo a retry) via outbox + NATS + webhook;
   consumidor dedupe por `event_id`.
2. GREEN: sink → outbox com `Nats-Msg-Id = event_id`; `Raw` best-effort para
   o webhook.
3. Cobrir `at-least-once`: retry republica com o mesmo `event_id`.

Verificação: `go test ./internal/events/ ./internal/app/ -count=1`
(+ `WZAP_TEST_NATS_URL` quando o broker estiver disponível para o teste de
integração do publisher; sem ele, unitários devem passar) + padrão do lote.
Commit: `feat(events): inbound poll, reaction and interactive events`.
Gate do lote B: `gofmt -l .`, `go vet ./...`,
`go test ./internal/message/ ./internal/httpapi/ ./internal/events/ ./internal/app/ -count=1`.

## 5. Lote C — Fase 2: grupos e newsletters (tarefas 3.1–3.4)

### 5.1 Tarefa 3.1 — grupos

Arquivos: `internal/session/session.go` (métodos de grupo: criar, consultar,
atualizar assunto/descrição/foto, membros/admins, entrar/sair por convite,
códigos), `internal/session/whatsmeow/` + `sessiontest/`,
`internal/instance/service.go` (delegações), `internal/httpapi/groups.go`
(novo) + `server.go` (rotas), `internal/httpapi/groups_test.go` (novo:
`TestGroup*`).

Micro-passos TDD:
1. RED: ciclo de vida + membros/admins + convites; matriz `403` sem
   ownership / `404` grupo ou instância desconhecidos / `422` payload
   inválido; cenários de dono e convite da tarefa.
2. GREEN: operações síncronas (conta/conversa, não mensagem — sem outbox);
   erros do upstream traduzidos sem vazar detalhe interno.
3. RBAC: dono imutável reutilizado; instance key só na própria instância.

Verificação: `go test ./internal/httpapi/ -run TestGroup -count=1` + padrão do lote.
Commit: `feat(api): manage groups`.

### 5.2 Tarefa 3.2 — newsletters

Arquivos: `internal/httpapi/newsletters.go` (novo: seguir/deixar de seguir,
consultar, listar com cursor `next_cursor`), `server.go`,
`internal/httpapi/newsletters_test.go` (`TestNewsletter*`).

Micro-passos TDD:
1. RED: `404` canal desconhecido; página com `next_cursor`; limites default
   50 / max 100 como nas listagens existentes.
2. GREEN: refresh sob demanda dos metadados (cache, nunca fonte de verdade).

Verificação: `go test ./internal/httpapi/ -run TestNewsletter -count=1` + padrão do lote.
Commit: `feat(api): follow and list newsletters`.

### 5.3 Tarefa 3.3 — eventos de grupo

Arquivos: mesmos de 2.2 (`session.go` eventos de grupo, adapter, `app/`,
`events/`, `webhook/`).

Micro-passos TDD:
1. RED: evento de grupo no envelope versionado com ator, afetado e
   `event_id` estável.
2. GREEN: via outbox + NATS + webhook, mesmo caminho de 2.2.

Verificação: `go test ./internal/events/ ./internal/app/ -count=1` + padrão do lote.
Commit: `feat(events): group events`.

### 5.4 Tarefa 3.4 — migration de metadados grupo/newsletter

Arquivos: `internal/storage/migrations/00006_group_newsletter.sql` (novo,
aditivo: tabelas de metadados + `updated_at`), `internal/storage/repository.go`
(contratos), `internal/storage/postgres/` (implementação),
`internal/storage/postgres/postgrestest` (isolamento já existente — nunca
tocar o schema compartilhado).

Micro-passos:
1. Escrever migration só aditiva (`CREATE TABLE IF NOT EXISTS`, sem
   `ALTER` destrutivo); aplica em banco limpo via `wzap migrate` e via
   `WZAP_AUTO_MIGRATE=true`.
2. TTL/refresh sob demanda consumindo o repositório; `updated_at` exposto
   nas respostas.
3. Teste de integração com schema isolado por teste.

Verificação:
`WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5432/wzap_test?sslmode=disable' go test ./internal/storage/... -count=1`
+ padrão do lote. Pré-requisito uma vez (fora do plano):
`docker compose exec postgres createdb -U wzap wzap_test` em volume novo.
Commit: `feat(storage): group and newsletter metadata`.
Gate do lote C: `gofmt -l .`, `go vet ./...`,
`go test ./internal/httpapi/ ./internal/events/ ./internal/app/ -count=1`
+ storage com Postgres quando disponível.

## 6. Lote D — Fase 2: status, chamadas, perfil e privacidade (tarefas 4.1–4.3)

### 6.1 Tarefa 4.1 — status/stories

Arquivos: `internal/httpapi/status.go` (novo: publicar `202` quando via
outbox / `200` consulta / apagar), `server.go`, `internal/httpapi/status_test.go`
(`TestStatus*`).

Micro-passos TDD:
1. RED: publicar texto/imagem/vídeo com limite de mídia; `202`/`200` ok;
   `422` acima do limite; apagar e listar.
2. GREEN: reuso do pipeline de mídia (`WZAP_DATA_DIR` + `MaxMediaBytes`).

Verificação: `go test ./internal/httpapi/ -run TestStatus -count=1` + padrão do lote.
Commit: `feat(api): publish and manage status`.

### 6.2 Tarefa 4.2 — chamadas

Arquivos: `internal/app/` (eventos inbound oferta/aceite/recusa/fim),
`internal/httpapi/calls.go` (rejeitar ativa), `server.go`.

Micro-passos TDD:
1. RED (`TestCall*`): evento inbound observado; rejeição responde `200`
   quando o upstream suporta, `501` documentado quando não.
2. GREEN: nunca iniciar chamada (fora do protocolo suportado —
   out-of-scope explícito); só observar/rejeitar.

Verificação: `go test ./internal/app/ ./internal/httpapi/ -run 'TestCall' -count=1`
+ padrão do lote.
Commit: `feat(api): observe and reject calls`.

### 6.3 Tarefa 4.3 — perfil e privacidade

Arquivos: `internal/httpapi/profile.go` (novo: nome, foto, recado;
privacidade: última visualização, foto, status, confirmações de leitura,
quem pode adicionar a grupos), `server.go`,
`internal/httpapi/profile_test.go` (`TestProfile|TestPrivacy`).

Micro-passos TDD:
1. RED: `200` válido; `422` fora da allowlist (validar antes da sessão).
2. GREEN: síncrono; `501` se o upstream não suportar privacidade fina
   (decidir no apply, documentar por operação no Swagger).

Verificação: `go test ./internal/httpapi/ -run 'TestProfile|TestPrivacy' -count=1`
+ padrão do lote.
Commit: `feat(api): manage profile and privacy`.
Gate do lote D: `gofmt -l .`, `go vet ./...`,
`go test ./internal/httpapi/ ./internal/app/ -count=1`.

## 7. Lote E — Fechamento (tarefas 5.1–5.2)

### 7.1 Tarefa 5.1 — README, Swagger, `openspec/specs/`

Arquivos: `README.md` (rotas + eventos + exemplos de envelope), `docs/*`
(regenerado), `openspec/specs/` (sync das 8 deltas de
`openspec/changes/expand-whatsapp-coverage/specs/` sem reescrever requisitos
à mão — copiar comportamento observável, manter `WHEN/THEN`), `tasks.md`
desta change (nada aqui além de marcar no apply).

Micro-passos:
1. `swag init --parseInternal` final + check de frescura.
2. Documentar por operação: suporte upstream, janela de revogação e formato
   de `revoked:false`, allowlists de presença/perfil/privacidade, opt-in de
   webhook para os tipos novos.
3. `gofmt -l .` vazio + `go vet ./...` limpo.

Verificação: a da tarefa (`gofmt -l .` vazio, `go vet ./...` limpo).
Commit: `docs(specs): expand whatsapp coverage`.

### 7.2 Tarefa 5.2 — gates finais (ordem do CI)

Dentro do worktree, exatamente nesta ordem (`.github/workflows/ci.yml`):

```bash
gofmt -l .
go vet ./...
golangci-lint run
go test ./... -count=1
go build ./...
```

Variantes com integração (somente se os serviços estiverem de pé; senão
reportar como pulados, nunca como verdes):
```bash
WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5432/wzap_test?sslmode=disable' go test ./... -count=1
WZAP_TEST_NATS_URL='nats://127.0.0.1:4222' go test ./internal/events/ -count=1
```

Critério de saída: quatro comandos limpos + build ok antes de qualquer
apply em `main`; checkboxes 1.1–5.2 marcados somente então. Não escrever
`verify.md`/`retrospective.md` neste plano (pós-apply, antes do PR).
