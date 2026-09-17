# Brainstorm — componentização das páginas do manager

Data: 2026-09-17. Pedido original (PT): páginas devem fazer só função de
página, com componentes reutilizáveis em `manager/app/pages`
(accounts, instances, index.vue, login.vue).

## Classificação do caminho

- Proposta inicial: **bounded** (fluxo existente, mudança delimitada).
- Usuário optou por **arquitetural** (ratchet one-way, sem downgrade).
- Gate: nenhum código ou scaffold até aprovação do design — cumprido; só
  leitura de arquivos + docs Nuxt UI + este registro.

## Contexto levantado

- `pages/index.vue` (overview): 146 linhas, já magra (`OverviewStats`,
  `OverviewChart`, `OverviewRecent`); header/toolbar/estados inline.
- `pages/login.vue`: 63 linhas, quase só `UAuthForm`.
- `pages/accounts/index.vue`: 873 linhas — tabela + 3 modais inline
  (create/quota/delete) + bulk delete; maior problema.
- `pages/instances/index.vue`: 638 linhas — modais já extraídos
  (`Create/Edit/DeleteInstanceModal`), mas toolbar/tabela/paginação
  duplicadas com accounts (~200 linhas).
- `pages/instances/[id].vue`: 559 linhas — 7 seções inline
  (overview/messages/groups/channels/profile/integrations/settings).
- Componentes já existem por domínio (`accounts/`, `instances/`,
  `overview/`); não há pasta compartilhada. Composables de tabela:
  `useAccountsTable`, `useInstancesTable`.
- Nuxt UI v4.11 analisada a pedido do usuário: `UDashboardPanel` com
  slots header/body (+ `UDashboardNavbar` no header); `UTable` sobre
  TanStack com `tableApi` via template ref e toolbar externa (filtro +
  dropdown de colunas); `UModal` com `v-model:open` + slots body/footer;
  `UAuthForm` com fields/schema + slot validation (doc recomenda envolver
  em `UPageCard` na página de login).

## Perguntas e decisões

1. Papel da página ("só composição" vs "pode orquestrar dados" vs "só
   extrair blocos") — usuário desviou para análise da doc Nuxt UI;
   **assunção registrada**: página pode orquestrar via composables
   (opção intermediária), sem markup de tabela/modal/seção.
2. Reuso entre tabelas (shell genérico vs só domínio vs tudo genérico) —
   resposta: adotar o padronizado, genérico + domínio se fizer sentido.
3. Abordagem A/B/C — escolhida **A híbrida (recomendada)**:
   `components/shared` (só o idêntico) + componentes de domínio.

## Abordagens consideradas

- **A híbrida**: base compartilhada (`PageState`, `DataTableShell`
  toolbar/footer) + cellules/tabelas/modais/seções por domínio.
  Elimina a duplicação, segue os slots da Nuxt UI. Escolhida.
- **B só domínio**: `AccountsTable`/`InstancesTable` independentes, sem
  abstração comum. Menor risco, mas mantém ~200 linhas duplicadas.
  Rejeitada.
- **C só composables**: extrai lógica, mantém markup na página. Mudança
  mínima, mas não atende "página só faz função de página". Rejeitada.

## Seções do design (todas aprovadas uma a uma)

1. Arquitetura e regra da página (página ≤~120 linhas; nova pasta
   `components/shared`; composables e i18n inalterados).
2. Compartilhados: `PageState`, `DataTableToolbar`, `DataTableFooter`;
   `UTable` tipada fica no domínio (evita generic complexo no TanStack).
3. Domínio: `AccountsTable` + 3 modais de accounts;
   `InstancesTable` reutilizando modais existentes; 7 seções do `[id]` +
   `SectionNav`; `OverviewHeader`; `LoginForm`.
4. Fluxo (página orquestra props/emits, estado de tabela no componente,
   modais `v-model:open` + emits) + verificação (typecheck, lint, build,
   checklist das 5 rotas, meta zero `UTable`/`UModal` nas páginas).
