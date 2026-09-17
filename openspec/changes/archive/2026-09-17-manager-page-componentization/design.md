## Context

See `brainstorm.md` for the dialogue capture. Current state
(`manager/app/pages`): `accounts/index.vue` 873 linhas (tabela + 3
`UModal` inline + bulk delete), `instances/index.vue` 638 linhas (modais
já extraídos, toolbar/tabela/paginação duplicadas com accounts),
`instances/[id].vue` 559 linhas (7 seções inline), `index.vue` 146
linhas (parcialmente componentizada), `login.vue` 63 linhas (só
`UAuthForm`). Componentes por domínio já existem (`accounts/`,
`instances/`, `overview/`); sem pasta compartilhada. Padrões Nuxt UI
v4.11 verificados: `UDashboardPanel` header/body + `UDashboardNavbar`,
`UTable`/TanStack com `tableApi` e toolbar externa, `UModal`
`v-model:open` + body/footer, `UAuthForm` fields/schema + validation
(envelopada em `UPageCard` no login). Constraint: `manager/` é EN,
`baseURL /manager/`, embutido via `go:embed`; nenhum contrato Go muda.

## Goals / Non-Goals

- Goals: páginas com ≤~120 linhas (login ~30) contendo só SEO/meta,
  guard/navigate, chamada a composables e composição; todo markup de
  tabela/toolbar/modal/seção em componentes reutilizáveis; eliminar a
  duplicação toolbar/paginação/estados entre as duas listas.
- Non-Goals: nenhuma mudança de API, auth/RBAC, i18n keys, contratos de
  emits dos cards existentes, `data-testid`s ou comportamento de fetch
  (incl. o fetch redundante do aside/slideover de mensagens, já aceito).

## Out-of-Scope

- No Go, rotas, migrations, NATS ou `openspec/specs/`; no novas
  dependências; no runner de testes frontend; no redesign visual além da
  componentização (tokens e `:ui` atuais mantidos).

## Decisions

- **Regra da página: compor + orquestrar via composables, sem markup de
  domínio.** Rationale: atende "página só faz função de página" sem
  proibir fetch/guard (assunção explícita do brainstorm, a confirmar na
  revisão). Alternative (página burra 100%, tudo em wrappers)
  rejected: criaria componentes de passagem sem valor.
- **Novo `components/shared/` só para o idêntico: `PageState`,
  `DataTableToolbar`, `DataTableFooter`.** Rationale: as duas listas
  duplicam ~200 linhas de skeleton/erro, busca com debounce, dropdown
  de colunas e rodapé de paginação. Alternative (tudo por domínio)
  rejected: mantém a duplicação; alternative (shell genérico total
  incluindo `UTable`) rejected: generic tipado no TanStack complica sem
  ganho.
- **`UTable` tipada vive em `AccountsTable` / `InstancesTable`, que
  compõem o shell via slots e expõem `tableApi` com `defineExpose`.**
  Rationale: segue o exemplo oficial (toolbar fora, `tableApi` via
  template ref) e isola os tipos `AccountUser`/`Instance`.
  Alternative (UTable direto na página) rejected: é o problema atual.
- **Modais de accounts extraídos no molde dos de instances
  (`CreateAccountModal`, `EditQuotaModal`, `DeleteAccountModal`, com
  `v-model:open` + emits).** Rationale: instances já prova o padrão;
  cada modal detém schema zod, `parseQuota` e erros de campo.
  Alternative (manter modais na página) rejected: ~140 linhas inline.
- **Detalhe `[id]` fatiado em seções + `InstanceSectionNav`
  (`UNavigationMenu`); página mantém `load`, `section` e toasts.**
  Rationale: 7 ramos inline viram unidades testáveis sem mudar o
  contrato de montagem CSS-gated de mensagens. Alternative (uma seção
  por rota) rejected: muda navegação/URLs fora do escopo.
- **`OverviewHeader` e `LoginForm` fecham as pontas; composables e
  `en.json` intocados.** Rationale: overview/login já são pequenas;
  extrair só header/form evita churn. Alternative (deixar como estão)
  rejected: header do overview e lógica do login ainda moram na página.

## Risks / Trade-offs

- [Risk] Abstração `shared/` prematura diverge entre listas no futuro ->
  Mitigation: shell só recebe o idêntico via props/slots; divergência
  volta para o componente de domínio, nunca vira flag no shared.
- [Risk] Estado de tabela (sorting/filter/pagination/selection) muda de
  dono (página → componente) e quebra contadores não-reativos ->
  Mitigation: contadores via `computed` sobre os mesmos refs + leitura
  de `tableApi` com fallback, como hoje; `defineExpose` tipado.
- [Risk] `UTable` sem slot `th` (aria-sort já tratado via aria-label
  nos botões) regride a11y na extração -> Mitigation: copiar os
  templates de header/cell verbatim, sem reinventar.
- [Risk] `Reka SelectItem` proíbe value vazio (sentinela `'all'`) ->
  Mitigation: manter o mapeamento sentinela↔sem-filtro dentro dos
  Table components.

## Migration Plan

- Frontend-only, sem migration: build embutido (`nuxt generate` →
  `go:embed`); rollback = binário anterior; sem feature flag (troca
  interna, sem mudança visual intencional).
- Sequência: shared → `AccountsTable` + modais → `InstancesTable`
  → seções `[id]` → `OverviewHeader`/`LoginForm` → fatiar páginas.
- Validação: `pnpm --dir manager typecheck` → `pnpm --dir manager lint`
  → `pnpm --dir manager build`; matriz manual: 5 rotas × 7 seções do
  detalhe × admin/user × light/dark; métricas: páginas ≤~120 linhas,
  zero `UTable`/`UModal` em `pages/`, shell reusado 2×.

## Open Questions

- Confirmar na revisão: página pode orquestrar via composables (opção
  intermediária) ou deve ser 100% burra? Design assume a intermediária.
