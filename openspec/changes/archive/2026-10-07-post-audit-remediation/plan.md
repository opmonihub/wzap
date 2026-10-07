# post-audit-remediation — plano SDD

> Executar com subagent-driven-development. Um implementer por task; review após cada task.

## Global Constraints

- Commits convencionais; nunca `--amend` salvo hook; nunca segredos.
- Gates: `gofmt -l .` vazio, `go vet ./...`, `golangci-lint run`, `/usr/local/go/bin/go test ./... -count=1`, `go build ./...`.
- Manager: `pnpm --dir manager typecheck` e `pnpm --dir manager build` quando tocar `manager/`.
- Postgres integration: skipped se `WZAP_TEST_DATABASE_URL` ausente; registrar no report.
- Escopo mínimo por task; sem refactors colaterais.

## Task 1 — Fechar instance-config-aggregation no working tree

Write scope: marcar `openspec/changes/archive/2026-10-06-instance-config-aggregation/tasks.md` 7.2; commit de todo o delta de agregação (Go, manager, docs, migration `00009`, specs).

1. Confirmar gates verdes.
2. Commit único ou commits por escopo conforme histórico do change.
3. Marcar task 7.2 `[x]` com nota de specs já em `openspec/specs/`.

## Task 2 — Webhook fanout só após outbox persistido

Write scope: `internal/webhook/worker.go`, `internal/webhook/worker_test.go`.

1. TDD: falha se webhook enfileirado quando `inner.Write` retorna erro; sucesso quando Write OK.
2. `fanoutWriter.Write`: chamar `inner.Write` primeiro; `dispatch` só em sucesso.
3. Atualizar comentários que descrevem ordem inversa.
4. `go test ./internal/webhook/ -count=1`.

## Task 3 — README operacional

Write scope: `README.md`.

1. Shutdown: incluir mirror Chatwoot + import scheduler antes do relay (alinhar `cmd/wzap/main.go`).
2. Documentar subcomando `media-migrate`.
3. Nota curta: outbox não compartilha transação SQL com writes de domínio; webhooks best-effort; fila webhook pode dropar quando cheia.

## Task 4 — OpenSpec: arquivar changes completas

Write scope: `openspec/changes/*`, `openspec/specs/` via CLI.

1. `openspec-archive-change` para: `correct-observed-log-errors`, `support-instance-name-addressing`, `simplify-swagger-instance-listing`, `document-all-http-routes` (tasks já `[x]`).
2. Produzir `verify.md`/`retrospective.md` mínimos onde ausentes (correct-observed-log-errors).

## Task 5 — whatsmeow-parity-routes: reconciliar tasks vs código

Write scope: `openspec/changes/whatsmeow-parity-routes/tasks.md`, testes existentes.

1. Rodar testes nomeados em tasks; marcar `[x]` o que já passa com rotas em `server.go`.
2. Listar gaps reais restantes no report (não implementar fase inteira neste change).

## Task 6 — Manager: editar default disappearing timer

Write scope: `manager/app/composables/` (novo ou estender profile/messaging), `InstanceSettingsSection.vue`, `en.json`, `api.ts` se necessário.

1. Form/select valores `0`, `24h`, `168h`, `2160h` → `PUT /instances/{id}/chats/default-disappearing`.
2. Refresh instance após save; typecheck + build.

## Task 7 — Manager: section na URL

Write scope: `manager/app/pages/instances/[id].vue`, `InstanceSectionNav.vue`.

1. Sync `?section=` com nav (overview, messages, …); read on mount; replace shallow ao trocar aba.

## Task 8 — Manager: api key indicator

Write scope: `useInstances.ts`, settings UI.

1. Remover heurística enganosa de `localStorage` OU substituir por campo explícito se API expuser `has_api_key` — preferir remover/indicar "rotate to issue key" sem fake state.

## Task 9 — Manager: notificações

Write scope: `useDashboard.ts`, `NotificationsSlideover.vue`.

1. Remover atalho `n` até haver feed real, ou desabilitar com copy "coming soon" (sem slideover vazio enganoso).

## Task 10 — Enqueue: alinhar sessão ao vivo

Write scope: `internal/message/service.go`, `service_test.go`.

1. Após check DB `connected`, verificar sessão registrada no manager (ou resolver disponível) antes de `Create`; mapear para `409`/`503` coerente com handlers existentes.
2. Testes unitários com fake session manager.

## Task 11 — Gates finais + ledger

1. Gates completos; atualizar `.superpowers/sdd/progress.md`.

## Out of scope (registrar no retrospective deste change)

- Assinatura webhook Chatwoot v2 (design separado).
- Outbox transacional 2PC com mensagens.
- E2E browser suite.
- Otimização latência `GET /instances` agregado.
