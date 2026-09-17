# Executar: manager-dashboard-frontend

Implementar o refactor do `manager/` como frontend padrão do template dashboard, com dados da API `internal/` preservados + endpoint aditivo `GET /instances/stats`.

## Global Constraints (vinculam todas as tasks)

- Contratos congelados: envelopes `{"data":...}` / `{"error":{"code","message"}}` + `X-Request-Id`; cursor `next_cursor`; `external_ref` opaco; RBAC admin vê tudo / user vê só próprias / instance key sem visão de coleção (403); sessão `wzap_session` cookie; i18n só EN; dívida `keySeen/localStorage` mantida.
- Endpoint novo: `GET /instances/stats` responde `{"data":{"total":N,"by_status":{"connected":N,"disconnected":N,"pairing":N,"error":N}}}` com escopo (admin/global tudo, user próprias, instance key 403).
- Frontend: só primitivas canônicas do template (`UDashboard*`, `UTable` + `getPaginationRowModel()`, `UForm`+Zod4, `UModal` com `#footer`, `UEmpty`/`USkeleton`, `UPageGrid`/`UPageCard`, Unovis `VisLine`/`VisArea` + `date-fns` só no overview); tokens semânticos, sem hex hardcoded, sem `file:` cru; sem mocks; sem telas/fluxos novos além do overview Home.
- Qualidade: `gofmt`, `go vet`, `golangci-lint` (govet/staticcheck/errcheck/ineffassign/unused), `go test`, `pnpm lint/typecheck/build`; commits convencionais (`feat(api):`, `feat(manager):` etc.) com `Co-authored-by: factory-droid[bot] <138933559+factory-droid[bot]@users.noreply.github.com>`.
- Escopo por task: implementar exatamente o que a task pede; YAGNI; seguir padrões existentes; não reestruturar fora da task.

## Estado inicial (já existe no worktree, NÃO recriar)

- Change OpenSpec: `openspec/changes/manager-dashboard-frontend/` com `brainstorm.md`, `proposal.md`, `specs/wzap-manager/spec.md`, `design.md`, `tasks.md` (+ `.openspec.yaml`).
- Backend parcial NÃO commitado: `internal/httpapi/instances.go` já contém `instanceStatsResponse` + `handleInstanceStats` (acumula via `instances.List` com `maxInstancesLimit`, filtra por `filterInstancesByOwner`, status desconhecido dobra em `disconnected`); `internal/httpapi/server.go` já registra `GET /instances/stats` ANTES de `GET /instances`. Verificar, testar e commitar — não reescrever do zero.
- Spec autoridade: `openspec/specs/wzap-manager/spec.md` + delta em `openspec/changes/manager-dashboard-frontend/specs/wzap-manager/spec.md`.

## Task 1: backend GET /instances/stats com escopo e testes

Arquivos: `internal/httpapi/instances.go`, `internal/httpapi/server.go`, `internal/httpapi/instances_stats_test.go` (novo), `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go` (regenerados via swag).

Passos:
1. Confirmar o handler parcial existente (`instanceStatsResponse`, `handleInstanceStats`, rota em `server.go` antes de `/instances` — ordem importa no mux). Ajustar só se divergir das constraints.
2. TDD: escrever `internal/httpapi/instances_stats_test.go` com `TestInstanceStats*`: admin/global contam tudo por status; user conta só próprias (legado NULL-owner excluído); instance key recebe 403 `forbidden`; status desconhecido dobra em `disconnected`; erro do service vira 500 sem vazar causa; paginação acumula via cursor (duas páginas). Reusar `fakeInstanceService`, `serveRBAC`, `newRBACFixture`/`rbacServer` de `rbac_test.go` e `instances_test.go`.
3. Rodar `go test ./internal/httpapi/ -run 'TestInstanceStats' -count=1` (espera-se RED antes do ajuste, GREEN depois).
4. Regenerar swagger (`swag init` conforme `docs/` do repo) e rodar `go test ./internal/httpapi/ -run 'TestSwagger' -count=1`.
5. `gofmt -l internal/httpapi/`, `go vet ./internal/httpapi/`, commit `feat(api):`.

## Task 2: deps do overview + composable useOverview

Arquivos: `manager/package.json`, `manager/pnpm-lock.yaml`, `manager/app/composables/useOverview.ts` (novo), `manager/app/types/api.ts` (tipo `InstanceStats`).

