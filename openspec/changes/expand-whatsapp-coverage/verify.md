# Verify — expand-whatsapp-coverage

Change: `expand-whatsapp-coverage` (branch `feat/expand-whatsapp-coverage`, base `main`,
fork point `9c3da60`). Data: 2026-09-17. Todas as 17 tarefas de `tasks.md` (1.1–5.2)
marcadas completas; 21 commits à frente de `main`.

## 1. Gates executados na branch (ordem do CI)

Em 2026-09-17, no worktree `.worktrees/expand-whatsapp-coverage`
(toolchain `go1.27.0`, `go.mod` pinado em `go 1.26.0`, `golangci-lint` v2.13.2 = versão do CI):

- `gofmt -l .` — vazio (exit 0).
- `go vet ./...` — limpo (sem saída, exit 0).
- `golangci-lint run` — `0 issues` (exit 0).
- `go test ./... -count=1` — **verde**: todos os pacotes `ok`, 0 falhas
  (incl. `internal/httpapi` 7.5s, `internal/auth` 1.3s, `internal/webhook` 1.3s,
  `internal/session/whatsmeow` 0.5s, `internal/chatwoot/mirror` 0.5s,
  `internal/events` 0.17s, `internal/app`, `internal/message`, `internal/instance`).
  Pacotes sem testes inalterados (`internal/model`, `internal/storage`,
  `internal/storage/migrations`, `internal/version`, `docs`).
- `go build ./...` — limpo (exit 0).
- `WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5432/wzap_test?sslmode=disable' go test ./internal/storage/... -count=1` — **verde**
  (`internal/storage/postgres` 35.9s, `postgrestest` ok). A variável foi setada e o
  banco estava alcançável (listener em `127.0.0.1:5432`), então conta como executado.
- `WZAP_TEST_NATS_URL='nats://127.0.0.1:4222' go test ./internal/events/ -count=1` — **verde**
  (`ok`, 0.21s). Broker alcançável (TCP `127.0.0.1:4222` OK), então conta como executado.

## 2. Verificação por tarefa (comandos do `tasks.md`, todos verdes em 2026-09-17)

- **1.1** `go test ./internal/httpapi/ -run 'TestRevoke|TestMarkRead' -count=1` — ok
  (`200` conectado, `409` desconectado, `422` sem autor em grupo).
- **1.2** `go test ./internal/httpapi/ -run TestPresence -count=1` — ok
  (`200` allowlist, `422` estado desconhecido).
- **1.3** `go test ./internal/httpapi/ -run TestPairPhone -count=1` — ok
  (`200` com canal aberto, `409` sem canal).
- **1.4** `go test ./internal/httpapi/ -run TestSwagger -count=1` — ok
  (rotas da fase 1 documentadas, dual auth + envelopes).
- **2.1** `go test ./internal/message/ ./internal/httpapi/ -count=1` — ok
  (aceite `202` idempotente de enquete/reação/figurinha/lista/botões; replay, `422`, `409`).
- **2.2** `go test ./internal/events/ ./internal/app/ -count=1` — ok
  (inbound voto/reação/resposta interativa no envelope versionado, `event_id` estável).
- **3.1** `go test ./internal/httpapi/ -run TestGroup -count=1` — ok
  (ciclo de vida, membros/admins, convites; `403`/`404`/`422`).
- **3.2** `go test ./internal/httpapi/ -run TestNewsletter -count=1` — ok
  (`404` desconhecido, página com `next_cursor`).
- **3.3** `go test ./internal/events/ ./internal/app/ -count=1` — ok
  (eventos de grupo com ator/afeto e `event_id` estável).
- **3.4** `WZAP_TEST_DATABASE_URL=... go test ./internal/storage/... -count=1` — ok
  (migration aditiva `00006_group_newsletter.sql` aplica em banco limpo; schema isolado por teste).
- **4.1** `go test ./internal/httpapi/ -run TestStatus -count=1` — ok
  (publicar/listar/apagar status texto/imagem/vídeo; `422` acima do limite).
- **4.2** `go test ./internal/app/ ./internal/httpapi/ -run 'TestCall' -count=1` — ok
  (evento inbound de chamada; rejeição `200` quando suportado, `501` documentado senão).
