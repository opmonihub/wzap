## Why

O console (`manager/`) é o frontend do serviço, mas o overview (`pages/index.vue`) é estático: 4 `UCard` sem números reais, sem gráfico, sem toolbar. O template canônico `nuxt-ui-templates/dashboard` (referência em `/home/obsidian/dev/research/dashboard`) mostra o padrão: Home com stats clicáveis, gráfico por período e tabela de recentes. Alinhar o console a esse padrão torna o overview útil no dia a dia e deixa as listas/detalhe consistentes com customers/settings/inbox.

## What Changes

- Overview vira Home do template: navbar com sino + ação criar, `UDashboardToolbar` com seletor de período, body com `OverviewStats` (4 `UPageCard`), `OverviewChart` (Unovis linha+área por `created_at`) e `OverviewRecent` (5 mais recentes).
- Novo endpoint aditivo `GET /instances/stats` respeitando escopo (admin vê tudo, user só as próprias), resposta `{"data":{"total":N,"by_status":{...}}}`; frontend usa com fallback para contagem local cursor-acumulada.
- Listas de instâncias e contas adotam o `ui` de tabela do template customers (bordas arredondadas, header com fundo) e footer com contagem de selecionados + `UPagination`, mantendo dados e ações atuais.
- Detalhe da instância adota split settings+inbox: forms centrais `lg:max-w-2xl`, messages como painel lateral no desktop e `USlideover` no mobile (já existe, só falta o split).
- Novas deps diretas `@unovis/vue` + `date-fns` (só para período do overview).
- Mesmos componentes base do template: `UDashboard*`, `UTable` + `getPaginationRowModel()`, `UForm`+Zod4, `UModal` com `#footer`, `UEmpty`/`USkeleton`/toasts, tokens semânticos.

## Capabilities

### New Capabilities

- Overview Home com métricas reais: stats por status, gráfico de criação por período e recentes, derivados de `GET /instances/stats` + listagem.

### Modified Capabilities

- `wzap-manager`: overview, tabelas e detalhe seguem o layout do template; comportamentos (RBAC, escopo, confirmações, pairing, keys, webhook, envio) preservados.

## Out-of-Scope

- Nenhuma mudança em NATS, schema Postgres, envelopes, auth/sessão, quotas, webhook, chatwoot, envio, eventos; `external_ref` segue opaco; i18n segue só EN.
- Sem novas telas/fluxos além do overview Home; sem mocks; sem `GET /users/stats` nem série temporal no backend nesta change.
- Sem fork total do template (sem inbox/teams fictícios); sem trocar `keySeen`/`localStorage` por campo de API.
- `verify.md` + `retrospective.md` só pós-apply, antes do PR.

## Impact

- `manager/`: pages, `components/overview/*`, `composables/useOverview.ts`, `i18n/locales/en.json`, `package.json` + lockfile, rebuild do embed Go.
- `internal/httpapi/`: rota aditiva `GET /instances/stats` + handler + testes; sem migração.
- Sem impacto em consumidores: nenhum contrato externo muda, endpoint é aditivo.