Passos:
1. `pnpm --dir manager install` após adicionar `@unovis/vue` + `date-fns` (versões compatíveis com o template em `/home/obsidian/dev/research/dashboard`); commitar lockfile atualizado.
2. Criar `useOverview.ts`: busca `GET /instances/stats` (via `useApi`, cookie de sessão automático) com fallback para contagem local da listagem cursor-acumulada (`listInstances` de `useInstances`) quando o endpoint falhar; expõe `{ stats, recent, pending, failure, fallback }`; `recent` = 5 mais recentes por `created_at` desc da listagem acumulada (limite de páginas razoável).
3. Adicionar tipo `InstanceStats { total: number; by_status: Record<string, number> }` em `types/api.ts`.
4. Verificar `pnpm --dir manager typecheck`, commit `feat(manager):`.

## Task 3: componentes overview + Home

Arquivos: `manager/app/components/overview/OverviewStats.vue`, `OverviewChart.client.vue`, `OverviewChart.server.vue`, `OverviewRecent.vue` (novos), `manager/app/pages/index.vue` (reescrita como Home), `manager/i18n/locales/en.json` (chaves `overview.stats.*`, `overview.chart.*`).

Passos:
1. `OverviewStats.vue`: `UPageGrid` com 4 `UPageCard` (total, connected, pairing/disconnected, error) com ícone, valor e link para `/instances`, seguindo `HomeStats.vue` do template; textos via i18n.
2. `OverviewChart.client.vue` + `.server.vue`: Unovis `VisLine`+`VisArea` sobre histograma de `created_at` por dia/semana/mês (`date-fns` `eachDayOfInterval`/`eachWeekOfInterval`/`eachMonthOfInterval`), props `period`+`range`; server rende skeleton; seguir `HomeChart.client.vue`/`HomeChart.server.vue` do template.
3. `OverviewRecent.vue`: mini `UTable` com as 5 recentes (nome, status badge, JID, link detalhe); `UEmpty` quando vazio.
4. `pages/index.vue`: navbar com sino + botão criar, `UDashboardToolbar` com seletor de período, body Stats+Chart+Recent; mantém saudação/escopo atuais de forma discreta; `UAlert`+retry em falha; nunca inventar dados.
5. Verificar `pnpm --dir manager lint`, `typecheck`, `nuxt generate` (build), commit `feat(manager):`.

## Task 4: listas e detalhe no padrão do template

Arquivos: `manager/app/pages/instances/index.vue`, `manager/app/pages/accounts/index.vue`, `manager/app/pages/instances/[id].vue`.

Passos:
1. Listas: adotar `ui` de tabela do template customers (`table-fixed border-separate border-spacing-0`, thead com fundo, bordas arredondadas, `td border-b`) e footer com contagem de selecionados + `UPagination`; manter colunas, filtros, ordenação, seleção, paginação, bulk actions e toasts atuais — só visual.
2. Detalhe: reorganizar em split settings+inbox — forms (nome/external_ref, key, webhook, test-send) em coluna central `lg:max-w-2xl`, pairing+info no topo, messages como painel lateral no desktop / `USlideover` no mobile (já existe), danger no fim; NENHUM card filho alterado, só wrappers e classes.
3. Verificar `pnpm --dir manager lint`, `typecheck`, `nuxt generate`, viewports desktop+mobile descritos no relatório, commit `refactor(manager):`.

## Task 5: verificação final e sincronização OpenSpec

Arquivos: `openspec/changes/manager-dashboard-frontend/tasks.md` (checkboxes), `openspec/changes/manager-dashboard-frontend/plan.md` (este arquivo, se precisar de correção — não reescrever), nenhum código salvo instrução em contrário.

Passos:
1. Rebuild `pnpm --dir manager build` e rodar a ordem CI: `go vet ./...`, `golangci-lint run`, `go test ./... -count=1` (sem Postgres a menos que `WZAP_TEST_DATABASE_URL` esteja definido), `go build ./...`, `gofmt -l .` deve imprimir nada.
2. Marcar checkboxes concluídos em `tasks.md` (um commit `docs(specs):` separado se houver outra alteração pendente — nunca misturar).
3. Relatório final: matriz manual (stats batem com lista, gráfico muda com período, filtros/paginação/seleção/loadMore nas duas tabelas, detalhe desktop+mobile, login válido/inválido) + comandos e saídas.
4. NÃO escrever `verify.md` nem `retrospective.md` (pós-apply, antes do PR, fora deste plano).
