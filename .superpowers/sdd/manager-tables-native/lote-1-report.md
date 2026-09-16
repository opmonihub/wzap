# Lote 1 report — instances → bindings nativos UTable

BASE: b05486d. Escopo: só `manager/` (+ este ledger). Sem subagentes, sem worktree.

## Arquivos (1 commit)

- `manager/app/composables/useInstancesTable.ts` — coluna `select` display
  (`enableSorting:false, enableHiding:false`, checkboxes via slots na página);
  `name`/`status` com sorting nativo; `status` com `filterFn: 'equalsString'` +
  `meta.filterVariant: 'select'`; `external_ref` sem sort; `owner` admin-only via
  meta (header no computed, locale runtime); `whatsapp_jid` sem sort +
  `enableHiding:true`; global filter nativo (`name + external_ref`,
  case-insensitive) exposto via `globalFilterOptions`; retorna `{ columns,
  sorting, globalFilter, columnFilters, columnVisibility, rowSelection,
  pagination }` (+ `globalFilterOptions`, `paginationOptions`); `void items`
  mantido (items flui como `:data` na página), `tableState` removido.
- `manager/app/pages/instances/index.vue` — `UTable` com `v-model:sorting`,
  `v-model:global-filter`, `v-model:column-filters`,
  `v-model:column-visibility`, `v-model:row-selection`, `v-model:pagination` +
  `ref="table"` (`tableApi`); `:data="items"` (lista cheia); engine manual
  removido (`filteredItems/sortedFilteredItems/pagedItems/toggleSort/sortIcon`);
  headers `UButton` com `column.getIsSorted()/column.toggleSorting()` +
  `aria-sort` + `aria-label` i18n, `size="sm"` + `min-h-11` (≥44px, sem
  `-mx-2.5`); search com `aria-label`; filtro status via `USelect` ↔
  `columnFilters`; visibilidade via `UDropdownMenu` (colunas hideáveis);
  seleção + barra bulk (contador + `Copiar nomes` + `Limpar`); `onSelectRow`
  preservado + teclado via `NuxtLink` na célula nome (link explícito e
  focável; clique no link não dispara `@select` por guarda interna da UTable);
  `UPagination` via `tableApi` (`total=getFilteredRowModel().rows.length`,
  `setPageIndex`); `#empty` (lista zerada → CTA criar; sem resultados →
  limpar filtros); `:loading="pending"` + skeleton; `loadMore`/`onCreated`/
  owner emails/`loadedLabel`/`pageLabel` preservados; `truncate` em
  nome/jid/ref; `owner`/`jid` colapsam por viewport (hidden até mount).
- `manager/i18n/locales/en.json` — só adições sob `instances.table.*`:
  `selectAll, selectRow, selectedCount, clearSelection, copyNames,
  copiedNames, allStatuses, statusFilter, visibility, filtersLabel,
  clearFilters`. Nenhum literal novo em UI.
- Commit: `feat(manager): instances table on native UTable bindings`
  (só os 3 arquivos acima; `.gitkeep` do `.output` restaurado; sujeira
  preexistente de `openspec/changes/...` fora do commit).

## Testes (comandos + outputs)

- `pnpm --dir manager lint` → limpo (`$ eslint .`, sem erros).
- `pnpm --dir manager typecheck` → só o erro baseline aceito
  `nuxt.config.ts(3,24) process` (arquivo intocado; pré-existente). Dois erros
  intermediários meus foram corrigidos: inferência circular do
  `useTemplateRef('table')` (resolvido com interface estrutural
  `InstancesTableApi`, sem importar `@tanstack/*`) e índices `string |
  undefined` sob `noUncheckedIndexedAccess`.
- `pnpm --dir manager build` → ok (`Generated public .output/public`,
  prerender inclui `/instances`).
- Go intocado (`gofmt` n/a; sem `go vet` — fora do escopo `manager/`).

## Prova grep (como verificado)

- `rg "column\.toggleSorting" manager/app/pages/instances/index.vue` → 2
  ocorrências (headers name + status, padrão docs Nuxt UI).
- `rg "v-model:sorting|v-model:global-filter|v-model:column-filters|v-model:column-visibility|v-model:row-selection|v-model:pagination"`
  → 6 linhas presentes no `UTable`.
- `rg "toggleSort[^i]|sortIcon|filteredItems|sortedFilteredItems|pagedItems|tableState"`
  nos 2 arquivos → sem match (engine manual removida; `toggleSort` só aparece
  como substring de `toggleSorting` nativo).
- `rg "#empty|aria-sort"` → `#empty` (1 slot) + `aria-sort` (2 botões sort).
- `rg "@tanstack" manager/app/` → só comentários + 1 comentário pré-existente
  em `pages/accounts/index.vue` (fora do escopo); nenhum `import`, nenhuma
  dependência adicionada (`git diff manager/package.json` vazio).
- `rg "console\.log"` nos 2 arquivos → sem match.

## Decisões / divergências

1. Paginação (divergência documentada do brief): a SAÍDA do brief (`:data` =
   slice de `tableApi.getSortedRowModel()` em computed) é circular — o slice
   realimenta o modelo e páginas >1 convergem para vazio. Em vez disso,
   `:pagination-options` com `clientPaginationRowModel()` local (~15 linhas,
   em `useInstancesTable.ts`), que fatia `getPrePaginationRowModel()` como o
   `getPaginationRowModel` oficial (padrão docs Nuxt UI, sem importar
   `@tanstack/*`, sem nova dependência). `:data` segue com a lista cheia;
   sorting/filter/pagination 100% nativos, sem engine manual. `getRowId`
   estável (`row.id`) preserva seleção entre páginas; `:auto-reset-all="false"`
   + watchers explícitos (`pageIndex=0` em filter/sort, clamp em shrink) para
   `loadMore` não resetar a página.
2. Teclado na linha: UTable dá `tabindex=0`/`role=button` na `tr` mas sem
   handler de teclado; escolha simples = `NuxtLink` explícito na célula nome
   (Enter nativo, sem navegação dupla).
3. `aria-sort` no `UButton` do header (o `th` não é alcançável via slots);
   `aria-label` i18n já anuncia a ação.
4. Visibilidade = viewport-base + overrides do usuário (diff no setter do
   `v-model`); dropdown deriva das colunas hideáveis, sem depender de
   `tableApi` no primeiro paint.
5. Bulk além do mínimo: contador + limpar + copiar nomes (clipboard
   `@vueuse/core`, toast i18n); sem ação destrutiva.

## Concerns

- `aria-sort` no botão em vez do `th[scope=col]`: ATs podem não anunciar
  estado de ordenação no contexto da coluna; mitigado pelo `aria-label`
  ("Sort X ascending/descending"). Lote 3 pode reavaliar.
- `tr` focável sem ativação por teclado (limitação UTable): o link do nome é
  o caminho de teclado; usuário que der Enter na `tr` não navega.
- `useClipboard` sem fallback de erro: falha de permissão não mostra toast de
  erro (só não copia); aceitar ou tratar no harden.
- Dropdown de visibilidade permite ocultar `name`/`status`/`external_ref`
  (hideáveis por padrão); só `select` trava (`enableHiding:false`). Intencional
  (docs pattern), mas Lote 3 pode travar mais colunas.