- **4.3** `go test ./internal/httpapi/ -run 'TestProfile|TestPrivacy' -count=1` — ok
  (`200` válido, `422` fora da allowlist).
- **5.1** `gofmt -l .` vazio + `go vet ./...` limpo — ok (README, Swagger e `openspec/specs/` atualizados sem **BREAKING**).
- **5.2** Gates finais na ordem do CI (`vet` → `lint` → `test` → `build`) — todos limpos (seção 1).

## 3. Contratos preservados (não tocados pela change)

- Envelopes REST (`{"data": ...}` / `{"error": {"code", "message"}}` + `X-Request-Id`).
- Contrato versionado de eventos (`event_version: 1`, `event_id` estável como
  `Nats-Msg-Id`); entrega at-least-once com dedupe por `event_id` no consumidor.
- Fronteira de auth inalterada (JWT de sessão manager em cookie httpOnly OU header
  `apikey:` global/por instância); nenhuma rota pública nova (`/healthz`, `/readyz`,
  `/swagger/*`, `/manager/` seguem as únicas públicas).
- Sem **BREAKING**: tudo aditivo (rotas, corpos, tipos de evento, migration aditiva).

## 4. Diff resumido

106 arquivos vs `main` (`+23983/−1445`): Fase 1 (revogação, leitura, presença,
pair-phone + Swagger), Fase 2 (mensagens ricas, grupos, newsletters, status, chamadas,
perfil/privacidade), eventos inbound correspondentes, migration `00006_group_newsletter.sql`
+ repositórios Postgres, `README.md`/`docs/*` regenerados, 8 deltas em
`openspec/changes/expand-whatsapp-coverage/specs/` sincronizadas para `openspec/specs/`
(`wzap-groups`, `wzap-message-lifecycle`, `wzap-newsletters`, `wzap-phone-pairing`,
`wzap-presence`, `wzap-profile-privacy`, `wzap-rich-messaging`, `wzap-status-calls`).

## 5. Code review final (2026-09-17, subagente revisor sobre `9c3da60...831b8d5`)

- **Strengths**: contrato aditivo preservado; auth/ownership consistente em todos os
  handlers novos; validação antes da sessão com erros sem vazamento; `PairPhone` lê a
  expiração do canal antes de emitir; grupo parcial `201` em falha de convite;
  migration só `CREATE TABLE IF NOT EXISTS`.
- **Critical**: nenhum encontrado.
- **Important 1** (foto de grupo/perfil aceita qualquer `image/*`): procedente em parte —
  verificado no código que `SetGroupPhoto` encaminha bytes crus ao upstream e erro
  genérico cai no `default` 500 de `writeInstanceError` (`instances.go:693`). Porém:
  (a) `SetProfilePhoto` sempre responde `501` (sem setter na lib pinada), então metade
  do issue é moot; (b) allowlist de header não validaria os bytes de verdade
  (sniffing seria escopo novo, fora de `tasks.md`/design). Triagem: **follow-up
  não-bloqueante** (validar magic bytes ou allowlist `jpeg/png/webp` em change futura).
- **Important 2** (creates de grupo sem middleware `Idempotency`): **rejeitado com
  motivo** — decisão explícita do design ("ricas assíncronas no outbox, resto
  síncrono"; grupos são operações de conta/conversa) e o Swagger já adverte contra
  retry do create. Não é defeito.
- **Minor 3–7** (upsert por item em `ListNewsletters`, descrição em bytes vs runas,
  trim no limite de nome, heurística `isGroupJID` `@g.us`, letra da spec de
  `revoked:true`): registrados como follow-ups, nenhum bloqueante.
- **Assessment final**: READY TO MERGE com follow-ups documentados (nenhum Critical,
  Importants triados acima).

## 6. Pós-merge

Reexecutar `go test ./... -count=1` sobre o resultado do merge antes de qualquer
cleanup; com `WZAP_TEST_DATABASE_URL`/`WZAP_TEST_NATS_URL` setados, incluir os
testes de integração (ambos verdes nesta branch).
